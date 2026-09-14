package cardmarket

import "testing"

// Cardmarket caps how far a listing counts: past 1000 matches it reports the
// cap rather than the real total, which ParseContentRange must surface as
// capped rather than as a number a caller could mistake for exact.
func TestParseContentRange(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantTotal  int
		wantCapped bool
		wantOK     bool
	}{
		{"small exact page", "0-29/33", 33, false, true},
		{"single result", "0-0/1", 1, false, true},
		{"capped at the 1000 ceiling", "0-99/1000+", 1000, true, true},
		{"empty header", "", 0, false, false},
		{"missing the total segment", "0-29", 0, false, false},
		{"unparseable total", "0-29/many", 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, capped, ok := ParseContentRange(tt.header)
			if total != tt.wantTotal || capped != tt.wantCapped || ok != tt.wantOK {
				t.Errorf("ParseContentRange(%q) = (%d, %v, %v), want (%d, %v, %v)",
					tt.header, total, capped, ok, tt.wantTotal, tt.wantCapped, tt.wantOK)
			}
		})
	}
}
