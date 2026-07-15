package alerts

import (
	"context"
	"fmt"
	"time"

	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
)

const cooldownKeyPrefix = "mp:alert:cd:"

// CooldownStore tracks per-rule push cooldown in Redis.
type CooldownStore struct {
	rdb      *platformredis.Client
	timezone *time.Location
}

func NewCooldownStore(rdb *platformredis.Client, tz *time.Location) *CooldownStore {
	if tz == nil {
		tz = time.FixedZone("CST", 8*3600)
	}
	return &CooldownStore{rdb: rdb, timezone: tz}
}

func cooldownKey(ruleID int64) string {
	return fmt.Sprintf("%s%d", cooldownKeyPrefix, ruleID)
}

// IsActive reports whether the rule is still in cooldown.
func (c *CooldownStore) IsActive(ctx context.Context, ruleID int64) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, nil
	}
	n, err := c.rdb.Exists(ctx, cooldownKey(ruleID)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (c *CooldownStore) ttlFor(rule Rule) time.Duration {
	switch rule.Frequency {
	case FrequencyOnce:
		return 24 * time.Hour
	case FrequencyLoop:
		mins := rule.IntervalMinutes
		if mins <= 0 {
			mins = 10
		}
		return time.Duration(mins) * time.Minute
	case FrequencyDaily:
		now := time.Now().In(c.timezone)
		end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, c.timezone).Add(time.Second)
		ttl := end.Sub(now)
		if ttl < time.Minute {
			return time.Minute
		}
		return ttl
	default:
		return time.Hour
	}
}

// TrySet claims cooldown with SETNX so concurrent evaluations cannot double-fire.
// Returns claimed=false when the key already exists.
func (c *CooldownStore) TrySet(ctx context.Context, rule Rule) (claimed bool, err error) {
	if c == nil || c.rdb == nil {
		return true, nil
	}
	ok, err := c.rdb.SetNX(ctx, cooldownKey(rule.ID), "1", c.ttlFor(rule)).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// Set applies cooldown TTL based on frequency policy (overwrites existing key).
func (c *CooldownStore) Set(ctx context.Context, rule Rule) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Set(ctx, cooldownKey(rule.ID), "1", c.ttlFor(rule)).Err()
}

// Clear removes cooldown (used when rule deleted).
func (c *CooldownStore) Clear(ctx context.Context, ruleID int64) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, cooldownKey(ruleID)).Err()
}
