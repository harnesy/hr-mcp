package mcpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAnswerNeverExceedsMaxResponse: whatever the handler behind the limit writes, an answer above MaxResponse does not
// leave: the caller gets an error status and none of the answer's bytes.
func TestAnswerNeverExceedsMaxResponse(t *testing.T) {
	answer := func(n int) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			chunk := bytes.Repeat([]byte("a"), 4096)
			for left := n; left > 0; left -= min(left, len(chunk)) {
				_, _ = w.Write(chunk[:min(left, len(chunk))])
			}
		})
	}
	for _, c := range []struct {
		name   string
		bytes  int
		status int
	}{
		{"a small answer", 10, http.StatusOK},
		{"an answer of exactly the limit", MaxResponse, http.StatusOK},
		{"one byte over the limit", MaxResponse + 1, http.StatusInternalServerError},
		{"many times the limit", 70 * MaxResponse, http.StatusInternalServerError},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			capped(answer(c.bytes)).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
			body := rec.Body.String()
			if rec.Code != c.status || len(body) > MaxResponse {
				t.Fatalf("status %d with %d bytes, want %d and at most %d bytes", rec.Code, len(body), c.status, MaxResponse)
			}
			if c.status == http.StatusOK && (len(body) != c.bytes || rec.Header().Get("Content-Type") != "application/json") {
				t.Fatalf("an answer within the limit must pass whole: %d of %d bytes, Content-Type %q", len(body), c.bytes, rec.Header().Get("Content-Type"))
			}
			if c.status != http.StatusOK && (strings.Contains(body, "aaaa") || strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json")) {
				t.Fatalf("an answer over the limit must not leave in part: %q (%s)", body, rec.Header().Get("Content-Type"))
			}
		})
	}
}
