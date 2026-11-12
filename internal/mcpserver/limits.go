package mcpserver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// refuseBatch answers 400 to a JSON-RPC batch before the SDK reads it. The SDK takes an array of calls from a request
// of protocol 2025-03-26, or of no named version, and answers all of them in one response: 64 KB of calls for tens of
// megabytes. No client of this server needs a batch, so one message per request is the rule for every version.
//
// It reads the whole body (at most MaxRequest bytes, refused with 413 above that) and hands the same bytes on.
func refuseBatch(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxRequest))
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge):
			http.Error(w, fmt.Sprintf("request body exceeds %d bytes", MaxRequest), http.StatusRequestEntityTooLarge)
		case err != nil:
			http.Error(w, "failed to read body", http.StatusBadRequest)
		case isBatch(body):
			http.Error(w, "a JSON-RPC batch is not taken: send one message per request", http.StatusBadRequest)
		default:
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		}
	})
}

// isBatch reports whether a body is a JSON array: its first byte after white space is «[».
func isBatch(body []byte) bool {
	for _, c := range body {
		if c > ' ' {
			return c == '['
		}
	}
	return false
}

// capped holds an answer back until the handler is done and lets it out only when it is at most MaxResponse bytes.
// A larger one is dropped as soon as it passes the limit and the caller gets 500 with none of its bytes: nothing was
// sent yet, so the status is still free. The tools keep their answers far below the limit; this is the gate that holds
// when one does not.
func capped(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := &capWriter{w: w}
		next.ServeHTTP(c, r)
		if c.over {
			http.Error(w, "the answer is larger than this endpoint sends", http.StatusInternalServerError)
			return
		}
		if c.status != 0 {
			w.WriteHeader(c.status)
		}
		_, _ = w.Write(c.buf.Bytes())
	})
}

// capWriter keeps a handler's answer in memory, up to MaxResponse bytes.
type capWriter struct {
	w      http.ResponseWriter
	status int
	buf    bytes.Buffer
	over   bool
}

func (c *capWriter) Header() http.Header { return c.w.Header() }

func (c *capWriter) WriteHeader(status int) {
	if c.status == 0 && status >= http.StatusOK {
		c.status = status
	}
}

func (c *capWriter) Write(p []byte) (int, error) {
	if c.over || c.buf.Len()+len(p) > MaxResponse {
		c.over, c.buf = true, bytes.Buffer{}
		return 0, errors.New("the answer is larger than this endpoint sends")
	}
	return c.buf.Write(p)
}
