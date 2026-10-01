package cardmarket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A non-2xx status with an empty body must surface as an error, not read as
// a clean zero-result answer, which is what an edge-level block (403, empty
// body, no APIError JSON) looks like. A 2xx empty body (204, the documented
// shape for "the query matched nothing") must still read clean.
func TestGetEmptyBodyStatusHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{"403 empty body is an error", http.StatusForbidden, "", true},
		{"401 empty body is an error", http.StatusUnauthorized, "", true},
		// 5xx is retried, so TestRetryBudgets covers it; 401 and 403 reach
		// the same check without the backoff.
		{"204 empty body is a clean success", http.StatusNoContent, "", false},
		{"200 empty body is a clean success", http.StatusOK, "", false},
		{"403 with an APIError body still decodes as that error", http.StatusForbidden,
			`{"mkm_error_description":"nope","http_status_code":403}`, true},
		{"200 with a real body decodes normally", http.StatusOK, `{"ok":true}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			mkm := NewClient("test-token", "test-secret")
			var out struct {
				OK bool `json:"ok"`
			}
			_, err := mkm.get(context.Background(), server.URL, &out)
			if (err != nil) != tt.wantErr {
				t.Errorf("get() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// A body that does not decode is reported by its status and the decode
// error, with only the start of the body: an HTML page must not become the
// whole error message.
func TestGetUndecodableBody(t *testing.T) {
	page := "<html>" + strings.Repeat("x", 10000) + "</html>"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer server.Close()

	var out struct {
		OK bool `json:"ok"`
	}
	_, err := NewClient("test-token", "test-secret").get(context.Background(), server.URL, &out)
	if err == nil {
		t.Fatal("get() succeeded, want an error")
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("error does not wrap the decode error: %v", err)
	}
	if !strings.Contains(err.Error(), "200 OK") {
		t.Errorf("error does not name the status: %v", err)
	}
	if len(err.Error()) > 400 {
		t.Errorf("error carries %d bytes, want the body cut short", len(err.Error()))
	}
}
