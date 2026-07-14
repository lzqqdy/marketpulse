package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
)

const (
	rlPrefixIP      = "mp:rl:login:ip:"
	rlPrefixPhone   = "mp:rl:login:phone:"
	rlPrefixFail    = "mp:rl:login:fail:"
	rlPrefixLock    = "mp:rl:login:lock:"
	dummyBcryptHash = "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/X4.G2oQ.5e5e5e5e5e" // invalid filler; real compare uses valid hash below
)

// Precomputed bcrypt of a fixed dummy so missing-user logins still burn similar CPU.
var timingPadHash string

func init() {
	h, err := hashPassword("marketpulse-timing-pad")
	if err != nil {
		timingPadHash = dummyBcryptHash
		return
	}
	timingPadHash = h
}

// DenyInfo carries rate-limit / lockout metadata for HTTP headers.
type DenyInfo struct {
	RetryAfter time.Duration
	Reason     string // ip | phone | lockout
}

func (d *DenyInfo) Error() string {
	if d == nil {
		return "users: denied"
	}
	switch d.Reason {
	case "lockout":
		return ErrLoginLocked.Error()
	default:
		return ErrRateLimited.Error()
	}
}

func (d *DenyInfo) Unwrap() error {
	if d != nil && d.Reason == "lockout" {
		return ErrLoginLocked
	}
	return ErrRateLimited
}

type loginGuard struct {
	rdb *platformredis.Client
	cfg config.UsersSecurityCfg
}

func newLoginGuard(rdb *platformredis.Client, cfg config.UsersSecurityCfg) *loginGuard {
	return &loginGuard{rdb: rdb, cfg: cfg}
}

func (g *loginGuard) enabled() bool {
	return g != nil && g.rdb != nil
}

func (g *loginGuard) allowAttempt(ctx context.Context, ip, phone string) error {
	if !g.enabled() {
		return nil
	}
	if phone != "" {
		if ttl, err := g.rdb.TTL(ctx, rlPrefixLock+phone).Result(); err == nil && ttl > 0 {
			return &DenyInfo{RetryAfter: ttl, Reason: "lockout"}
		}
	}
  // Soft-fail redis errors: allow login rather than 5xx storm if redis flakes
	if ip != "" {
		n, err := g.incrWindow(ctx, rlPrefixIP+ip, g.cfg.Window)
		if err == nil && int(n) > g.cfg.MaxAttemptsPerIP {
			ttl, _ := g.rdb.TTL(ctx, rlPrefixIP+ip).Result()
			if ttl < time.Second {
				ttl = g.cfg.Window
			}
			return &DenyInfo{RetryAfter: ttl, Reason: "ip"}
		}
	}
	if phone != "" {
		n, err := g.incrWindow(ctx, rlPrefixPhone+phone, g.cfg.Window)
		if err == nil && int(n) > g.cfg.MaxAttemptsPerPhone {
			ttl, _ := g.rdb.TTL(ctx, rlPrefixPhone+phone).Result()
			if ttl < time.Second {
				ttl = g.cfg.Window
			}
			return &DenyInfo{RetryAfter: ttl, Reason: "phone"}
		}
	}
	return nil
}

func (g *loginGuard) incrWindow(ctx context.Context, key string, window time.Duration) (int64, error) {
	n, err := g.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_ = g.rdb.Expire(ctx, key, window).Err()
	}
	return n, nil
}

func (g *loginGuard) recordFailure(ctx context.Context, phone string) {
	if !g.enabled() || phone == "" {
		return
	}
	n, err := g.incrWindow(ctx, rlPrefixFail+phone, g.cfg.LockoutTTL)
	if err != nil {
		return
	}
	if int(n) >= g.cfg.LockoutFailures {
		_ = g.rdb.Set(ctx, rlPrefixLock+phone, "1", g.cfg.LockoutTTL).Err()
		_ = g.rdb.Del(ctx, rlPrefixFail+phone).Err()
	}
}

func (g *loginGuard) clearFailures(ctx context.Context, phone string) {
	if !g.enabled() || phone == "" {
		return
	}
	_ = g.rdb.Del(ctx, rlPrefixFail+phone, rlPrefixLock+phone).Err()
}

func padLoginTiming(password string) {
	_ = checkPassword(timingPadHash, password)
}

func denyRetrySeconds(err error) int {
	var d *DenyInfo
	if errors.As(err, &d) && d != nil && d.RetryAfter > 0 {
		sec := int(d.RetryAfter.Seconds())
		if sec < 1 {
			sec = 1
		}
		return sec
	}
	return 60
}

// DenyRetryAfterSeconds exposes retry hint for HTTP Retry-After.
func DenyRetryAfterSeconds(err error) int {
	return denyRetrySeconds(err)
}

// DenyMessage is a user-facing Chinese message for rate limit / lockout.
func DenyMessage(err error) string {
	return fmtDenyMessage(err)
}

func fmtDenyMessage(err error) string {
	var d *DenyInfo
	if errors.As(err, &d) && d != nil {
		switch d.Reason {
		case "lockout":
			return fmt.Sprintf("登录失败次数过多，请 %d 秒后再试", denyRetrySeconds(err))
		case "phone":
			return fmt.Sprintf("该手机号尝试过于频繁，请 %d 秒后再试", denyRetrySeconds(err))
		default:
			return fmt.Sprintf("请求过于频繁，请 %d 秒后再试", denyRetrySeconds(err))
		}
	}
	if errors.Is(err, ErrLoginLocked) {
		return "登录失败次数过多，请稍后再试"
	}
	return "请求过于频繁，请稍后再试"
}
