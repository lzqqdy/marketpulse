package equity

import (
	"fmt"
	"testing"
)

func TestTencentKlineSymbol(t *testing.T) {
	cases := []struct {
		id   string
		sym  string
		want string
		ok   bool
	}{
		{"crude", "hf_CL", "usCL", true},
		{"dji", "s_usDJI", "usDJI", true},
		{"sh000001", "s_sh000001", "sh000001", true},
		{"gold", "hf_GC", "", false},
		{"n225", "gzN225", "", false},
	}
	for _, tc := range cases {
		got, ok := tencentKlineSymbol(IndexDef{ID: tc.id, TencentSymbol: tc.sym})
		if ok != tc.ok || got != tc.want {
			t.Fatalf("%s: got %q ok=%v want %q ok=%v", tc.id, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseTencentDayRows(t *testing.T) {
	rows := [][]any{
		{"2026-06-23", "77.10", "76.50", "77.80", "75.90", "12345"},
	}
	candles, err := parseTencentDayRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 1 || candles[0].Close != 76.5 || candles[0].High != 77.8 {
		t.Fatalf("candles=%+v", candles)
	}
}

func TestIsRetryableNetworkErr(t *testing.T) {
	if !isRetryableNetworkErr(fmt.Errorf("Get ...: EOF")) {
		t.Fatal("EOF should be retryable")
	}
	if isRetryableNetworkErr(fmt.Errorf("parse error")) {
		t.Fatal("parse error should not be retryable")
	}
}
