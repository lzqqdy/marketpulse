package equity

import (
	"fmt"
	"net/http"

	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

const (
	eastmoneyIndustryFS = "m:90+t:2"
	eastmoneyConceptFS  = "m:90+t:3"
	sectorFields        = "f12,f14,f2,f3,f8,f104,f105,f128,f140,f141,f136"
)

// FetchIndustrySectors loads ranked Eastmoney industry boards.
func FetchIndustrySectors(client *http.Client) ([]store.SectorQuote, error) {
	rows, err := fetchClistAll(client, eastmoneyIndustryFS, sectorFields, 500)
	if err != nil {
		return nil, err
	}
	return parseSectorRows(rows), nil
}

// FetchConceptSectors loads ranked Eastmoney concept boards.
func FetchConceptSectors(client *http.Client) ([]store.SectorQuote, error) {
	rows, err := fetchClistAll(client, eastmoneyConceptFS, sectorFields, 500)
	if err != nil {
		return nil, err
	}
	return parseSectorRows(rows), nil
}

func parseSectorRows(rows []clistRow) []store.SectorQuote {
	out := make([]store.SectorQuote, 0, len(rows))
	for _, row := range rows {
		if row.F12 == "" || row.F14 == "" {
			continue
		}
		out = append(out, store.SectorQuote{
			Code:            row.F12,
			Name:            row.F14,
			ChangePct:       roundPct(row.F3),
			TurnoverRate:    row.F8,
			UpCount:         int(row.F104),
			DownCount:       int(row.F105),
			LeaderName:      row.F128,
			LeaderChangePct: roundPct(row.F136),
		})
	}
	return out
}

func topNSectors(rows []store.SectorQuote, n int, strongest bool) []store.SectorQuote {
	if len(rows) == 0 || n <= 0 {
		return nil
	}
	cp := append([]store.SectorQuote(nil), rows...)
	if strongest {
		sortSectors(cp, true)
	} else {
		sortSectors(cp, false)
	}
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}

func sortSectors(rows []store.SectorQuote, desc bool) {
	for i := 1; i < len(rows); i++ {
		v := rows[i]
		j := i - 1
		for j >= 0 {
			if desc {
				if rows[j].ChangePct >= v.ChangePct {
					break
				}
			} else if rows[j].ChangePct <= v.ChangePct {
				break
			}
			rows[j+1] = rows[j]
			j--
		}
		rows[j+1] = v
	}
}

func avgSectorChange(rows []store.SectorQuote, n int) float64 {
	if len(rows) == 0 || n <= 0 {
		return 0
	}
	if n > len(rows) {
		n = len(rows)
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += rows[i].ChangePct
	}
	return sum / float64(n)
}

func sectorDispersion(rows []store.SectorQuote, n int) float64 {
	if len(rows) < 2 || n <= 1 {
		return 0
	}
	if n > len(rows) {
		n = len(rows)
	}
	top := rows[0].ChangePct
	bottom := rows[n-1].ChangePct
	if top < bottom {
		top, bottom = bottom, top
	}
	return top - bottom
}

func validateSectorFetch(rows []store.SectorQuote, kind string) error {
	if len(rows) == 0 {
		return fmt.Errorf("eastmoney %s sectors: empty result", kind)
	}
	return nil
}
