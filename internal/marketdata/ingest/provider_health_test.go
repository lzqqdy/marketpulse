package ingest

import (
	"testing"
	"time"
)

func TestDefaultProviderDefsIncludesOnDemandModules(t *testing.T) {
	defs := defaultProviderDefs(true, "bitget", true)
	names := make(map[string]providerDef, len(defs))
	for _, d := range defs {
		names[d.Name] = d
	}
	for _, name := range []string{"baidu_market_center", "baidu_expressnews", "odaily_expressnews"} {
		def, ok := names[name]
		if !ok {
			t.Fatalf("missing provider %s", name)
		}
		if name != "odaily_expressnews" && def.Disabled {
			t.Fatalf("%s should be enabled when baidu is enabled", name)
		}
		if name == "odaily_expressnews" && def.Disabled {
			t.Fatalf("odaily_expressnews should stay enabled")
		}
	}
}

func TestBaiduProvidersDisabledWhenBaiduOff(t *testing.T) {
	defs := defaultProviderDefs(false, "bitget", false)
	for _, d := range defs {
		switch d.Name {
		case "baidu_index", "baidu_market_center", "baidu_expressnews":
			if !d.Disabled {
				t.Fatalf("%s should be disabled", d.Name)
			}
		case "odaily_expressnews":
			if d.Disabled {
				t.Fatalf("odaily_expressnews should remain enabled when baidu is off")
			}
		}
	}
}

func TestProviderHealthReportSuccessAndStale(t *testing.T) {
	store := newProviderHealthStore([]providerDef{
		{Name: "baidu_expressnews", Label: "News", Category: "news", Role: "auxiliary", StaleAfter: time.Minute},
	})
	store.ReportSuccess("baidu_expressnews", 120*time.Millisecond)
	snap := store.Snapshot(time.Now())
	if len(snap.Providers) != 1 || snap.Providers[0].Status != ProviderHealthy {
		t.Fatalf("expected healthy, got %+v", snap.Providers)
	}
	snap = store.Snapshot(time.Now().Add(2 * time.Minute))
	if snap.Providers[0].Status != ProviderStale {
		t.Fatalf("expected stale, got %s", snap.Providers[0].Status)
	}
}

func TestProviderHealthReportFailureBeforeSuccess(t *testing.T) {
	store := newProviderHealthStore([]providerDef{
		{Name: "baidu_market_center", Label: "Center", Category: "market", Role: "auxiliary", StaleAfter: time.Minute},
	})
	store.ReportFailure("baidu_market_center", errSample("upstream timeout"))
	snap := store.Snapshot(time.Now())
	if snap.Providers[0].Status != ProviderUnavailable {
		t.Fatalf("expected unavailable, got %s", snap.Providers[0].Status)
	}
}

type errSample string

func (e errSample) Error() string { return string(e) }
