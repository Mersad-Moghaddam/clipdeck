package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/ipc"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

func pickerLabel(s string) string {
	out := []rune{}
	for _, r := range s {
		if len(out) >= 159 {
			out = append(out, '…')
			break
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			r = ' '
		}
		out = append(out, r)
	}
	return string(out)
}
func selectedRow(rows []string, output string) (int, error) {
	output = strings.TrimSuffix(output, "\n")
	for i, row := range rows {
		if row == output {
			return i, nil
		}
	}
	return -1, fmt.Errorf("selector returned an unknown row")
}

func runHistoryCommand(cfg *config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return 2
	}
	command := args[0]
	if command != "list" && command != "copy" && command != "pick" {
		fmt.Fprintln(stderr, "Unknown history command")
		return 2
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	limit := fs.Int("limit", 50, "maximum items (1..200)")
	offset := fs.Int("offset", 0, "page offset")
	query := fs.String("query", "", "search text")
	kind := fs.String("type", "", "content type")
	asJSON := fs.Bool("json", false, "safe display JSON")
	tool := fs.String("tool", "", "selector executable")
	paste := fs.Bool("paste", false, "request paste after selection")
	plain := fs.Bool("plain", false, "copy plain text")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	badFlag := false
	fs.Visit(func(f *flag.Flag) {
		allowed := command == "list" && (f.Name == "limit" || f.Name == "offset" || f.Name == "query" || f.Name == "type" || f.Name == "json") || command == "copy" && f.Name == "plain" || command == "pick" && (f.Name == "tool" || f.Name == "paste" || f.Name == "query" || f.Name == "limit" || f.Name == "type")
		if !allowed {
			fmt.Fprintf(stderr, "--%s is not valid for %s\n", f.Name, command)
			badFlag = true
		}
	})
	if badFlag {
		return 2
	}
	if *limit < 1 || *limit > 200 || *offset < 0 {
		fmt.Fprintln(stderr, "limit must be 1..200 and offset nonnegative")
		return 2
	}
	if (command == "copy" && fs.NArg() != 1) || (command != "copy" && fs.NArg() != 0) {
		fmt.Fprintln(stderr, "copy requires one ID; list/pick accept flags only")
		return 2
	}
	c, err := ipc.NewClient(cfg.IPC.SocketPath)
	if err != nil {
		fmt.Fprintln(stderr, "Daemon unavailable; start clipdeckd and run clipdeck doctor.")
		return 1
	}
	defer c.Close()
	copyID := func(id string) int {
		method := "copy"
		if *paste {
			method = "paste"
		}
		var res map[string]any
		if err := c.Call(method, map[string]any{"id": id, "plain": *plain}, &res); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *paste {
			fmt.Fprintln(stderr, "Copied; paste injection requested (delivery is not confirmed).")
		}
		return 0
	}
	if command == "copy" {
		return copyID(fs.Arg(0))
	}
	opts := store.QueryOptions{Query: *query, Limit: *limit, Offset: *offset}
	if *kind != "" {
		opts.Types = []string{*kind}
	}
	var page store.QueryPage
	if err := c.Call("query", opts, &page); err != nil {
		fmt.Fprintln(stderr, "Query failed; this command requires a query-capable daemon:", err)
		return 1
	}
	if command == "list" {
		if *asJSON {
			if err := json.NewEncoder(stdout).Encode(page); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		} else {
			for _, item := range page.Items {
				fmt.Fprintf(stdout, "%s\t[%s] %s\n", item.ID, item.Type, pickerLabel(item.Preview))
			}
		}
		return 0
	}
	if len(page.Items) == 0 {
		fmt.Fprintln(stderr, "No clips found.")
		return 0
	}
	tools := map[string][]string{"fuzzel": {"--dmenu"}, "wofi": {"--dmenu"}, "rofi": {"-dmenu", "-p", "ClipDeck"}, "dmenu": {"-p", "ClipDeck"}, "fzf": {"--prompt=ClipDeck> "}}
	chosen := *tool
	if chosen == "" {
		candidates := []string{"rofi", "dmenu"}
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			candidates = []string{"fuzzel", "wofi", "rofi"}
		}
		if fi, e := os.Stderr.Stat(); e == nil && fi.Mode()&os.ModeCharDevice != 0 {
			candidates = append(candidates, "fzf")
		}
		for _, name := range candidates {
			if _, e := exec.LookPath(name); e == nil {
				chosen = name
				break
			}
		}
	}
	argv, ok := tools[chosen]
	if !ok {
		fmt.Fprintln(stderr, "Install fuzzel/wofi/rofi, or select --tool fzf in a terminal.")
		return 2
	}
	rows := make([]string, len(page.Items))
	for i, item := range page.Items {
		rows[i] = fmt.Sprintf("%d [%s] %s", i+1, item.Type, pickerLabel(item.Preview))
	}
	cmd := exec.Command(chosen, argv...)
	cmd.Stdin = strings.NewReader(strings.Join(rows, "\n") + "\n")
	cmd.Stderr = stderr
	output, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 1 || exit.ExitCode() == 130) {
			return 0
		}
		fmt.Fprintln(stderr, "Selector failed:", err)
		return 1
	}
	if len(output) == 0 {
		return 0
	}
	index, err := selectedRow(rows, string(output))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return copyID(page.Items[index].ID)
}
