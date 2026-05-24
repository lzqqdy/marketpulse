package equity

import (
	"testing"
	"time"
)

func cst(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, marketLocation)
}

func TestIsMarketActive(t *testing.T) {
	tests := []struct {
		name string
		id   string
		now  time.Time
		want bool
	}{
		{"a share before open", "sh000001", cst(2026, time.May, 18, 9, 14), false},
		{"a share open", "sh000001", cst(2026, time.May, 18, 9, 15), true},
		{"a share after close", "sh000001", cst(2026, time.May, 18, 15, 16), false},
		{"japan open", "n225", cst(2026, time.May, 18, 7, 45), true},
		{"us night session", "dji", cst(2026, time.May, 18, 22, 0), true},
		{"us after midnight session", "dji", cst(2026, time.May, 19, 3, 0), true},
		{"us monday early closed", "dji", cst(2026, time.May, 18, 3, 0), false},
		{"us saturday early friday session", "dji", cst(2026, time.May, 23, 3, 0), true},
		{"us saturday after close", "dji", cst(2026, time.May, 23, 6, 0), false},
		{"gold short break", "gold", cst(2026, time.May, 18, 5, 30), false},
		{"gold active day", "gold", cst(2026, time.May, 18, 6, 0), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMarketActive(tt.id, tt.now); got != tt.want {
				t.Fatalf("IsMarketActive(%q, %s) = %v, want %v", tt.id, tt.now, got, tt.want)
			}
		})
	}
}

func TestCacheTTL(t *testing.T) {
	def := IndexDef{ID: "hsi"}
	if got := CacheTTL(def, cst(2026, time.May, 18, 10, 0)); got != ActiveTTL {
		t.Fatalf("active ttl = %v", got)
	}
	if got := CacheTTL(def, cst(2026, time.May, 18, 18, 0)); got != InactiveTTL {
		t.Fatalf("inactive ttl = %v", got)
	}
}

func TestNextPollInterval(t *testing.T) {
	defs := []IndexDef{{ID: "sh000001"}, {ID: "dji"}}
	if got := NextPollInterval(defs, cst(2026, time.May, 18, 10, 0)); got != ActiveTTL {
		t.Fatalf("active interval = %v", got)
	}
	if got := NextPollInterval(defs, cst(2026, time.May, 18, 16, 0)); got != time.Hour {
		t.Fatalf("closed interval = %v", got)
	}
	if got := NextPollInterval([]IndexDef{{ID: "sh000001"}}, cst(2026, time.May, 18, 8, 45)); got != 30*time.Minute {
		t.Fatalf("pre-open interval = %v", got)
	}
}
