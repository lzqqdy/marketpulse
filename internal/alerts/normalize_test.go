package alerts

import "testing"

func TestNormalizeSymbol_alpha(t *testing.T) {
	got, err := normalizeSymbol(AssetAlpha, "QQQ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "qqq" {
		t.Fatalf("got %q", got)
	}
}
