package main

import (
	"bytes"
	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"strings"
	"testing"
)

func TestHistoryCommandValidation(t *testing.T) {
	for _, args := range [][]string{{"pick", "--json"}, {"list", "--paste"}, {"copy", "--tool", "fzf", "id"}, {"unknown"}, {"list", "--limit", "0"}} {
		var out, diag bytes.Buffer
		if code := runHistoryCommand(config.DefaultConfig("dev"), args, &out, &diag); code != 2 {
			t.Errorf("%v: got %d, %s", args, code, diag.String())
		}
	}
}

func TestPickerSanitizesControls(t *testing.T) {
	got := pickerLabel("hello\nworld\t\x1b[31m\u202esecret")
	for _, bad := range []string{"\n", "\t", "\x1b", "\u202e"} {
		if strings.Contains(got, bad) {
			t.Fatalf("unsafe label %q", got)
		}
	}
	if len([]rune(pickerLabel(strings.Repeat("a", 500)))) > 160 {
		t.Fatal("unbounded label")
	}
}

func TestPickerExactRowMapping(t *testing.T) {
	rows := []string{"1 [text] same", "2 [text] same"}
	if i, err := selectedRow(rows, "2 [text] same\n"); err != nil || i != 1 {
		t.Fatalf("%d %v", i, err)
	}
	if _, err := selectedRow(rows, "2 forged"); err == nil {
		t.Fatal("accepted forged output")
	}
}
