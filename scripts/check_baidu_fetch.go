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
	defs := equity.DefaultIndices
	refs := equity.BaiduRefs(defs)
	cfg := baidu.Config{Enabled: true}
	client := &http.Client{Timeout: 15 * time.Second}
	rows, err := baidu.FetchQuotes(client, cfg, refs)
	if err != nil {
		fmt.Println("ERR:", err)
		return
	}
	fmt.Println("count:", len(rows))
	for _, id := range []string{"sh000001", "sz399001", "sh000300", "hsi", "dji", "gold"} {
		if r, ok := rows[id]; ok {
			fmt.Printf("%s: price=%.2f pct=%.2f source=%s\n", id, r.Price, r.ChangePct, r.Source)
		} else {
			fmt.Println(id, ": missing")
		}
	}
}
