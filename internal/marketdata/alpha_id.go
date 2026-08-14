package marketdata

import "strings"

// AlphaIDAliases returns lookup keys for an alpha id, covering the historical
// Binance Alpha suffix (qqqon) and the current Bitget-style id (qqq).
func AlphaIDAliases(id string) []string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return nil
	}
	out := []string{id}
	if base, ok := strings.CutSuffix(id, "on"); ok && base != "" {
		out = append(out, base)
	} else {
		out = append(out, id+"on")
	}
	return out
}

// FindAlphaQuote matches by id aliases or trading symbol (case-insensitive).
func FindAlphaQuote(indices, stocks []AlphaQuote, idOrSymbol string) (AlphaQuote, bool) {
	key := strings.ToLower(strings.TrimSpace(idOrSymbol))
	if key == "" {
		return AlphaQuote{}, false
	}
	aliases := make(map[string]struct{}, 4)
	for _, a := range AlphaIDAliases(key) {
		aliases[a] = struct{}{}
	}
	match := func(rows []AlphaQuote) (AlphaQuote, bool) {
		for _, row := range rows {
			rid := strings.ToLower(strings.TrimSpace(row.ID))
			if _, ok := aliases[rid]; ok {
				return row, true
			}
			sym := strings.ToLower(strings.TrimSpace(row.Symbol))
			if sym == "" {
				continue
			}
			if sym == key {
				return row, true
			}
			if _, ok := aliases[sym]; ok {
				return row, true
			}
		}
		return AlphaQuote{}, false
	}
	if row, ok := match(indices); ok {
		return row, true
	}
	return match(stocks)
}
