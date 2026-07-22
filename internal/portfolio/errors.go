package portfolio

import "errors"

var (
	ErrDisabled          = errors.New("portfolio disabled")
	ErrInvalidInput      = errors.New("invalid portfolio input")
	ErrSymbolUnavailable = errors.New("symbol unavailable")
	ErrNotFound          = errors.New("portfolio resource not found")
)
