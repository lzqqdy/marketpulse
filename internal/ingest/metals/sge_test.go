package metals

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

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

func TestFetchAu9999FallbackDelayedQuote(t *testing.T) {
	oldPrimary := sgeDelayedQuotesURL
	oldFallback := sgeDelayedQuotesFallbackURL
	oldDaily := sgeDailyReportURL
	defer func() {
		sgeDelayedQuotesURL = oldPrimary
		sgeDelayedQuotesFallbackURL = oldFallback
		sgeDailyReportURL = oldDaily
	}()

	client := testClient(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/primary":
			return http.StatusBadGateway, "bad gateway"
		case "/fallback":
			return http.StatusOK, `<tr>
				<td>Au99.99</td>
				<td><span class="colorRed">994.0</span></td>
				<td>1002.0</td>
				<td>984.0</td>
				<td>998.8</td>
			</tr>`
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
			return http.StatusNotFound, ""
		}
	})

	sgeDelayedQuotesURL = "https://sge.test/primary"
	sgeDelayedQuotesFallbackURL = "https://sge.test/fallback"
	sgeDailyReportURL = "https://sge.test/daily"

	q, err := FetchAu9999(client)
	if err != nil {
		t.Fatal(err)
	}
	if q.Price != 994.0 || q.ChangePct >= 0 {
		t.Fatalf("quote = %+v", q)
	}
}

func TestFetchAu9999DailyFallback(t *testing.T) {
	oldPrimary := sgeDelayedQuotesURL
	oldFallback := sgeDelayedQuotesFallbackURL
	oldDaily := sgeDailyReportURL
	defer func() {
		sgeDelayedQuotesURL = oldPrimary
		sgeDelayedQuotesFallbackURL = oldFallback
		sgeDailyReportURL = oldDaily
	}()

	client := testClient(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/primary", "/fallback":
			return http.StatusBadGateway, "bad gateway"
		case "/daily":
			if !strings.Contains(r.URL.RawQuery, "start_date=") {
				t.Fatalf("missing date query: %s", r.URL.RawQuery)
			}
			return http.StatusOK, `<tr>
				<td>2026-05-20</td><td>Au99.99</td><td>999.70</td><td>1000.50</td>
				<td>979.01</td><td>984.98</td><td>-13.02</td><td>-1.30%</td>
			</tr>`
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
			return http.StatusNotFound, ""
		}
	})

	sgeDelayedQuotesURL = "https://sge.test/primary"
	sgeDelayedQuotesFallbackURL = "https://sge.test/fallback"
	sgeDailyReportURL = "https://sge.test/daily"

	q, err := FetchAu9999(client)
	if err != nil {
		t.Fatal(err)
	}
	if q.Price != 984.98 || q.ChangePct != -1.30 {
		t.Fatalf("quote = %+v", q)
	}
}
