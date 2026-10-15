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
	"strings"
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
	// AgentCmd is the executable. When it is empty, ZeroBin is started with
	// the single argument acp (the historical launcher).
	AgentCmd  string
	AgentArgs []string
	// ZeroBin is the legacy launcher name. Empty means "zero" when AgentCmd
	// is also empty.
	ZeroBin  string
	ExtraEnv []string
	Stderr   io.Writer // optional; defaults to limited capture
	// Timeout limits the lifetime of the child context; 0 means no timeout.
	Timeout time.Duration
	// Notify and Inbound are applied before the read loop starts (avoids races with early notifications).
	Notify  NotifyFunc
	Inbound InboundHandler
}

// Client speaks JSON-RPC 2.0 (newline-delimited) to an ACP child process.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *limitedWriter

	writeMu sync.Mutex
	closed  atomic.Bool

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan rpcMessage

	handlerMu sync.RWMutex
	notify    NotifyFunc
	inbound   InboundHandler
	readDone  chan struct{}
	readErr   error

	ctx    context.Context
	cancel context.CancelFunc

	inboundSem  chan struct{}
	inboundWG   sync.WaitGroup
	inboundCtx  context.Context
	inboundStop context.CancelFunc

	state agentState
}

// Start launches the agent and begins reading stdout in the background.
// The child inherits the parent environment except ZERO_BRIDGE_TOKEN.
// ExtraEnv may add agent configuration. Argv is only the resolved path
// plus AgentArgs, never a shell string and never the bridge token.
func childEnv(extra []string) []string {
	env := make([]string, 0, len(os.Environ())+len(extra))
	keep := func(e string) bool {
		return !strings.HasPrefix(e, "ZERO_BRIDGE_TOKEN=")
	}
	for _, e := range os.Environ() {
		if keep(e) {
			env = append(env, e)
		}
	}
	for _, e := range extra {
		if keep(e) {
			env = append(env, e)
		}
	}
	return env
}

func Start(parent context.Context, opts Options) (*Client, error) {
	bin, args := opts.AgentCmd, opts.AgentArgs
	if bin == "" {
		bin = opts.ZeroBin
		if bin == "" {
			bin = "zero"
		}
		args = []string{"acp"}
	}
	resolved, err := ResolveCommand(bin, args)
	if err != nil {
		return nil, fmt.Errorf("acpclient: %w", err)
	}
	var ctx context.Context
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, opts.Timeout)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}
	started := false
	defer func() {
		if !started {
			cancel()
		}
	}()

	cmd := exec.CommandContext(ctx, resolved.Path, resolved.Args...)
	cmd.Env = childEnv(opts.ExtraEnv)

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
		return nil, fmt.Errorf("acpclient: start agent: %w", err)
	}
	started = true

	var inbound InboundHandler = DefaultInboundHandler{}
	if opts.Inbound != nil {
		inbound = opts.Inbound
	}
	inCtx, inStop := context.WithCancel(context.Background())
	c := &Client{
		cmd:         cmd,
		stdin:       stdin,
		stderr:      lw,
		pending:     make(map[int64]chan rpcMessage),
		inbound:     inbound,
		readDone:    make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
		inboundSem:  make(chan struct{}, 16),
		inboundCtx:  inCtx,
		inboundStop: inStop,
	}
	if opts.Notify != nil {
		c.handlerMu.Lock()
		c.notify = opts.Notify
		c.handlerMu.Unlock()
	}
	go c.readLoop(ctx, stdout)
	return c, nil
}

// SetNotifyHandler registers a callback for inbound notifications.
// Prefer Options.Notify at Start when notifications may arrive immediately after launch.
func (c *Client) SetNotifyHandler(fn NotifyFunc) {
	c.handlerMu.Lock()
	c.notify = fn
	c.handlerMu.Unlock()
}

// SetInboundHandler overrides permission/fs handling (tests may inject).
func (c *Client) SetInboundHandler(h InboundHandler) {
	if h != nil {
		c.handlerMu.Lock()
		c.inbound = h
		c.handlerMu.Unlock()
	}
}

// Done is closed when the agent stdout reader exits.
func (c *Client) Done() <-chan struct{} { return c.readDone }

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
	return c.recvResponse(ctx, ch, id, method, params, result)
}

func (c *Client) recvResponse(ctx context.Context, ch chan rpcMessage, id int64, method string, params any, result any) error {
	select {
	case <-ctx.Done():
		c.dropPending(id)
		return ctx.Err()
	case msg, ok := <-ch:
		if !ok {
			return errors.New("acpclient: connection closed")
		}
		return c.finishCall(method, params, msg, result)
	case <-c.readDone:
		select {
		case msg, ok := <-ch:
			if !ok {
				return errors.New("acpclient: connection closed")
			}
			return c.finishCall(method, params, msg, result)
		default:
			if c.readErr != nil {
				return c.readErr
			}
			return fmt.Errorf("acpclient: agent closed stdout: %w", io.ErrUnexpectedEOF)
		}
	}
}

func (c *Client) finishCall(method string, params any, msg rpcMessage, result any) error {
	if msg.Error != nil {
		return msg.Error
	}
	if result != nil && len(msg.Result) > 0 {
		if err := json.Unmarshal(msg.Result, result); err != nil {
			return fmt.Errorf("acpclient: decode result: %w", err)
		}
	}
	c.noteResult(method, params, msg.Result)
	return nil
}

// Close shuts down the child process and waits for the read loop to finish.
func (c *Client) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	if c.inboundStop != nil {
		c.inboundStop()
	}
	_ = c.stdin.Close()
	c.inboundWG.Wait()

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
			return fmt.Errorf("acpclient: agent timed out: %w", waitErr)
		}
		return fmt.Errorf("acpclient: agent exited: %w", waitErr)
	}
	return nil
}

// AttachPipes connects an existing command's stdio (used by tests with a fake agent).
func AttachPipes(stdout io.Reader, stdin io.WriteCloser, cmd *exec.Cmd) *Client {
	return attachPipes(stdout, stdin, cmd, Options{})
}

func attachPipes(stdout io.Reader, stdin io.WriteCloser, cmd *exec.Cmd, opts Options) *Client {
	var inbound InboundHandler = DefaultInboundHandler{}
	if opts.Inbound != nil {
		inbound = opts.Inbound
	}
	inCtx, inStop := context.WithCancel(context.Background())
	c := &Client{
		cmd:         cmd,
		stdin:       stdin,
		stderr:      &limitedWriter{limit: 1 << 20},
		pending:     make(map[int64]chan rpcMessage),
		inbound:     inbound,
		readDone:    make(chan struct{}),
		inboundSem:  make(chan struct{}, 16),
		inboundCtx:  inCtx,
		inboundStop: inStop,
	}
	if opts.Notify != nil {
		c.handlerMu.Lock()
		c.notify = opts.Notify
		c.handlerMu.Unlock()
	}
	go c.readLoop(c.inboundCtx, stdout)
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
	pr, pw := io.Pipe()
	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		_, err := io.Copy(pw, r)
		_ = pw.CloseWithError(err)
	}()
	go func() {
		select {
		case <-ctx.Done():
			_ = pw.CloseWithError(ctx.Err())
		case <-copyDone:
		}
	}()
	reader := bufio.NewReader(pr)
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
			if fn := c.getNotify(); fn != nil {
				fn(msg.Method, msg.Params)
			}
		case msg.isRequest():
			c.dispatchInbound(msg)
		case msg.isResponse():
			id, ok := parseID(msg.ID)
			if ok {
				c.deliverResponse(id, msg)
			}
		}
	}
}

func (c *Client) getNotify() NotifyFunc {
	c.handlerMu.RLock()
	fn := c.notify
	c.handlerMu.RUnlock()
	return fn
}

func (c *Client) getInbound() InboundHandler {
	c.handlerMu.RLock()
	h := c.inbound
	c.handlerMu.RUnlock()
	return h
}

func (c *Client) dispatchInbound(msg rpcMessage) {
	select {
	case c.inboundSem <- struct{}{}:
		c.inboundWG.Add(1)
		go func() {
			defer c.inboundWG.Done()
			defer func() { <-c.inboundSem }()
			c.handleInbound(msg)
		}()
	default:
		_ = c.writeMessage(rpcMessage{
			JSONRPC: jsonRPCVersion,
			ID:      msg.ID,
			Error:   &rpcError{Code: -32000, Message: "busy"},
		})
	}
}

func (c *Client) handleInbound(msg rpcMessage) {
	result, err := c.getInbound().HandleRequest(msg.Method, msg.Params)
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
