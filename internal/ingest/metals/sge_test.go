package metals

import "testing"

func TestParseAu9999(t *testing.T) {
	html := `<tr><td>Au99.99</td><td>552.12</td><td>553.10</td><td>548.60</td><td>550.00</td></tr>`
	latest, open, err := parseAu9999([]byte(html))
	if err != nil {
		t.Fatal(err)
	}
	if latest != 552.12 {
		t.Fatalf("latest = %v", latest)
	}
	if open != 550 {
		t.Fatalf("open = %v", open)
	}
}

func TestParseAu9999Daily(t *testing.T) {
	html := `<tr>
		<td>2026-05-15</td><td>Au99.99</td><td>1031.50</td><td>1031.50</td>
		<td>1000.05</td><td>1006.01</td><td>-22.67</td><td>-2.20%</td>
	</tr>`
	price, changePct, err := parseAu9999Daily([]byte(html))
	if err != nil {
		t.Fatal(err)
	}
	if price != 1006.01 {
		t.Fatalf("price = %v", price)
	}
	if changePct != -2.20 {
		t.Fatalf("changePct = %v", changePct)
	}
}
