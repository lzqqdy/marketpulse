package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/users"
)

var streamUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// StreamServer serves authenticated alert WebSocket connections.
type StreamServer struct {
	Alerts Service
	Users  users.Service
}

func NewStreamServer(alerts Service, userSvc users.Service) *StreamServer {
	return &StreamServer{Alerts: alerts, Users: userSvc}
}

func (s *StreamServer) ServeWS(w http.ResponseWriter, r *http.Request, token string) {
	if s.Alerts == nil || !s.Alerts.Enabled() {
		http.Error(w, "alerts disabled", http.StatusServiceUnavailable)
		return
	}
	token = strings.TrimSpace(token)
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, err := s.Users.UserIDFromToken(r.Context(), token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := streamUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	hub := s.Alerts.Hub()
	if hub != nil {
		hub.Register(userID, conn)
		defer hub.Unregister(userID, conn)
	}
	s.sendInboxSnapshot(r.Context(), conn, userID)
	s.readLoop(r.Context(), conn, userID)
}

func (s *StreamServer) sendInboxSnapshot(ctx context.Context, conn *websocket.Conn, userID int64) {
	items, err := s.Alerts.InboxSnapshot(ctx, userID)
	if err != nil || len(items) == 0 {
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, map[string]any{
			"deliveryId": item.DeliveryID,
			"ruleId":     item.RuleID,
			"title":      item.Title,
			"body":       item.Body,
			"symbol":     item.Symbol,
			"createdAt":  item.CreatedAt,
		})
	}
	payload, _ := json.Marshal(map[string]any{
		"type": "inbox_snapshot",
		"data": map[string]any{"items": data},
	})
	_ = conn.WriteMessage(websocket.TextMessage, payload)
}

func (s *StreamServer) readLoop(ctx context.Context, conn *websocket.Conn, userID int64) {
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg struct {
			Type        string  `json:"type"`
			DeliveryIDs []int64 `json:"deliveryIds"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type == "ack" && len(msg.DeliveryIDs) > 0 {
			_ = s.Alerts.AckInbox(ctx, userID, msg.DeliveryIDs)
		}
		if msg.Type == "ping" {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong"}`))
		}
	}
}
