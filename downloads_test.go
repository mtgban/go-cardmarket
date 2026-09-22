package cardmarket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The published files answer 403 with an XML error body for a game whose
// catalog the site has not built yet - Gundam's price guide did, for weeks
// after the game appeared. Decoding that body reports a stray '<', which
// reads as a bug in this package rather than as the missing file it is, so
// the status is checked before the body is ever read.
func TestGetDownloadStatusHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{"403 with an XML error body is an error", http.StatusForbidden,
			`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code></Error>`, true},
		{"404 is an error", http.StatusNotFound, "", true},
		{"200 decodes normally", http.StatusOK, `{"version":1,"products":[]}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			resp, err := getDownload(context.Background(), server.URL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getDownload() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				// The status is what the caller needs to see, not a
				// complaint about the body's first character.
				if strings.Contains(err.Error(), "invalid character") {
					t.Errorf("error blames the body, not the status: %v", err)
				}
				return
			}
			resp.Body.Close()
		})
	}
}
