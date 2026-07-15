package channels

import "context"

// InboxItem is the payload written to Redis inbox and WS push.
type InboxItem struct {
	DeliveryID int64
	RuleID     int64
	Title      string
	Body       string
	Symbol     string
	CreatedAt  int64
}

// InboxWriter persists unread in-app alerts.
type InboxWriter interface {
	Push(ctx context.Context, userID int64, item InboxItem) error
}

// HubWriter pushes live WebSocket payloads.
type HubWriter interface {
	Push(userID int64, payload []byte)
}

// InApp delivers via Redis inbox and optional live WebSocket hub.
type InApp struct {
	Inbox InboxWriter
	Hub   HubWriter
}

func (d *InApp) Deliver(ctx context.Context, userID int64, item InboxItem, wsPayload []byte) error {
	if d.Inbox != nil {
		if err := d.Inbox.Push(ctx, userID, item); err != nil {
			return err
		}
	}
	if d.Hub != nil && len(wsPayload) > 0 {
		d.Hub.Push(userID, wsPayload)
	}
	return nil
}
