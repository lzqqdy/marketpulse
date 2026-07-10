//go:build ignore

package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.Dial("wss://finance-ws.pae.baidu.com", http.Header{})
	if err != nil {
		fmt.Println("dial err:", err)
		return
	}
	defer conn.Close()
	fmt.Println("dial ok (no headers)")
	sub := map[string]any{
		"source": "pc-web", "method": "subscribe", "product": "snapshot",
		"items": []map[string]string{{"code": "000001", "name": "上证", "market": "ab", "financeType": "index"}},
	}
	_ = conn.WriteJSON(sub)
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		fmt.Println("first read:", err)
		return
	}
	fmt.Println("first msg:", len(data))
}
