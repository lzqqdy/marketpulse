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

func TestParseStooqQuoteCSV(t *testing.T) {
	raw := `Symbol,Date,Time,Open,High,Low,Close,Volume
^SPX,2026-05-15,23:00:00,7445.11,7454.85,7397.5,7408.5,3344117601
`
	row, err := parseStooqQuoteCSV(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if row.open != 7445.11 || row.close != 7408.5 {
		t.Fatalf("row=%+v", row)
	}
}
