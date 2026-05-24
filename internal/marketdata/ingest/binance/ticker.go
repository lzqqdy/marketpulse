package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// TickerUpdate is a normalized miniTicker event.
type TickerUpdate struct {
	Symbol       string
	PriceUsdt    float64
	Open24h      float64
	High24h      float64
	Low24h       float64
	Change24hPct float64
	Volume24h    float64
	EventTime    time.Time
}

type combinedMessage struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

type miniTickerEvent struct {
	EventType   string `json:"e"`
	EventTimeMs int64  `json:"E"` // 必须声明，否则 encoding/json 大小写不敏感会把 "E" 填进 "e"
	Symbol      string `json:"s"`
	Close       string `json:"c"`
	Open        string `json:"o"`
	High        string `json:"h"`
	Low         string `json:"l"`
	Volume      string `json:"v"`
}

// RunMiniTicker connects to Binance combined miniTicker stream until ctx ends.
func RunMiniTicker(ctx context.Context, streamURL string, onTick func(TickerUpdate)) error {
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, streamURL, http.Header{})
	if err != nil {
		return fmt.Errorf("binance miniTicker dial: %w", err)
	}
	defer conn.Close()

	const readWait = 90 * time.Second
	_ = conn.SetReadDeadline(time.Now().Add(readWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(readWait))
	})

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		case <-done:
		}
	}()
	defer close(done)

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-pingTicker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			}
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("binance miniTicker read: %w", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(readWait))
		tick, ok := parseMiniTickerMessage(data)
		if !ok {
			continue
		}
		onTick(tick)
	}
}

func parseMiniTickerMessage(data []byte) (TickerUpdate, bool) {
	var wrap combinedMessage
	if err := json.Unmarshal(data, &wrap); err == nil && len(wrap.Data) > 0 {
		var ev miniTickerEvent
		if json.Unmarshal(wrap.Data, &ev) == nil && ev.Symbol != "" {
			return normalizeMiniTicker(ev)
		}
	}
	var ev miniTickerEvent
	if err := json.Unmarshal(data, &ev); err != nil || ev.Symbol == "" {
		return TickerUpdate{}, false
	}
	return normalizeMiniTicker(ev)
}

func normalizeMiniTicker(ev miniTickerEvent) (TickerUpdate, bool) {
	closep, err := strconv.ParseFloat(ev.Close, 64)
	if err != nil || closep <= 0 {
		return TickerUpdate{}, false
	}
	open, _ := strconv.ParseFloat(ev.Open, 64)
	high, _ := strconv.ParseFloat(ev.High, 64)
	low, _ := strconv.ParseFloat(ev.Low, 64)
	volume, _ := strconv.ParseFloat(ev.Volume, 64)

	chg := 0.0
	if open > 0 {
		chg = (closep - open) / open * 100
	}

	sym := strings.TrimSuffix(strings.ToUpper(ev.Symbol), "USDT")
	if sym == "" {
		return TickerUpdate{}, false
	}

	return TickerUpdate{
		Symbol:       sym,
		PriceUsdt:    closep,
		Open24h:      open,
		High24h:      high,
		Low24h:       low,
		Change24hPct: chg,
		Volume24h:    volume,
		EventTime:    time.Now().UTC(),
	}, true
}
