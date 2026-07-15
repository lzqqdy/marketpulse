package alerts

import "errors"

var (
	ErrDisabled              = errors.New("alerts: module disabled")
	ErrConditionAlreadyMet   = errors.New("alerts: condition already met")
	ErrInvalidParams         = errors.New("alerts: invalid params")
	ErrSymbolUnavailable     = errors.New("alerts: symbol unavailable")
	ErrNotFound              = errors.New("alerts: not found")
	ErrUnauthorized          = errors.New("alerts: unauthorized")
)
