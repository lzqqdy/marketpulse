package baidu

// IndexRef is the minimal index mapping needed by Baidu Finance clients.
type IndexRef struct {
	ID          string
	Name        string
	Code        string
	Market      string
	FinanceType string
	MinPrice    float64
	MaxPrice    float64
}

func (r IndexRef) financeType() string {
	if t := trim(r.FinanceType); t != "" {
		return lower(t)
	}
	return "index"
}

func (r IndexRef) valid() bool {
	return trim(r.Code) != "" && trim(r.Market) != ""
}

func (r IndexRef) validatePrice(price float64) error {
	if price <= 0 {
		return errEmptyQuote(r.ID)
	}
	if r.MinPrice > 0 && price < r.MinPrice {
		return errSuspiciousPrice(r.ID, price, r.MinPrice, "below")
	}
	if r.MaxPrice > 0 && price > r.MaxPrice {
		return errSuspiciousPrice(r.ID, price, r.MaxPrice, "above")
	}
	return nil
}
