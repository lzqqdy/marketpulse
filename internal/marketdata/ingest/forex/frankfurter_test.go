package forex

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchUSDCNY(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"rates":{"CNY":7.24}}`))
	}))
	defer srv.Close()

	old := apiURL
	apiURL = srv.URL
	t.Cleanup(func() { apiURL = old })

	p, err := FetchUSDCNY(srv.Client())
	if err != nil || p != 7.24 {
		t.Fatalf("price=%v err=%v", p, err)
	}
}
