package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/api"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
)

func TestMountStatic_SPAAndAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>ok</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	assets := filepath.Join(dir, "assets")
	if err := os.Mkdir(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		App:     config.AppConfig{Addr: ":0", Mode: "release", StaticDir: dir},
		Symbols: []string{"BTC"},
	}
	r := gin.New()
	h := &api.Handler{Config: cfg, MarketData: marketdata.New(cfg), StartedAt: time.Now().UTC()}
	api.Register(r, h)
	if err := mountStatic(r, dir); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		want int
		body string
	}{
		{"/", 200, "ok"},
		{"/assets/app.js", 200, "console"},
		{"/dashboard", 200, "ok"},
		{"/api/v1/snapshot", 200, ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status %d want %d", w.Code, tc.want)
			}
			if tc.body != "" && !strings.Contains(w.Body.String(), tc.body) {
				t.Fatalf("body %q", w.Body.String())
			}
		})
	}
}
