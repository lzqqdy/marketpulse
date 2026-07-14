package users

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
)

const sessionKeyPrefix = "mp:session:"

type sessionPayload struct {
	UserID    int64     `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type sessionStore struct {
	rdb *platformredis.Client
	ttl time.Duration
}

func newSessionStore(rdb *platformredis.Client, ttl time.Duration) *sessionStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &sessionStore{rdb: rdb, ttl: ttl}
}

func (s *sessionStore) Create(ctx context.Context, userID int64) (token string, expiresAt time.Time, err error) {
	if s.rdb == nil {
		return "", time.Time{}, fmt.Errorf("session: redis unavailable")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	expiresAt = time.Now().UTC().Add(s.ttl)
	payload, err := json.Marshal(sessionPayload{UserID: userID, ExpiresAt: expiresAt})
	if err != nil {
		return "", time.Time{}, err
	}
	if err := s.rdb.Set(ctx, sessionKeyPrefix+token, payload, s.ttl).Err(); err != nil {
		return "", time.Time{}, fmt.Errorf("session set: %w", err)
	}
	return token, expiresAt, nil
}

func (s *sessionStore) UserID(ctx context.Context, token string) (int64, error) {
	if s.rdb == nil || token == "" {
		return 0, ErrUnauthorized
	}
	raw, err := s.rdb.Get(ctx, sessionKeyPrefix+token).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return 0, ErrUnauthorized
		}
		return 0, fmt.Errorf("session get: %w", err)
	}
	var payload sessionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, ErrUnauthorized
	}
	if payload.UserID <= 0 || time.Now().UTC().After(payload.ExpiresAt) {
		return 0, ErrUnauthorized
	}
	return payload.UserID, nil
}

func (s *sessionStore) Delete(ctx context.Context, token string) error {
	if s.rdb == nil || token == "" {
		return nil
	}
	return s.rdb.Del(ctx, sessionKeyPrefix+token).Err()
}
