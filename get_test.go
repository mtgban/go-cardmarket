package cardmarket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A non-2xx status with an empty body must surface as an error, not read as
// a clean zero-result answer - an edge-level block (403, empty body, no
// APIError JSON) once did exactly that, silently. A 2xx empty body (204, the
// documented shape for "the query matched nothing") must still read clean.
func TestGetEmptyBodyStatusHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{"403 empty body is an error", http.StatusForbidden, "", true},
		{"401 empty body is an error", http.StatusUnauthorized, "", true},
		// 5xx is deliberately not covered here: it's retryablehttp's own
		// retryable-status set, so a 500 test would actually exercise the
		// client's full retry/backoff policy (RetryMax=20, several minutes
		// worst case) rather than the status-check logic under test here.
		// 403/401 aren't retried and already exercise the same code path.
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
			err := mkm.get(context.Background(), server.URL, &out)
			if (err != nil) != tt.wantErr {
				t.Errorf("get() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
