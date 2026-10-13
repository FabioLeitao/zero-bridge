package acpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// NotifyFunc receives JSON-RPC notifications from the ACP server (e.g. session/update).
type NotifyFunc func(method string, params json.RawMessage)

// InboundHandler answers JSON-RPC requests from the agent (permission, fs/*).
type InboundHandler interface {
	HandleRequest(method string, params json.RawMessage) (result any, err error)
}

// Options configures how the ACP child process is started.
type Options struct {
	ZeroBin  string
	ExtraEnv []string
	Stderr   io.Writer // optional; defaults to limited capture
}

// Client speaks JSON-RPC 2.0 (newline-delimited) to a `zero acp` child process.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *limitedWriter

	writeMu sync.Mutex
	closed  atomic.Bool

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan rpcMessage

	notify   NotifyFunc
	inbound  InboundHandler
	readDone chan struct{}
	readErr  error

	ctx    context.Context
	cancel context.CancelFunc
}

// Start launches `zero acp` and begins reading stdout in the background.
func Start(parent context.Context, opts Options) (*Client, error) {
	if opts.ZeroBin == "" {
		opts.ZeroBin = "zero"
	}
	ctx, cancel := context.WithTimeout(parent, 120*time.Second)
	started := false
	defer func() {
		if !started {
			cancel()
		}
	}()

	cmd := exec.CommandContext(ctx, opts.ZeroBin, "acp")
	cmd.Env = append(os.Environ(), opts.ExtraEnv...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("acpclient: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("acpclient: stdout pipe: %w", err)
	}
	lw := &limitedWriter{limit: 64 << 10}
	if opts.Stderr != nil {
		cmd.Stderr = io.MultiWriter(lw, opts.Stderr)
	} else {
		cmd.Stderr = lw
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("acpclient: start zero acp: %w", err)
	}
	started = true

	c := &Client{
		cmd:      cmd,
		stdin:    stdin,
		stderr:   lw,
		pending:  make(map[int64]chan rpcMessage),
		inbound:  DefaultInboundHandler{},
		readDone: make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
	}
	go c.readLoop(ctx, stdout)
	return c, nil
}

// SetNotifyHandler registers a callback for inbound notifications.
func (c *Client) SetNotifyHandler(fn NotifyFunc) { c.notify = fn }

// SetInboundHandler overrides permission/fs handling (tests may inject).
func (c *Client) SetInboundHandler(h InboundHandler) {
	if h != nil {
		c.inbound = h
	}
}

// StderrSnippet returns captured stderr from the child (redacted by caller if needed).
func (c *Client) StderrSnippet() string { return c.stderr.String() }

// Call performs a JSON-RPC request and waits for the matching response.
func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	id, ch := c.registerPending()
	req := rpcMessage{JSONRPC: jsonRPCVersion, ID: json.RawMessage(idString(id)), Method: method}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			c.dropPending(id)
			return err
		}
		req.Params = b
	}
	if err := c.writeMessage(req); err != nil {
		c.dropPending(id)
		return err
	}
	return c.recvResponse(ctx, ch, id, result)
}

func (c *Client) recvResponse(ctx context.Context, ch chan rpcMessage, id int64, result any) error {
	select {
	case <-ctx.Done():
		c.dropPending(id)
		return ctx.Err()
	case msg, ok := <-ch:
		if !ok {
			return errors.New("acpclient: connection closed")
		}
		return c.finishCall(msg, result)
	case <-c.readDone:
		select {
		case msg, ok := <-ch:
			if !ok {
				return errors.New("acpclient: connection closed")
			}
			return c.finishCall(msg, result)
		default:
			if c.readErr != nil {
				return c.readErr
			}
			return fmt.Errorf("acpclient: agent closed stdout: %w", io.ErrUnexpectedEOF)
		}
	}
}

func (c *Client) finishCall(msg rpcMessage, result any) error {
	if msg.Error != nil {
		return msg.Error
	}
	if result != nil && len(msg.Result) > 0 {
		if err := json.Unmarshal(msg.Result, result); err != nil {
			return fmt.Errorf("acpclient: decode result: %w", err)
		}
	}
	return nil
}

// Close shuts down the child process and waits for the read loop to finish.
func (c *Client) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	_ = c.stdin.Close()

	if c.cmd == nil {
		<-c.readDone
		return nil
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- c.cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-time.After(2 * time.Second):
		if c.cancel != nil {
			c.cancel()
		}
		waitErr = <-waitCh
	}
	<-c.readDone

	if waitErr != nil && !errors.Is(waitErr, context.Canceled) {
		if errors.Is(waitErr, context.DeadlineExceeded) {
			return fmt.Errorf("acpclient: zero acp timed out: %w", waitErr)
		}
		return fmt.Errorf("acpclient: zero acp exited: %w", waitErr)
	}
	return nil
}

// AttachPipes connects an existing command's stdio (used by tests with a fake agent).
func AttachPipes(stdout io.Reader, stdin io.WriteCloser, cmd *exec.Cmd) *Client {
	c := &Client{
		cmd:      cmd,
		stdin:    stdin,
		stderr:   &limitedWriter{limit: 1 << 20},
		pending:  make(map[int64]chan rpcMessage),
		inbound:  DefaultInboundHandler{},
		readDone: make(chan struct{}),
	}
	go c.readLoop(context.Background(), stdout)
	return c
}

func (c *Client) registerPending() (int64, chan rpcMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpcMessage, 1)
	c.pending[id] = ch
	return id, ch
}

func (c *Client) dropPending(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch, ok := c.pending[id]; ok {
		close(ch)
		delete(c.pending, id)
	}
}

func (c *Client) deliverResponse(id int64, msg rpcMessage) {
	c.mu.Lock()
	ch, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.mu.Unlock()
	if ok {
		ch <- msg
		close(ch)
	}
}

func idString(id int64) string {
	return fmt.Sprintf("%d", id)
}

func parseID(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	return 0, false
}

func (c *Client) writeMessage(msg rpcMessage) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed.Load() {
		return errors.New("acpclient: closed")
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = c.stdin.Write(b)
	return err
}

func (c *Client) readLoop(ctx context.Context, r io.Reader) {
	defer close(c.readDone)
	reader := bufio.NewReader(r)
	for {
		if ctx.Err() != nil {
			c.readErr = ctx.Err()
			return
		}
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				c.readErr = fmt.Errorf("acpclient: agent closed stdout: %w", io.ErrUnexpectedEOF)
			} else {
				c.readErr = err
			}
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			c.readErr = fmt.Errorf("acpclient: invalid json: %w", err)
			return
		}
		switch {
		case msg.isNotify():
			if c.notify != nil {
				c.notify(msg.Method, msg.Params)
			}
		case msg.isRequest():
			c.handleInbound(msg)
		case msg.isResponse():
			id, ok := parseID(msg.ID)
			if ok {
				c.deliverResponse(id, msg)
			}
		}
	}
}

func (c *Client) handleInbound(msg rpcMessage) {
	result, err := c.inbound.HandleRequest(msg.Method, msg.Params)
	var resp rpcMessage
	resp.JSONRPC = jsonRPCVersion
	resp.ID = msg.ID
	if err != nil {
		if re, ok := err.(*rpcError); ok {
			resp.Error = re
		} else {
			resp.Error = &rpcError{Code: -32603, Message: err.Error()}
		}
	} else if result != nil {
		b, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			resp.Error = &rpcError{Code: -32603, Message: marshalErr.Error()}
		} else {
			resp.Result = b
		}
	}
	_ = c.writeMessage(resp)
}

type limitedWriter struct {
	mu    sync.Mutex
	limit int
	buf   bytes.Buffer
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() < w.limit {
		remain := w.limit - w.buf.Len()
		if len(p) > remain {
			p = p[:remain]
		}
		_, _ = w.buf.Write(p)
	}
	return len(p), nil
}

func (w *limitedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}
