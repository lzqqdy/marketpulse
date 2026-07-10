//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/baidu"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/equity"
	"github.com/gorilla/websocket"
)

func main() {
	refs := equity.BaiduRefs(equity.DefaultIndices)
	items := make([]map[string]string, 0, len(refs))
	byCode := make(map[string]baidu.IndexRef)
	for _, ref := range refs {
		item, ok := refWS(ref)
		if !ok {
			continue
		}
		items = append(items, item)
		byCode[item["market"]+":"+item["code"]] = ref
	}
	fmt.Println("items", len(items))

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.Dial("wss://finance-ws.pae.baidu.com", http.Header{})
	if err != nil {
		fmt.Println("dial", err)
		return
	}
	defer conn.Close()
	sub := map[string]any{"source": "pc-web", "method": "subscribe", "product": "snapshot", "items": items}
	_ = conn.WriteJSON(sub)

	got := 0
	parsed := 0
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("read err:", err)
			break
		}
		got++
		rows := parseWS(data, byCode)
		parsed += rows
		if got <= 2 {
			fmt.Println("msg", got, "parsed", rows)
		}
	}
	fmt.Println("total msgs", got, "parsed quotes", parsed)
}

func refWS(ref baidu.IndexRef) (map[string]string, bool) {
	if ref.Code == "" || ref.Market == "" {
		return nil, false
	}
	return map[string]string{
		"code": ref.Code, "name": ref.Name, "market": ref.Market, "financeType": "index",
	}, true
}

func parseWS(data []byte, byCode map[string]baidu.IndexRef) int {
	// mirror current production parser behavior
	var generic map[string]json.RawMessage
	if json.Unmarshal(data, &generic) != nil {
		return 0
	}
	n := 0
	if code := field(generic, "code"); code != "" {
		n++
	}
	if raw, ok := generic["data"]; ok {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) == nil && field(obj, "code") != "" {
			market := strings.ToLower(field(obj, "market"))
			code := strings.ToUpper(field(obj, "code"))
			if _, ok := byCode[market+":"+code]; ok {
				n++
			}
		}
	}
	return n
}

func field(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}
