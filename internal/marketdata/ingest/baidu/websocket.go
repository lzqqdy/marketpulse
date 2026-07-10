package baidu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

// RunIndexSnapshotWS maintains a Baidu Finance snapshot subscription until ctx ends.
func RunIndexSnapshotWS(ctx context.Context, cfg Config, refs []IndexRef, onUpdate func(map[string]store.IndexQuote)) error {
	cfg = normalizeConfig(cfg)
	if !cfg.Enabled || !cfg.WSEnabled {
		return fmt.Errorf("baidu ws: disabled")
	}
	items := make([]wsSubscribe, 0, len(refs))
	byCode := make(map[string]IndexRef, len(refs))
	for _, ref := range refs {
		item, ok := wsSubscribeItem(ref)
		if !ok {
			continue
		}
		items = append(items, item)
		byCode[item.Market+":"+strings.ToUpper(item.Code)] = ref
	}
	if len(items) == 0 {
		return fmt.Errorf("baidu ws: no mapped symbols")
	}

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, cfg.WSURL, baiduWSHeaders())
	if err != nil {
		return fmt.Errorf("baidu ws dial: %w", err)
	}
	defer conn.Close()

	subscribe := wsSubscribeMessage{
		Source:  "pc-web",
		Method:  "subscribe",
		Product: "snapshot",
		Items:   items,
	}
	if err := conn.WriteJSON(subscribe); err != nil {
		return fmt.Errorf("baidu ws subscribe: %w", err)
	}

	const readWait = 90 * time.Second
	_ = conn.SetReadDeadline(time.Now().Add(readWait))
	patchEvery := time.Duration(cfg.WSPatchInterval) * time.Second
	patchTicker := time.NewTicker(patchEvery)
	defer patchTicker.Stop()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		case <-done:
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-patchTicker.C:
			patch := wsSubscribeMessage{
				Source:  "pc-web",
				Method:  "patch",
				Product: "snapshot",
				Items:   items,
			}
			if err := conn.WriteJSON(patch); err != nil {
				return fmt.Errorf("baidu ws patch: %w", err)
			}
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("baidu ws read: %w", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(readWait))
		rows := parseWSUpdates(data, byCode, time.Now().UTC())
		if len(rows) > 0 && onUpdate != nil {
			onUpdate(rows)
		}
	}
}

func parseWSUpdates(data []byte, byCode map[string]IndexRef, now time.Time) map[string]store.IndexQuote {
	out := make(map[string]store.IndexQuote)
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(data, &generic); err != nil {
		return out
	}
	tryAppendWSQuote(out, generic, byCode, now)
	if raw, ok := generic["data"]; ok {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) == nil {
			tryAppendWSQuote(out, obj, byCode, now)
		}
	}
	if raw, ok := generic["Result"]; ok {
		tryAppendWSQuote(out, map[string]json.RawMessage{"Result": raw}, byCode, now)
	}
	if raw, ok := generic["items"]; ok {
		var items []json.RawMessage
		if json.Unmarshal(raw, &items) == nil {
			for _, item := range items {
				var obj map[string]json.RawMessage
				if json.Unmarshal(item, &obj) != nil {
					continue
				}
				tryAppendWSQuote(out, obj, byCode, now)
			}
		}
	}
	return out
}

func tryAppendWSQuote(out map[string]store.IndexQuote, payload map[string]json.RawMessage, byCode map[string]IndexRef, now time.Time) {
	code := extractWSField(payload, "code")
	market := strings.ToLower(extractWSField(payload, "market", "market_type"))
	if code == "" {
		return
	}
	ref, ok := byCode[market+":"+strings.ToUpper(code)]
	if !ok {
		ref, ok = byCode[":"+strings.ToUpper(code)]
	}
	if !ok {
		return
	}
	priceRaw := extractWSField(payload, "price", "lastPx", "last")
	ratioRaw := extractWSField(payload, "ratio", "pxChangeRate", "changePercent")
	if raw, ok := payload["cur"]; ok {
		var cur map[string]string
		if json.Unmarshal(raw, &cur) == nil {
			if priceRaw == "" {
				priceRaw = cur["price"]
			}
			if ratioRaw == "" {
				ratioRaw = cur["ratio"]
			}
		}
	}
	if priceRaw == "" {
		return
	}
	price, err := strconvParse(priceRaw)
	if err != nil {
		return
	}
	if err := ref.validatePrice(price); err != nil {
		return
	}
	changePct := parseBaiduPercent(ratioRaw)
	out[ref.ID] = store.IndexQuote{
		ID:        ref.ID,
		Name:      ref.Name,
		Price:     price,
		ChangePct: changePct,
		Source:    "baidu",
		FetchedAt: now,
		UpdatedAt: now,
	}
}

func extractWSField(payload map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		var n json.Number
		if json.Unmarshal(raw, &n) == nil {
			return n.String()
		}
	}
	return ""
}

func baiduWSHeaders() http.Header {
	header := http.Header{}
	header.Set("Origin", "https://finance.baidu.com")
	header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	header.Set("Referer", "https://finance.baidu.com/")
	return header
}
