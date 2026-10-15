package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
	reqSeq atomic.Int64
}

func NewClient(socketPath string) (*Client, error) {
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial unix socket %s: %w", socketPath, err)
	}

	return &Client{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) Call(method string, params any, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	reqID := fmt.Sprintf("req-%d", c.reqSeq.Add(1))
	var paramsRaw json.RawMessage
	if params != nil {
		pBytes, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		paramsRaw = pBytes
	}

	req := Request{
		ID:     reqID,
		Method: method,
		Params: paramsRaw,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	_ = c.conn.SetDeadline(time.Now().Add(5 * time.Second))
	defer func() {
		_ = c.conn.SetDeadline(time.Time{})
	}()

	if _, err := c.conn.Write(reqBytes); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}

		// Could be an asynchronous broadcast event, skip if it has "event" instead of "id"
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		if _, isEvent := raw["event"]; isEvent {
			// Skip event line when waiting for method response
			continue
		}

		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}

		if resp.ID != reqID {
			continue
		}

		if resp.Error != nil {
			return fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}

		if result != nil && resp.Result != nil {
			resBytes, err := json.Marshal(resp.Result)
			if err != nil {
				return fmt.Errorf("remarshal result: %w", err)
			}
			if err := json.Unmarshal(resBytes, result); err != nil {
				return fmt.Errorf("unmarshal result into target: %w", err)
			}
		}

		return nil
	}
}
