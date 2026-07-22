package metals

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParseSinaGold(t *testing.T) {
	raw := `var hq_str_gds_AUTD="904.72,0,904.50,904.99,905.39,896.07,21:56:45,898.29,899.50,4338,5.00,1.00,2026-07-22,黄金延期";`
	q, err := parseSinaGold(raw)
	if err != nil {
		t.Fatal(err)
	}
	if q.Price != 904.72 {
		t.Fatalf("price = %v", q.Price)
	}
	if q.Source != "sina" || q.ID != domesticGoldID {
		t.Fatalf("quote = %+v", q)
	}
	// prev≈899.50 → ~0.58%
	if q.ChangePct < 0.5 || q.ChangePct > 0.7 {
		t.Fatalf("changePct = %v", q.ChangePct)
	}
}

func TestFetchAu9999EastmoneyPrimary(t *testing.T) {
	oldEM := eastmoneyQuoteURL
	oldSina := sinaQuoteURL
	defer func() {
		eastmoneyQuoteURL = oldEM
		sinaQuoteURL = oldSina
	}()

	client := testClient(func(r *http.Request) (int, string) {
		if strings.Contains(r.URL.Host, "eastmoney") || strings.Contains(r.URL.Path, "/api/qt/stock/get") {
			return http.StatusOK, `{"rc":0,"data":{"f43":902.01,"f46":900,"f57":"AU9999","f58":"黄金9999","f60":899,"f86":1784728605,"f169":3.01,"f170":0.33}}`
		}
		t.Fatalf("unexpected url %s", r.URL.String())
		return http.StatusNotFound, ""
	})

	eastmoneyQuoteURL = "https://eastmoney.test/api/qt/stock/get"
	sinaQuoteURL = "https://sina.test/fail"

	q, err := FetchAu9999(client)
	if err != nil {
		t.Fatal(err)
	}
	if q.Price != 902.01 || q.ChangePct != 0.33 || q.Source != "eastmoney" {
		t.Fatalf("quote = %+v", q)
	}
}

func TestFetchAu9999SinaFallback(t *testing.T) {
	oldEM := eastmoneyQuoteURL
	oldSina := sinaQuoteURL
	defer func() {
		eastmoneyQuoteURL = oldEM
		sinaQuoteURL = oldSina
	}()

	client := testClient(func(r *http.Request) (int, string) {
		u := r.URL.String()
		if strings.Contains(u, "eastmoney") {
			return http.StatusBadGateway, "bad"
		}
		if strings.Contains(u, "sina") {
			return http.StatusOK, `var hq_str_gds_AUTD="910.00,0,909,911,912,900,10:00:00,905,900.00,1,1,1,2026-07-22,黄金延期";`
		}
		t.Fatalf("unexpected url %s", u)
		return http.StatusNotFound, ""
	})

	eastmoneyQuoteURL = "https://eastmoney.test/api/qt/stock/get"
	sinaQuoteURL = "https://sina.test/?list=gds_AUTD"

	q, err := FetchAu9999(client)
	if err != nil {
		t.Fatal(err)
	}
	if q.Price != 910 || q.Source != "sina" {
		t.Fatalf("quote = %+v", q)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testClient(fn func(*http.Request) (int, string)) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		status, body := fn(req)
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}
}
