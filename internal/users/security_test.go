package users

import (
	"errors"
	"testing"
	"time"
)

func TestDenyInfoUnwrap(t *testing.T) {
	err := &DenyInfo{RetryAfter: 30 * time.Second, Reason: "lockout"}
	if !errors.Is(err, ErrLoginLocked) {
		t.Fatal("expected lockout")
	}
	err2 := &DenyInfo{RetryAfter: 10 * time.Second, Reason: "ip"}
	if !errors.Is(err2, ErrRateLimited) {
		t.Fatal("expected rate limited")
	}
	if DenyRetryAfterSeconds(err) != 30 {
		t.Fatalf("retry: %d", DenyRetryAfterSeconds(err))
	}
	msg := DenyMessage(err)
	if msg == "" {
		t.Fatal("expected message")
	}
}
