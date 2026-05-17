package equity

import (
	"strings"
	"testing"
)

func TestParseStooqCSV(t *testing.T) {
	raw := `Date,Open,High,Low,Close,Volume
2026-05-14,100,101,99,100,123
2026-05-15,100,103,98,102,456
`
	rows, err := parseStooqCSV(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("len=%d", len(rows))
	}
	if rows[1].close != 102 {
		t.Fatalf("latest close=%v", rows[1].close)
	}
}
