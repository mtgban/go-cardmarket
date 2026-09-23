package cardmarket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// The two budgets exist because the two failures are not the same failure,
// and the cost of confusing them was a whole dump: expansion 1645 answered
// 21 straight 5xx over six minutes and took the Pokemon catalog with it.
//
// Retry-After is honoured for both 429 and 503 (retryablehttp's own default
// backoff), so a zero here runs the real client through its real policy at
// full speed rather than sleeping the 2-10 seconds a live one would.
func TestRetryBudgets(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCalls  int
	}{
		// A 429 is the concurrency limiter saying "not yet". It clears,
		// so it is worth the full budget.
		{"rate limit keeps the full budget", http.StatusTooManyRequests, "", rateLimitRetries + 1},
		// A 5xx is the API unwell on this one request. It does not clear.
		{"server error gets the short budget", http.StatusServiceUnavailable, "", serverErrorRetries},
		// The body must not decide whether the status was an error. A
		// 5xx answering the shape being decoded once read as a clean
		// empty result, which in a catalog walk is a shelf recorded
		// with no cards and every card on it quietly unpriced.
		{"server error carrying a decodable body is still an error",
			http.StatusServiceUnavailable, `{"single":[]}`, serverErrorRetries},
		// 503 rather than 500 for both: DefaultBackoff honours
		// Retry-After only for 429 and 503, so any other 5xx would make
		// this sleep through the real 2-4-8 second backoff for a code
		// path that is identical either way.
		{"server error carrying an error page is still an error",
			http.StatusServiceUnavailable, `<html>oops</html>`, serverErrorRetries},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(tt.statusCode)
				fmt.Fprint(w, tt.body)
			}))
			defer server.Close()

			mkm := NewClient("test-token", "test-secret")
			var out struct {
				Single []Product `json:"single"`
			}
			_, err := mkm.get(context.Background(), server.URL, &out)
			if err == nil {
				t.Fatal("get() succeeded, want an error")
			}
			if got := int(calls.Load()); got != tt.wantCalls {
				t.Errorf("made %d requests, want %d", got, tt.wantCalls)
			}
			// The caller has to be able to tell the two apart, which is
			// the whole point of spending different budgets on them.
			if !strings.Contains(err.Error(), http.StatusText(tt.statusCode)) {
				t.Errorf("error does not name the status: %v", err)
			}
			// Naming the count proves the error came through the
			// give-up path rather than from some later check that
			// happens to reject this response for its own reasons.
			want := fmt.Sprintf("after %d attempt(s)", tt.wantCalls)
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not say %q: %v", want, err)
			}
		})
	}
}

// A 5xx that clears is still retried - the short budget is a ceiling, not a
// refusal to try twice.
func TestServerErrorRecovers(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < serverErrorRetries {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	mkm := NewClient("test-token", "test-secret")
	var out struct {
		OK bool `json:"ok"`
	}
	if _, err := mkm.get(context.Background(), server.URL, &out); err != nil {
		t.Fatalf("get() = %v, want success", err)
	}
	if !out.OK {
		t.Error("body did not decode after the retries")
	}
}

// A request made without a tally must not be refused outright - it falls
// back to the single budget rather than failing on the first 5xx.
func TestCheckRetryWithoutTally(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusInternalServerError}
	retry, err := checkRetry(context.Background(), resp, nil)
	if err != nil {
		t.Fatalf("checkRetry() error = %v", err)
	}
	if !retry {
		t.Error("checkRetry() = false without a tally, want true")
	}
}
