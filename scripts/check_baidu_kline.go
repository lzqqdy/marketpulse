//go:build ignore

package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/baidu"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/equity"
)

func main() {
	cfg := baidu.Config{Enabled: true}
	client := &http.Client{Timeout: 15 * time.Second}
	for _, id := range []string{"ks11", "n225", "sh000001", "sh000688", "sz399006", "hsi", "dji", "gold"} {
		def, ok := equity.DefaultIndexByID(id)
		if !ok {
			continue
		}
		c, err := baidu.FetchKlines(client, cfg, def.BaiduRef(), "1d", 50)
		last := 0.0
		if len(c) > 0 {
			last = c[len(c)-1].Close
		}
		fmt.Printf("%s baidu kline: count=%d last=%.2f err=%v\n", id, len(c), last, err)
	}
}
