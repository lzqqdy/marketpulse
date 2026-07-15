package alerts

import (
	"context"
	"encoding/json"
	"fmt"

	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
)

const inboxKeyPrefix = "mp:alert:inbox:"

// InboxStore manages unread in-app alerts in Redis.
type InboxStore struct {
	rdb    *platformredis.Client
	maxLen int
}

func NewInboxStore(rdb *platformredis.Client, maxLen int) *InboxStore {
	if maxLen <= 0 {
		maxLen = 100
	}
	return &InboxStore{rdb: rdb, maxLen: maxLen}
}

func inboxKey(userID int64) string {
	return fmt.Sprintf("%s%d", inboxKeyPrefix, userID)
}

// Push adds an unread item (newest first).
func (s *InboxStore) Push(ctx context.Context, userID int64, item InboxItem) error {
	if s == nil || s.rdb == nil {
		return nil
	}
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	key := inboxKey(userID)
	pipe := s.rdb.Pipeline()
	pipe.LPush(ctx, key, raw)
	pipe.LTrim(ctx, key, 0, int64(s.maxLen-1))
	_, err = pipe.Exec(ctx)
	return err
}

// List returns all unread items (newest first).
func (s *InboxStore) List(ctx context.Context, userID int64) ([]InboxItem, error) {
	if s == nil || s.rdb == nil {
		return nil, nil
	}
	raws, err := s.rdb.LRange(ctx, inboxKey(userID), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]InboxItem, 0, len(raws))
	for _, raw := range raws {
		var item InboxItem
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

// Ack removes items matching delivery IDs.
func (s *InboxStore) Ack(ctx context.Context, userID int64, deliveryIDs []int64) error {
	if s == nil || s.rdb == nil || len(deliveryIDs) == 0 {
		return nil
	}
	want := make(map[int64]struct{}, len(deliveryIDs))
	for _, id := range deliveryIDs {
		want[id] = struct{}{}
	}
	items, err := s.List(ctx, userID)
	if err != nil {
		return err
	}
	key := inboxKey(userID)
	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, key)
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if _, drop := want[item.DeliveryID]; drop {
			continue
		}
		raw, _ := json.Marshal(item)
		pipe.LPush(ctx, key, raw)
	}
	_, err = pipe.Exec(ctx)
	return err
}
