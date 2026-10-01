package cardmarket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

// serveFile answers every request with body, as a published file would.
func serveFile(t *testing.T, body string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// The files date themselves with an offset that has no colon, which is not
// RFC 3339, and the time it names must survive the parse.
func TestPriceGuideFile(t *testing.T) {
	link := serveFile(t, `{"version":1,"createdAt":"2026-09-30T09:55:34+0200",`+
		`"priceGuides":[{"idProduct":908749,"idCategory":1674,"low":1.5,"trend":2}]}`)

	file, err := priceGuideFile(context.Background(), link)
	if err != nil {
		t.Fatalf("priceGuideFile() = %v", err)
	}
	if want := time.Date(2026, 9, 30, 7, 55, 34, 0, time.UTC); !file.CreatedAt.Equal(want) {
		t.Errorf("CreatedAt = %v, want %v", file.CreatedAt, want)
	}
	if file.Version != 1 || len(file.PriceGuides) != 1 || file.PriceGuides[0].LowPrice != 1.5 {
		t.Errorf("file = %+v", file)
	}
}

func TestProductListFile(t *testing.T) {
	link := serveFile(t, `{"version":1,"createdAt":"2026-09-29T13:28:05+0200",`+
		`"products":[{"idProduct":908749,"name":"Starter Deck: Heroic Beginnings"}]}`)

	file, err := productListFile(context.Background(), link)
	if err != nil {
		t.Fatalf("productListFile() = %v", err)
	}
	if want := time.Date(2026, 9, 29, 11, 28, 5, 0, time.UTC); !file.CreatedAt.Equal(want) {
		t.Errorf("CreatedAt = %v, want %v", file.CreatedAt, want)
	}
	if len(file.Products) != 1 || file.Products[0].IDProduct != 908749 {
		t.Errorf("file = %+v", file)
	}
}

// A caller asking for the date means to judge it, so an unreadable one is an
// error there; a caller asking only for the rows never reads it.
func TestUnreadableCreatedAt(t *testing.T) {
	link := serveFile(t, `{"version":1,"createdAt":"yesterday","products":[{"idProduct":1}]}`)

	if _, err := productListFile(context.Background(), link); err == nil {
		t.Error("productListFile() accepted an unreadable createdAt")
	}
	file, err := downloadFile(context.Background(), link)
	if err != nil || len(file.Products) != 1 {
		t.Errorf("downloadFile() = %+v, %v; the rows alone must not need the date", file, err)
	}
}
