//go:build ignore

package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	header := http.Header{}
	header.Set("Origin", "https://finance.baidu.com")
	header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	header.Set("Referer", "https://finance.baidu.com/")

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, resp, err := dialer.Dial("wss://finance-ws.pae.baidu.com", header)
	if err != nil {
		fmt.Println("dial err:", err)
		return
	}
	defer conn.Close()
	fmt.Println("dial ok", resp.Status)

	sub := map[string]any{
		"source":  "pc-web",
		"method":  "subscribe",
		"product": "snapshot",
		"items": []map[string]string{
			{"code": "000001", "name": "上证", "market": "ab", "financeType": "index"},
			{"code": "HSI", "name": "恒生", "market": "hk", "financeType": "index"},
		},
	}
	if err := conn.WriteJSON(sub); err != nil {
		fmt.Println("subscribe err:", err)
		return
	}
	fmt.Println("subscribed")

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("read:", err)
			continue
		}
		fmt.Println("msg:", truncate(string(data), 300))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
