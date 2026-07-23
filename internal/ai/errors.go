package ai

import "errors"

var (
	ErrDisabled          = errors.New("ai disabled")
	ErrMisconfigured     = errors.New("ai misconfigured")
	ErrInvalidInput      = errors.New("invalid ai input")
	ErrNotFound          = errors.New("ai resource not found")
	ErrConversationBusy  = errors.New("ai conversation busy")
	ErrUpstream          = errors.New("ai upstream error")
	ErrQuotaExceeded     = errors.New("ai quota exceeded")
)
