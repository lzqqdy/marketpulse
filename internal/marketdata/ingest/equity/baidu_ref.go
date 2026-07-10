package equity

import "github.com/lzqqdy/marketpulse/internal/marketdata/ingest/baidu"

// BaiduRef converts an index definition into a Baidu provider reference.
func (d IndexDef) BaiduRef() baidu.IndexRef {
	return baidu.IndexRef{
		ID:          d.ID,
		Name:        d.Name,
		Code:        d.BaiduCode,
		Market:      d.BaiduMarket,
		FinanceType: d.resolvedBaiduFinanceType(),
		MinPrice:    d.MinPrice,
		MaxPrice:    d.MaxPrice,
	}
}

// BaiduRefs maps configured index definitions to Baidu references.
func BaiduRefs(defs []IndexDef) []baidu.IndexRef {
	out := make([]baidu.IndexRef, 0, len(defs))
	for _, def := range defs {
		if !def.HasBaidu() {
			continue
		}
		out = append(out, def.BaiduRef())
	}
	return out
}
