package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client manages a persistent LSP session.
type Client struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	counter int64
	pending map[int64]chan *Response
	mu      sync.Mutex
}

func NewClient(binary string, args ...string) (*Client, error) {
	cmd := exec.Command(binary, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		pending: make(map[int64]chan *Response),
	}

	go c.readLoop()

	return c, nil
}

func (c *Client) readLoop() {
	reader := bufio.NewReader(c.stdout)
	for {
		// LSP uses Content-Length headers
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		var contentLength int
		if _, err := fmt.Sscanf(line, "Content-Length: %d", &contentLength); err != nil {
			continue
		}

		// Read empty line
		reader.ReadString('\n')

		// Read body
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			return
		}

		var resp Response
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()

		if ok {
			ch <- &resp
		}
	}
}

func (c *Client) Call(ctx context.Context, method string, params interface{}) (*Response, error) {
	id := atomic.AddInt64(&c.counter, 1)
	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Response, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	payload := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)
	if _, err := c.stdin.Write([]byte(payload)); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-ch:
		return resp, nil
	}
}

func (c *Client) Initialize(ctx context.Context, rootURI string) error {
	params := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   rootURI,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"hover": map[string]interface{}{
					"contentFormat": []string{"plaintext"},
				},
			},
		},
	}

	resp, err := c.Call(ctx, "initialize", params)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("LSP error: %s", resp.Error.Message)
	}

	_, err = c.Call(ctx, "initialized", map[string]interface{}{})
	return err
}

func (c *Client) DidOpen(ctx context.Context, uri, languageID, text string) error {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": languageID,
			"version":    1,
			"text":       text,
		},
	}
	_, err := c.Call(ctx, "textDocument/didOpen", params)
	return err
}

func (c *Client) GetHover(ctx context.Context, uri string, line, col int) (string, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": col,
		},
	}

	resp, err := c.Call(ctx, "textDocument/hover", params)
	if err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", fmt.Errorf("LSP error: %s", resp.Error.Message)
	}

	var result struct {
		Contents struct {
			Value string `json:"value"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", err
	}

	return result.Contents.Value, nil
}

func (c *Client) Close() {
	c.stdin.Close()
	c.stdout.Close()
	c.cmd.Process.Kill()
}
