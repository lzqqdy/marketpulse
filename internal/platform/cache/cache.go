// Package cache provides a small Redis-backed JSON cache for MySQL read paths.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
)

// Store wraps Redis GET/SET/DEL for JSON values.
type Store struct {
	rdb    *platformredis.Client
	prefix string
}

// New returns a cache store. rdb may be nil (all ops become no-ops / misses).
func New(rdb *platformredis.Client, prefix string) *Store {
	if prefix == "" {
		prefix = "mp:cache:"
	}
	return &Store{rdb: rdb, prefix: prefix}
}

func (s *Store) key(k string) string {
	return s.prefix + k
}

// Enabled reports whether Redis backing is available.
func (s *Store) Enabled() bool {
	return s != nil && s.rdb != nil
}

// GetJSON unmarshals a cached value into dest. ok=false on miss or disabled store.
func (s *Store) GetJSON(ctx context.Context, key string, dest any) (bool, error) {
	if !s.Enabled() {
		return false, nil
	}
	raw, err := s.rdb.Get(ctx, s.key(key)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("cache get: %w", err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, fmt.Errorf("cache unmarshal: %w", err)
	}
	return true, nil
}

// SetJSON stores v under key with TTL.
func (s *Store) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	if !s.Enabled() {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}
	if err := s.rdb.Set(ctx, s.key(key), raw, ttl).Err(); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

// Delete removes a cache key.
func (s *Store) Delete(ctx context.Context, key string) error {
	if !s.Enabled() {
		return nil
	}
	if err := s.rdb.Del(ctx, s.key(key)).Err(); err != nil {
		return fmt.Errorf("cache del: %w", err)
	}
	return nil
}
