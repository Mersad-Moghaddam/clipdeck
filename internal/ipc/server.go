package ipc

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Mersad-Moghaddam/clipdeck/internal/capture"
	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/desktop"
	"github.com/Mersad-Moghaddam/clipdeck/internal/hypr"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
	"github.com/Mersad-Moghaddam/clipdeck/internal/transform"
)

type Request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Version is reported over IPC so clients can display the daemon they are
// talking to. The daemon entrypoint sets it at startup from its own version
// constant; the default keeps library-only users (tests, tools) sane.
var Version = "dev"

type Response struct {
	ID     string    `json:"id"`
	Result any       `json:"result,omitempty"`
	Error  *RPCError `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Event struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

type Server struct {
	cfg       *config.Config
	store     *store.Store
	capturer  *capture.Capturer
	listener  net.Listener
	clients   map[net.Conn]bool
	clientsMu sync.Mutex
	startTime time.Time
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewServer(cfg *config.Config, s *store.Store, cap *capture.Capturer) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		cfg:       cfg,
		store:     s,
		capturer:  cap,
		clients:   make(map[net.Conn]bool),
		startTime: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (s *Server) Start() error {
	sockPath := s.cfg.IPC.SocketPath
	if sockPath == "" {
		return fmt.Errorf("IPC socket path not configured")
	}

	dir := filepath.Dir(sockPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}
	_ = os.Chmod(dir, 0700)

	// Remove stale socket if exists
	_ = os.Remove(sockPath)

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listen on unix socket %s: %w", sockPath, err)
	}
	s.listener = l

	if err := os.Chmod(sockPath, 0600); err != nil {
		_ = l.Close()
		return fmt.Errorf("chmod socket 0600: %w", err)
	}

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

func (s *Server) Stop() {
	s.cancel()
	if s.listener != nil {
		_ = s.listener.Close()
	}

	s.clientsMu.Lock()
	for conn := range s.clients {
		_ = conn.Close()
	}
	s.clients = make(map[net.Conn]bool)
	s.clientsMu.Unlock()

	s.wg.Wait()
	if s.cfg.IPC.SocketPath != "" {
		_ = os.Remove(s.cfg.IPC.SocketPath)
	}
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			// A dead accept loop leaves the socket file lying around with
			// nobody serving it — every client would just time out. Exiting
			// lets the systemd unit restart the daemon into a known state.
			log.Printf("IPC accept failed fatally: %v; exiting", err)
			os.Exit(1)
		}

		unixConn, ok := conn.(*net.UnixConn)
		if !ok {
			_ = conn.Close()
			continue
		}

		if err := verifyPeerCred(unixConn); err != nil {
			log.Printf("IPC unauthorized connection rejected: %v", err)
			_ = conn.Close()
			continue
		}

		s.clientsMu.Lock()
		s.clients[conn] = true
		s.clientsMu.Unlock()

		go s.handleConnection(conn)
	}
}

func verifyPeerCred(conn *net.UnixConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var credErr error
	err = raw.Control(func(fd uintptr) {
		ucred, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			credErr = fmt.Errorf("failed to get peer cred: %w", err)
			return
		}
		if ucred.Uid != uint32(os.Getuid()) {
			credErr = fmt.Errorf("peer UID %d does not match process UID %d", ucred.Uid, os.Getuid())
		}
	})
	if err != nil {
		return err
	}
	return credErr
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
		_ = conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	// Support up to 10MB per line for large data transfers if needed
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendResponse(conn, Response{
				ID:    "",
				Error: &RPCError{Code: -32700, Message: "Parse error: " + err.Error()},
			})
			continue
		}

		res, rpcErr := s.dispatch(conn, req.Method, req.Params)
		s.sendResponse(conn, Response{
			ID:     req.ID,
			Result: res,
			Error:  rpcErr,
		})
	}
}

// writeTimeout bounds every IPC write: a client that stops reading (stalled
// GUI, wedged watcher) must not be able to block broadcasts for everyone
// else for unbounded time.
const writeTimeout = 2 * time.Second

// writeWithTimeout performs one write under a write deadline and closes the
// connection when it times out or errors. The reader goroutine's deferred
// cleanup removes it from the client set.
func (s *Server) writeWithTimeout(conn net.Conn, data []byte) error {
	defer func() { _ = conn.SetWriteDeadline(time.Time{}) }()

	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, err := conn.Write(data)
	if err != nil {
		_ = conn.Close() // wedged or gone; reader loop notices and exits
	}
	return err
}

func (s *Server) sendResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_ = s.writeWithTimeout(conn, data)
}

func (s *Server) sendEvent(conn net.Conn, eventName string, data any) {
	if conn == nil {
		return
	}
	evt := Event{
		Event: eventName,
		Data:  data,
	}
	msg, err := json.Marshal(evt)
	if err != nil {
		return
	}
	msg = append(msg, '\n')
	_ = s.writeWithTimeout(conn, msg)
}

// maxListStream bytes bound per-item content/rich-preview fields in list and
// search responses so a 200-item payload stays far below the 10 MiB IPC
// line limit even with multi-hundred-KB clips. Full text keeps coming from
// the id-based copy/paste/get/transform paths.
const (
	maxListStreamContent = 16 * 1024
	maxListStreamRich    = 96 * 1024
)

func trimToMax(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	// Back off past any cut-through UTF-8 continuation bytes.
	for len(cut) > 0 && (cut[len(cut)-1]&0xC0) == 0x80 {
		cut = cut[:len(cut)-1]
	}
	return cut + "\u2026"
}

func slimForList(items []store.Item) []store.Item {
	for i := range items {
		items[i].Content = trimToMax(items[i].Content, maxListStreamContent)
		if len(items[i].RichPreview) > maxListStreamRich {
			// Do not cut rich markup mid-tag with trimToMax: slicing HTML
			// bytes produces unclosed spans that choke the Qt RichText engine.
			// When markup exceeds the streaming cap, clear it so the client
			// cleanly falls back to plain text.
			items[i].RichPreview = ""
		}
	}
	return items
}

func (s *Server) Broadcast(eventName string, data any) {
	msg, err := json.Marshal(Event{Event: eventName, Data: data})
	if err != nil {
		return
	}
	msg = append(msg, '\n')

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for conn := range s.clients {
		_ = s.writeWithTimeout(conn, msg)
	}
}

func (s *Server) dispatch(conn net.Conn, method string, params json.RawMessage) (any, *RPCError) {
	switch method {
	case "ping":
		return map[string]any{"pong": true}, nil
	case "status":
		return s.handleStatus()
	case "list":
		return s.handleList(params)
	case "search":
		return s.handleSearch(params)
	case "query":
		return s.handleQuery(params)
	case "get":
		return s.handleGet(params)
	case "pin":
		return s.handlePin(params)
	case "delete":
		return s.handleDelete(params)
	case "clear":
		return s.handleClear(params)
	case "pause":
		return s.handlePause(params)
	case "copy":
		return s.handleCopy(params)
	case "paste":
		return s.handlePaste(conn, params)
	case "transform":
		return s.handleTransform(params)
	case "merge":
		return s.handleMerge(params)
	default:
		return nil, &RPCError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", method)}
	}
}

func (s *Server) handleStatus() (any, *RPCError) {
	total, pinned, sizeBytes, err := s.store.GetStats()
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: err.Error()}
	}

	return map[string]any{
		"capabilities":   []string{"query"},
		"version":        Version,
		"paused":         s.isPaused(),
		"total_items":    total,
		"pinned_items":   pinned,
		"db_size_bytes":  sizeBytes,
		"uptime_seconds": int64(time.Since(s.startTime).Seconds()),
	}, nil
}

// isPaused reads pause state from the capturer — the single source of truth
// (excluding this file avoids a second mutable copy and its data race).
func (s *Server) isPaused() bool {
	if s.capturer == nil {
		return false
	}
	return s.capturer.IsPaused()
}

type ListParams struct {
	Limit  int  `json:"limit"`
	Offset int  `json:"offset"`
	Pinned bool `json:"pinned"`
}

func (s *Server) handleList(params json.RawMessage) (any, *RPCError) {
	var p ListParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	if p.Limit <= 0 {
		p.Limit = 50
	}

	items, total, err := s.store.ListItems(p.Limit, p.Offset)
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: err.Error()}
	}

	return map[string]any{
		"items": slimForList(items),
		"total": total,
	}, nil
}

type SearchParams struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func (s *Server) handleSearch(params json.RawMessage) (any, *RPCError) {
	var p SearchParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	if p.Limit <= 0 {
		p.Limit = 50
	}

	items, total, err := s.store.SearchItems(p.Query, p.Limit, p.Offset)
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: err.Error()}
	}

	return map[string]any{
		"items": slimForList(items),
		"total": total,
	}, nil
}

type GetParams struct {
	ID string `json:"id"`
}

func (s *Server) handleGet(params json.RawMessage) (any, *RPCError) {
	var p GetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
	}

	item, err := s.store.GetItem(p.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &RPCError{Code: -32004, Message: "Item not found"}
		}
		return nil, &RPCError{Code: -32000, Message: err.Error()}
	}

	return map[string]any{"item": item}, nil
}

type PinParams struct {
	ID     string `json:"id"`
	Pinned bool   `json:"pinned"`
}

func (s *Server) handlePin(params json.RawMessage) (any, *RPCError) {
	var p PinParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
	}

	if err := s.store.PinItem(p.ID, p.Pinned); err != nil {
		return nil, &RPCError{Code: -32000, Message: err.Error()}
	}

	s.Broadcast("pin_changed", map[string]any{"id": p.ID, "pinned": p.Pinned})
	return map[string]any{"success": true}, nil
}

type DeleteParams struct {
	ID string `json:"id"`
}

func (s *Server) handleDelete(params json.RawMessage) (any, *RPCError) {
	var p DeleteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
	}

	if err := s.store.DeleteItem(p.ID); err != nil {
		return nil, &RPCError{Code: -32000, Message: err.Error()}
	}

	s.Broadcast("item_deleted", map[string]any{"id": p.ID})
	return map[string]any{"success": true}, nil
}

type ClearParams struct {
	IncludePinned bool `json:"include_pinned"`
}

func (s *Server) handleClear(params json.RawMessage) (any, *RPCError) {
	var p ClearParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}

	deleted, err := s.store.Clear(p.IncludePinned)
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: err.Error()}
	}

	s.Broadcast("history_cleared", map[string]any{"deleted": deleted})
	return map[string]any{"success": true, "deleted": deleted}, nil
}

type PauseParams struct {
	Paused bool `json:"paused"`
}

func (s *Server) handlePause(params json.RawMessage) (any, *RPCError) {
	var p PauseParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
	}

	if s.capturer != nil {
		s.capturer.SetPaused(p.Paused)
	}

	s.Broadcast("pause_changed", map[string]any{"paused": p.Paused})
	return map[string]any{"paused": p.Paused}, nil
}

type TransformParams struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Action string `json:"action"`
}

func (s *Server) handleTransform(params json.RawMessage) (any, *RPCError) {
	var p TransformParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
	}

	text := p.Text
	if text == "" && p.ID != "" {
		item, err := s.store.GetItem(p.ID)
		if err != nil || item == nil {
			return nil, &RPCError{Code: -32004, Message: "Item not found"}
		}
		text = item.Content
	}

	res, err := transform.Transform(text, p.Action)
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: err.Error()}
	}

	return map[string]any{"text": res}, nil
}

type MergeParams struct {
	IDs       []string `json:"ids"`
	Separator string   `json:"separator"`
	Action    string   `json:"action"` // "preview", "copy", "paste"
}

func (s *Server) handleMerge(params json.RawMessage) (any, *RPCError) {
	var p MergeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
	}

	if len(p.IDs) == 0 {
		return nil, &RPCError{Code: -32602, Message: "No IDs provided to merge"}
	}

	sep := p.Separator
	if sep == "" {
		sep = "\n"
	}

	var parts []string
	for _, id := range p.IDs {
		item, err := s.store.GetItem(id)
		if err == nil && item != nil {
			parts = append(parts, item.Content)
		}
	}

	merged := strings.Join(parts, sep)

	if p.Action == "copy" || p.Action == "paste" {
		if err := copyToClipboard(merged, "text/plain"); err != nil {
			return nil, &RPCError{Code: -32000, Message: "Copy failed: " + err.Error()}
		}
		if p.Action == "paste" {
			go func() {
				_ = hypr.PasteDirectly()
			}()
		}
	}

	return map[string]any{"text": merged}, nil
}

type CopyParams struct {
	ID        string `json:"id"`
	Plain     bool   `json:"plain"`
	Transform string `json:"transform"`
}

func (s *Server) handleCopy(params json.RawMessage) (any, *RPCError) {
	var p CopyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
	}

	item, err := s.store.GetItem(p.ID)
	if err != nil || item == nil {
		return nil, &RPCError{Code: -32004, Message: "Item not found"}
	}

	content := item.Content
	if p.Transform != "" {
		t, err := transform.Transform(content, p.Transform)
		if err == nil {
			content = t
		}
	}

	if item.Type == "image" && item.BlobPath != "" {
		if err := copyFileToClipboard(item.BlobPath, item.Mime); err != nil {
			return nil, &RPCError{Code: -32000, Message: "Copy image failed: " + err.Error()}
		}
	} else {
		mime := "text/plain"
		if !p.Plain && item.Mime != "" && item.Mime != "text/plain" {
			mime = item.Mime
		}
		if err := copyToClipboard(content, mime); err != nil {
			return nil, &RPCError{Code: -32000, Message: "Copy failed: " + err.Error()}
		}
	}

	return map[string]any{"success": true}, nil
}

type PasteParams struct {
	ID        string `json:"id"`
	Plain     bool   `json:"plain"`
	Transform string `json:"transform"`
}

func (s *Server) handlePaste(conn net.Conn, params json.RawMessage) (any, *RPCError) {
	var p PasteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
	}

	// First copy
	copyParamBytes, _ := json.Marshal(CopyParams{
		ID:        p.ID,
		Plain:     p.Plain,
		Transform: p.Transform,
	})
	if _, rpcErr := s.handleCopy(copyParamBytes); rpcErr != nil {
		return nil, rpcErr
	}

	// Dispatch paste directly
	go func() {
		if err := hypr.PasteDirectly(); err != nil {
			s.sendEvent(conn, "paste_failed", map[string]any{"id": p.ID, "message": "Copied; paste injection unavailable. Paste manually with Ctrl+V."})
		}
	}()

	return map[string]any{"success": true, "copied": true, "paste_requested": true}, nil
}

func copyToClipboard(text, mime string) error {
	if session, err := desktop.DetectSession(os.Getenv); err == nil && session == "x11" {
		return desktop.NewX11Backend().Write(context.Background(), desktop.Selection{MIME: mime, Data: []byte(text)})
	}
	cmd := exec.Command("wl-copy", "--type", mime)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func copyFileToClipboard(filePath, mime string) error {
	if session, err := desktop.DetectSession(os.Getenv); err == nil && session == "x11" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		return desktop.NewX11Backend().Write(context.Background(), desktop.Selection{MIME: mime, Data: data})
	}
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := exec.Command("wl-copy", "--type", mime)
	cmd.Stdin = f
	return cmd.Run()
}
