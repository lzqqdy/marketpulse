package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/event"
)

func (h *Handler) eventDisabled(c *gin.Context) bool {
	if h.Events == nil || !h.Events.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "event_disabled", "message": "市场异动引擎未启用"},
		})
		return true
	}
	return false
}

// ListEvents handles GET /api/v1/events.
func (h *Handler) ListEvents(c *gin.Context) {
	if h.eventDisabled(c) {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := 0
	if cur := strings.TrimSpace(c.Query("cursor")); cur != "" {
		if n, err := strconv.Atoi(cur); err == nil && n >= 0 {
			offset = n
		}
	}
	f := event.ListFilter{
		Market:   c.Query("market"),
		Type:     c.Query("type"),
		SubType:  c.Query("subType"),
		Severity: c.Query("severity"),
		Status:   c.Query("status"),
		Symbol:   c.Query("symbol"),
		Limit:    limit,
		Offset:   offset,
	}
	if v := c.Query("startTime"); v != "" {
		if t, ok := parseEventTime(v); ok {
			f.Start = &t
		}
	}
	if v := c.Query("endTime"); v != "" {
		if t, ok := parseEventTime(v); ok {
			f.End = &t
		}
	}
	res, err := h.Events.List(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, eventListJSON(res))
}

// GetEvent handles GET /api/v1/events/:eventID.
func (h *Handler) GetEvent(c *gin.Context) {
	if h.eventDisabled(c) {
		return
	}
	ev, err := h.Events.Get(c.Request.Context(), c.Param("eventID"))
	if err != nil {
		if errors.Is(err, event.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "event_not_found", "message": "事件不存在"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, eventDetailJSON(ev))
}

// GetEventTimeline handles GET /api/v1/events/:eventID/timeline.
func (h *Handler) GetEventTimeline(c *gin.Context) {
	if h.eventDisabled(c) {
		return
	}
	id := c.Param("eventID")
	items, err := h.Events.Timeline(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, event.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "event_not_found", "message": "事件不存在"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"eventId": id, "items": items})
}

// EventsStreamWS handles GET /ws/v1/events (public).
func (h *Handler) EventsStreamWS(c *gin.Context) {
	if h.Events == nil || !h.Events.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "event_disabled", "message": "市场异动引擎未启用"},
		})
		return
	}
	h.Events.ServeWS(c.Writer, c.Request)
}

func parseEventTime(v string) (time.Time, bool) {
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		// seconds or millis
		if n > 1_000_000_000_000 {
			return time.UnixMilli(n).UTC(), true
		}
		return time.Unix(n, 0).UTC(), true
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

func eventListJSON(res event.ListResult) gin.H {
	items := make([]gin.H, 0, len(res.Items))
	for i := range res.Items {
		items = append(items, eventSummaryJSON(&res.Items[i]))
	}
	out := gin.H{"items": items, "limit": res.Limit}
	if res.NextCursor != "" {
		out["nextCursor"] = res.NextCursor
	}
	return out
}

func eventSummaryJSON(ev *event.MarketEvent) gin.H {
	out := gin.H{
		"id":          ev.ID,
		"type":        ev.Type,
		"subType":     ev.SubType,
		"title":       ev.Title,
		"description": ev.Description,
		"severity":    ev.Severity,
		"score":       ev.Score,
		"peakScore":   ev.PeakScore,
		"status":      ev.Status,
		"symbols":     ev.Symbols,
		"markets":     ev.Markets,
		"startTime":   ev.StartTime.Unix(),
		"createdAt":   ev.CreatedAt.Unix(),
		"updatedAt":   ev.UpdatedAt.Unix(),
	}
	if ev.EndTime != nil {
		out["endTime"] = ev.EndTime.Unix()
	} else {
		out["endTime"] = nil
	}
	return out
}

func eventDetailJSON(ev *event.MarketEvent) gin.H {
	out := eventSummaryJSON(ev)
	sigs := make([]gin.H, 0, len(ev.Signals))
	for _, s := range ev.Signals {
		sigs = append(sigs, gin.H{
			"id":         s.ID,
			"signalType": s.SignalType,
			"symbol":     s.Symbol,
			"market":     s.Market,
			"value":      s.Value,
			"baseline":   s.Baseline,
			"ratio":      s.Ratio,
			"changePct":  s.ChangePct,
			"window":     s.Window,
			"direction":  s.Direction,
			"timestamp":  s.Timestamp.Unix(),
			"metadata":   s.Metadata,
		})
	}
	out["signals"] = sigs
	if ev.Context == nil {
		out["context"] = gin.H{}
	} else {
		out["context"] = ev.Context
	}
	return out
}
