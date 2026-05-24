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

type klineWSMessage struct {
	Event string `json:"e"`
	K     struct {
		Start  int64  `json:"t"`
		Open   string `json:"o"`
		High   string `json:"h"`
		Low    string `json:"l"`
		Close  string `json:"c"`
		Volume string `json:"v"`
		Closed bool   `json:"x"`
	} `json:"k"`
}

// StreamKline subscribes to Binance kline websocket until ctx is cancelled.
func StreamKline(ctx context.Context, baseSymbol, interval string, onCandle func(Candle)) error {
	interval, err := NormalizeInterval(interval)
	if err != nil {
		return err
	}
	pair := strings.ToLower(SymbolUSDT(baseSymbol))
	url := fmt.Sprintf("wss://stream.binance.com:9443/ws/%s@kline_%s", pair, interval)

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, url, http.Header{})
	if err != nil {
		return fmt.Errorf("binance kline ws dial: %w", err)
	}
	defer conn.Close()

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		case <-done:
		}
	}()
	defer close(done)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("binance kline ws read: %w", err)
		}
		var msg klineWSMessage
		if err := json.Unmarshal(data, &msg); err != nil || msg.Event != "kline" {
			continue
		}
		c, err := parseKlineK(msg.K.Start, msg.K.Open, msg.K.High, msg.K.Low, msg.K.Close, msg.K.Volume)
		if err != nil {
			continue
		}
		onCandle(c)
	}
}

func parseKlineK(start int64, o, h, l, c, v string) (Candle, error) {
	open, err := strconv.ParseFloat(o, 64)
	if err != nil {
		return Candle{}, err
	}
	high, err := strconv.ParseFloat(h, 64)
	if err != nil {
		return Candle{}, err
	}
	low, err := strconv.ParseFloat(l, 64)
	if err != nil {
		return Candle{}, err
	}
	closep, err := strconv.ParseFloat(c, 64)
	if err != nil {
		return Candle{}, err
	}
	vol, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return Candle{}, err
	}
	return Candle{
		Time:   start / 1000,
		Open:   open,
		High:   high,
		Low:    low,
		Close:  closep,
		Volume: vol,
	}, nil
}
