package portfolio

import "errors"

var (
	ErrDisabled          = errors.New("portfolio disabled")
	ErrInvalidInput      = errors.New("invalid portfolio input")
	ErrSymbolUnavailable = errors.New("symbol unavailable")
	ErrNotFound          = errors.New("portfolio resource not found")
	ErrSnapshotWindow    = errors.New("daily snapshot requires current-date 23:59 sampling; historical backfill unavailable")
)
