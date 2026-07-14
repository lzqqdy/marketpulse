package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lzqqdy/marketpulse/internal/platform/cache"
)

const userCacheTTL = 5 * time.Minute

type repository struct {
	db    *sql.DB
	cache *cache.Store
}

func newRepository(db *sql.DB, c *cache.Store) *repository {
	return &repository{db: db, cache: c}
}

func (r *repository) cacheKey(id int64) string {
	return fmt.Sprintf("user:%d", id)
}

func (r *repository) GetByID(ctx context.Context, id int64) (userRow, error) {
	var cached User
	if ok, err := r.cache.GetJSON(ctx, r.cacheKey(id), &cached); err == nil && ok {
		return userRow{
			ID:              cached.ID,
			Phone:           cached.Phone,
			DisplayName:     cached.DisplayName,
			AvatarURL:       cached.AvatarURL,
			Email:           cached.Email,
			WechatPushToken: cached.WechatPushToken,
			CreatedAt:       cached.CreatedAt,
			UpdatedAt:       cached.UpdatedAt,
		}, nil
	}

	row, err := r.scanOne(ctx, `
SELECT id, phone, password_hash, display_name, avatar_url, email, wechat_push_token, created_at, updated_at
FROM users WHERE id = ?`, id)
	if err != nil {
		return userRow{}, err
	}
	_ = r.cache.SetJSON(ctx, r.cacheKey(id), row.toPublic(), userCacheTTL)
	return row, nil
}

func (r *repository) GetByPhone(ctx context.Context, phone string) (userRow, error) {
	return r.scanOne(ctx, `
SELECT id, phone, password_hash, display_name, avatar_url, email, wechat_push_token, created_at, updated_at
FROM users WHERE phone = ?`, phone)
}

func (r *repository) Create(ctx context.Context, phone, passwordHash, displayName string) (userRow, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
INSERT INTO users (phone, password_hash, display_name, avatar_url, email, wechat_push_token, created_at, updated_at)
VALUES (?, ?, ?, '', '', '', ?, ?)`, phone, passwordHash, displayName, now, now)
	if err != nil {
		return userRow{}, fmt.Errorf("users insert: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return userRow{}, err
	}
	return r.GetByID(ctx, id)
}

func (r *repository) UpdateProfile(ctx context.Context, id int64, in UpdateProfileInput) (userRow, error) {
	cur, err := r.loadFull(ctx, id)
	if err != nil {
		return userRow{}, err
	}
	if in.DisplayName != nil {
		cur.DisplayName = *in.DisplayName
	}
	if in.AvatarURL != nil {
		cur.AvatarURL = *in.AvatarURL
	}
	if in.Email != nil {
		cur.Email = *in.Email
	}
	if in.WechatPushToken != nil {
		cur.WechatPushToken = *in.WechatPushToken
	}
	cur.UpdatedAt = time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
UPDATE users SET display_name=?, avatar_url=?, email=?, wechat_push_token=?, updated_at=?
WHERE id=?`, cur.DisplayName, cur.AvatarURL, cur.Email, cur.WechatPushToken, cur.UpdatedAt, id)
	if err != nil {
		return userRow{}, fmt.Errorf("users update profile: %w", err)
	}
	_ = r.cache.Delete(ctx, r.cacheKey(id))
	return r.GetByID(ctx, id)
}

func (r *repository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE users SET password_hash=?, updated_at=? WHERE id=?`,
		passwordHash, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("users update password: %w", err)
	}
	_ = r.cache.Delete(ctx, r.cacheKey(id))
	return nil
}

func (r *repository) loadFull(ctx context.Context, id int64) (userRow, error) {
	// Bypass cache so password_hash is available for change-password flows.
	return r.scanOne(ctx, `
SELECT id, phone, password_hash, display_name, avatar_url, email, wechat_push_token, created_at, updated_at
FROM users WHERE id = ?`, id)
}

func (r *repository) scanOne(ctx context.Context, query string, args ...any) (userRow, error) {
	var row userRow
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&row.ID, &row.Phone, &row.PasswordHash, &row.DisplayName, &row.AvatarURL,
		&row.Email, &row.WechatPushToken, &row.CreatedAt, &row.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return userRow{}, ErrNotFound
	}
	if err != nil {
		return userRow{}, fmt.Errorf("users query: %w", err)
	}
	return row, nil
}
