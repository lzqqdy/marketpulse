package otc

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchUSDTCNY(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":[{"bestOption":true,"price":"7.25"}]}`))
	}))
	defer srv.Close()

	old := apiURL
	apiURL = srv.URL
	t.Cleanup(func() { apiURL = old })

	p, err := FetchUSDTCNY(srv.Client())
	if err != nil || p != 7.25 {
		t.Fatalf("price=%v err=%v", p, err)
	}
}
