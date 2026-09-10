package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// lineStdioTransport implements github.com/gomcpgo/mcp/pkg/transport.Transport.
//
// It decodes JSON-RPC messages one complete top-level value at a time, using
// string- and escape-aware brace matching instead of assuming one message per
// line. This means:
//
//   - compact single-line messages (what real MCP clients send) work, and
//   - pretty-printed multi-line messages also work, because a message is
//     bounded by its balanced braces, not by newlines.
//   - Recovery: a malformed message is answered with a JSON-RPC parse-error
//     response, and because we only consume the exact byte span of that one
//     (balanced) object, the following messages are still processed. A single
//     bad message can never wedge the transport the way a single streaming
//     decoder over the whole stream would.
type lineStdioTransport struct {
	reader  *bufio.Reader
	encoder *json.Encoder

	stopOnce sync.Once

	requests  chan *protocol.Request
	responses chan *protocol.Response
	errors    chan error

	done chan struct{}
}

// maxBuffer bounds how much we accumulate while waiting for a complete value,
// guarding against unbounded growth from a value that never closes.
const maxBuffer = 16 * 1024 * 1024

func newLineStdioTransport() *lineStdioTransport {
	return &lineStdioTransport{
		reader:    bufio.NewReader(os.Stdin),
		encoder:   json.NewEncoder(os.Stdout),
		requests:  make(chan *protocol.Request),
		responses: make(chan *protocol.Response),
		errors:    make(chan error, 16),
		done:      make(chan struct{}),
	}
}

func (t *lineStdioTransport) finish() {
	t.stopOnce.Do(func() {
		close(t.done)
		close(t.requests)
		close(t.responses)
		close(t.errors)
	})
}

func (t *lineStdioTransport) Start(_ context.Context) error {
	go t.readLoop()
	return nil
}

func (t *lineStdioTransport) Stop(_ context.Context) error {
	t.finish()
	return nil
}

func (t *lineStdioTransport) write(v interface{}) error {
	return t.encoder.Encode(v)
}

func (t *lineStdioTransport) Send(response *protocol.Response) error { return t.write(response) }

func (t *lineStdioTransport) SendNotification(notification *protocol.Notification) error {
	return t.write(notification)
}

func (t *lineStdioTransport) SendRequest(request *protocol.Request) error {
	return t.write(request)
}

func (t *lineStdioTransport) Receive() <-chan *protocol.Request { return t.requests }

func (t *lineStdioTransport) Responses() <-chan *protocol.Response { return t.responses }

func (t *lineStdioTransport) Errors() <-chan error { return t.errors }

func (t *lineStdioTransport) doneClosed() bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

// readLoop pulls bytes from stdin and carves them into complete messages.
// It always advances past a delimited value (valid or not), so it cannot get
// stuck on bad input.
func (t *lineStdioTransport) readLoop() {
	defer t.finish()
	buf := make([]byte, 0, 64*1024)
	for {
		if t.doneClosed() {
			return
		}
		candidate, rest, complete := extractNext(buf)
		if complete {
			buf = rest
			t.handleValue(candidate)
			continue
		}
		// Incomplete (or only whitespace so far): read more.
		tmp := make([]byte, 8192)
		n, rerr := t.reader.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if rerr == io.EOF {
			// No more input. Anything left is a truncated value.
			if bytes.TrimSpace(buf) != nil {
				_ = t.Send(respError(nil, -32700, "Parse error: incomplete JSON (unexpected EOF)"))
			}
			return
		}
		if rerr != nil {
			if !errors.Is(rerr, io.EOF) {
				select {
				case t.errors <- rerr:
				case <-t.done:
				}
			}
			return
		}
		if len(buf) > maxBuffer {
			_ = t.Send(respError(nil, -32700, "Parse error: message too large"))
			buf = buf[:0]
		}
	}
}

// handleValue processes one delimited (balanced) candidate: if it is a valid,
// single JSON value it is routed; otherwise a parse error is sent. It never
// blocks the read loop.
func (t *lineStdioTransport) handleValue(candidate []byte) {
	if bytes.TrimSpace(candidate) == nil {
		return
	}
	if !json.Valid(candidate) {
		_ = t.Send(respError(nil, -32700, "Parse error: invalid JSON in message"))
		return
	}
	t.route(string(candidate))
}

// extractNext inspects buf and, if it contains a complete top-level JSON value,
// returns that value, the remaining bytes after it, and complete=true. When buf
// only holds whitespace or a partially-received value, complete is false and
// the value/rest are nil/zero.
func extractNext(buf []byte) (value []byte, rest []byte, complete bool) {
	i := firstNonWS(buf)
	if i == -1 {
		return nil, buf, false // only whitespace so far
	}

	if buf[i] != '{' && buf[i] != '[' {
		// A JSON-RPC message is an object. For anything else (a bare scalar,
		// i.e. not a valid message) fall back to the text up to the next
		// newline, or wait for one. We must not wedge, so this is best-effort.
		nl := bytes.IndexByte(buf[i:], '\n')
		if nl == -1 {
			return nil, buf, false
		}
		end := i + nl
		return buf[i:end+1], buf[end+1:], true
	}

	end := findValueEnd(buf, i)
	if end == -1 {
		return nil, buf, false // still waiting for the closing brace
	}
	return buf[i : end+1], buf[end+1:], true
}

// findValueEnd scans buf starting at the opening '{' or '[' at index i and
// returns the index of the matching closing bracket of that top-level value,
// or -1 if the value is not yet complete within buf. It is aware of string
// literals and escape sequences so brackets inside quoted values do not
// affect depth.
func findValueEnd(buf []byte, i int) int {
	depth := 0
	inString := false
	escaped := false
	for j := i; j < len(buf); j++ {
		c := buf[j]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

func firstNonWS(buf []byte) int {
	for i := 0; i < len(buf); i++ {
		c := buf[i]
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			return i
		}
	}
	return -1
}

func (t *lineStdioTransport) route(raw string) {
	var env struct {
		ID     interface{}     `json:"id"`
		Method string          `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		_ = t.Send(respError(nil, -32700, "Parse error: "+err.Error()))
		return
	}

	if env.Method != "" { // request (or notification)
		var req protocol.Request
		if err := json.Unmarshal([]byte(raw), &req); err != nil {
			_ = t.Send(respError(env.ID, -32600, "Invalid request: "+err.Error()))
			return
		}
		select {
		case t.requests <- &req:
		case <-t.done:
		}
		return
	}

	if len(env.Result) > 0 || len(env.Error) > 0 { // response
		var resp protocol.Response
		if err := json.Unmarshal([]byte(raw), &resp); err != nil {
			_ = t.Send(respError(env.ID, -32600, "Invalid response: "+err.Error()))
			return
		}
		select {
		case t.responses <- &resp:
		case <-t.done:
		}
		return
	}

	_ = t.Send(respError(env.ID, -32600, `Invalid request: expected "method", "result" or "error"`))
}

func respError(id interface{}, code int, msg string) *protocol.Response {
	return &protocol.Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &protocol.Error{Code: code, Message: msg},
	}
}
