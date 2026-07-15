package migrate

import (
	"strings"
	"testing"
)

func TestSplitSQL(t *testing.T) {
	raw := `
CREATE TABLE a (id INT);
CREATE TABLE b (id INT);
`
	got := splitSQL(raw)
	if len(got) != 2 {
		t.Fatalf("got %d stmts, want 2: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "CREATE TABLE a") || !strings.Contains(got[1], "CREATE TABLE b") {
		t.Fatalf("unexpected stmts: %#v", got)
	}
	if len(splitSQL("   ")) != 0 {
		t.Fatal("empty input should yield no statements")
	}
}
