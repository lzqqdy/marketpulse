package expressnews

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeItemFromFixture(t *testing.T) {
	raw := loadFixture(t)
	var result expressNewsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content.List) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Content.List))
	}
	first := normalizeItem(result.Content.List[0])
	if first.Title == "" {
		t.Fatal("title empty")
	}
	if first.Body == "" {
		t.Fatal("body empty")
	}
	if first.PublishTime != 1783758335 {
		t.Fatalf("publish time=%d", first.PublishTime)
	}
	if len(first.Entities) != 2 {
		t.Fatalf("entities=%d", len(first.Entities))
	}
	if first.Entities[0].ChangePct != -2 {
		t.Fatalf("change pct=%v", first.Entities[0].ChangePct)
	}
	if first.Entities[1].ChangePct != 5.97 {
		t.Fatalf("change pct=%v", first.Entities[1].ChangePct)
	}
}

func TestTTLStableWhenFingerprintUnchanged(t *testing.T) {
	fp := newFingerprintStore()
	fp.set("A股", "id-1")
	ttl := ttlForPage("A股", 0, "id-1", fp)
	if ttl != ttlStablePage0 {
		t.Fatalf("ttl=%s want stable", ttl)
	}
}

func TestTTLFreshWhenFingerprintChanged(t *testing.T) {
	fp := newFingerprintStore()
	fp.set("A股", "id-1")
	ttl := ttlForPage("A股", 0, "id-2", fp)
	if ttl != ttlFreshPage0 {
		t.Fatalf("ttl=%s want fresh", ttl)
	}
	if fp.get("A股") != "id-2" {
		t.Fatal("fingerprint not updated")
	}
}

func TestTTLHistoryPage(t *testing.T) {
	fp := newFingerprintStore()
	ttl := ttlForPage("", 2, "id-1", fp)
	if ttl != ttlHistoryPage {
		t.Fatalf("ttl=%s want history", ttl)
	}
}

func TestCacheHit(t *testing.T) {
	cache := newResponseCache()
	resp := Response{Tag: "", Pn: 0, Rn: 20, Items: []NewsItem{{ID: "a"}}}
	cache.set("k", resp, time.Minute)
	got, ok := cache.get("k")
	if !ok || len(got.Items) != 1 {
		t.Fatal("cache miss")
	}
}

func TestNormalizeTag(t *testing.T) {
	if NormalizeTag(" A股 ") != "A股" {
		t.Fatal("tag trim failed")
	}
}

func TestPageOffset(t *testing.T) {
	if got := pageOffset(0, 20); got != 0 {
		t.Fatalf("page0=%d", got)
	}
	if got := pageOffset(2, 20); got != 40 {
		t.Fatalf("page2=%d", got)
	}
	if got := pageOffset(3, 15); got != 45 {
		t.Fatalf("page3=%d", got)
	}
}

func loadFixture(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("testdata", "expressnews_sample.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
