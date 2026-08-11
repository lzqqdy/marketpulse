package equity

import "testing"

func TestUSEquityDefs(t *testing.T) {
	cases := []struct {
		id          string
		financeType string
		tencent     string
		wantBaidu   bool
	}{
		// QQQ/SPY skip Baidu: ETF snapshots round price/ratio and starve Tencent.
		{"us-qqq", "index", "s_usQQQ", false},
		{"us-spy", "index", "s_usSPY", false},
		{"us-aapl", "stock", "s_usAAPL", true},
		{"us-nvda", "stock", "s_usNVDA", true},
		{"us-mu", "stock", "s_usMU", true},
		{"us-avgo", "stock", "s_usAVGO", true},
		{"us-tsm", "stock", "s_usTSM", true},
		{"us-skhy", "stock", "s_usSKHY", true},
	}
	for _, tc := range cases {
		def, ok := DefaultIndexByID(tc.id)
		if !ok {
			t.Fatalf("missing def %s", tc.id)
		}
		if got := def.resolvedBaiduFinanceType(); got != tc.financeType {
			t.Fatalf("%s financeType: got %q want %q", tc.id, got, tc.financeType)
		}
		if def.TencentSymbol != tc.tencent {
			t.Fatalf("%s tencent: got %q want %q", tc.id, def.TencentSymbol, tc.tencent)
		}
		if def.HasBaidu() != tc.wantBaidu {
			t.Fatalf("%s HasBaidu: got %v want %v", tc.id, def.HasBaidu(), tc.wantBaidu)
		}
		if _, ok := ActiveWindows[tc.id]; !ok {
			t.Fatalf("%s missing ActiveWindows entry", tc.id)
		}
	}
}

func TestResolveDefsIncludesUSEquities(t *testing.T) {
	defs := ResolveDefs([]string{"dji", "us-qqq", "us-aapl"})
	if len(defs) != 3 {
		t.Fatalf("len=%d want 3", len(defs))
	}
	if defs[1].ID != "us-qqq" || defs[1].HasBaidu() {
		t.Fatalf("us-qqq should skip Baidu, got %#v", defs[1])
	}
	if defs[2].ID != "us-aapl" || defs[2].resolvedBaiduFinanceType() != "stock" {
		t.Fatalf("unexpected %#v", defs[2])
	}
}
