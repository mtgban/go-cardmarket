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
		{"negative total", "0-1/-5", 0, false, false},
		{"signed total", "0-1/+7", 0, false, false},
		{"negative capped total", "0-1/-5+", 0, false, false},
		{"total past int", "0-29/99999999999999999999", 0, false, false},
		{"spaced total", "0-29/ 33", 0, false, false},
		{"no range", "/33", 0, false, false},
		{"start past end", "29-0/33", 0, false, false},
		{"negative start", "-1-29/33", 0, false, false},
		{"signed end", "0-+29/33", 0, false, false},
		{"end past int", "0-99999999999999999999/33", 0, false, false},
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
