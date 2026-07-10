package baidu

import (
	"fmt"
	"net/url"
)

func quoteParams(ref IndexRef) (url.Values, error) {
	if !ref.valid() {
		return nil, fmt.Errorf("%s: no baidu mapping", ref.ID)
	}
	market := lower(ref.Market)
	financeType := ref.financeType()
	group := quoteGroup(market, financeType)
	q := url.Values{}
	q.Set("code", ref.Code)
	q.Set("market_type", market)
	q.Set("financeType", financeType)
	q.Set("group", group)
	q.Set("pointType", "string")
	q.Set("new_Format", "1")
	q.Set("finClientType", "pc")
	if financeType == "index" {
		q.Set("isIndex", "1")
	}
	if financeType == "futures" {
		q.Set("isFutures", "1")
	}
	return q, nil
}

func klineParams(ref IndexRef, interval string) (url.Values, error) {
	if !ref.valid() {
		return nil, fmt.Errorf("%s: no baidu mapping", ref.ID)
	}
	period, ktype, err := klinePeriod(interval)
	if err != nil {
		return nil, err
	}
	market := lower(ref.Market)
	financeType := ref.financeType()
	q := url.Values{}
	q.Set("code", ref.Code)
	q.Set("market_type", market)
	q.Set("group", klineGroup(market, financeType))
	q.Set("newFormat", "1")
	q.Set("all", "1")
	q.Set("eprop", period)
	if ktype != "" {
		q.Set("ktype", ktype)
	}
	if financeType == "index" {
		q.Set("isIndex", "1")
	}
	if financeType == "futures" {
		q.Set("isFutures", "1")
	}
	return q, nil
}

func quoteGroup(market, financeType string) string {
	switch financeType {
	case "futures":
		return "quotation_futures_minute"
	case "index":
		if market == "ab" || market == "hk" || market == "us" {
			return "quotation_minute_" + market
		}
		return "quotation_index_minute"
	default:
		return "quotation_minute_" + market
	}
}

func klineGroup(market, financeType string) string {
	if financeType == "futures" {
		return "quotation_futures_kline"
	}
	if financeType == "index" {
		return "quotation_index_kline"
	}
	if market == "ab" || market == "hk" || market == "us" {
		return "quotation_kline_" + market
	}
	return "quotation_kline_ab"
}

func klinePeriod(interval string) (period, ktype string, err error) {
	switch lower(trim(interval)) {
	case "1h":
		return "60minK", "", nil
	case "1d", "":
		return "dayK", "1", nil
	case "1w":
		return "weekK", "2", nil
	case "15m":
		return "", "", fmt.Errorf("baidu kline: interval 15m not supported")
	default:
		return "", "", fmt.Errorf("baidu kline: unsupported interval %s", interval)
	}
}

func wsSubscribeItem(ref IndexRef) (wsSubscribe, bool) {
	if !ref.valid() {
		return wsSubscribe{}, false
	}
	return wsSubscribe{
		Code:        ref.Code,
		Name:        ref.Name,
		Market:      lower(ref.Market),
		FinanceType: ref.financeType(),
	}, true
}
