// 在服务器上快速验证 Binance combined miniTicker 是否可收包。
// 用法: go run ./scripts/binance_ws_check 'wss://stream.binance.com:9443/stream?streams=btcusdt@miniTicker'
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ingest/binance"
)

func main() {
	url := "wss://stream.binance.com:9443/stream?streams=btcusdt@miniTicker"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	got := 0
	err := binance.RunMiniTicker(ctx, url, func(t binance.TickerUpdate) {
		got++
		fmt.Printf("    tick #%d %s %.4f USDT (24h %.2f%%)\n", got, t.Symbol, t.PriceUsdt, t.Change24hPct)
		if got >= 2 {
			cancel()
		}
	})
	if got == 0 {
		if err != nil {
			fmt.Fprintf(os.Stderr, "    error: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "    error: no ticker within 15s\n")
		}
		os.Exit(1)
	}
}
