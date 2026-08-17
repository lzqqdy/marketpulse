package redis

import (
	"fmt"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

// Embedded is an in-process Redis used when no Redis service is available
// (WeChat Cloud Run single replica). Data is lost on restart.
type Embedded struct {
	mini   *miniredis.Miniredis
	Client *Client
}

// OpenEmbedded starts miniredis and returns a go-redis client pointed at it.
func OpenEmbedded() (*Embedded, error) {
	mini, err := miniredis.Run()
	if err != nil {
		return nil, fmt.Errorf("redis embedded: %w", err)
	}
	client := goredis.NewClient(&goredis.Options{Addr: mini.Addr()})
	return &Embedded{mini: mini, Client: client}, nil
}

// Close stops the in-process Redis.
func (e *Embedded) Close() {
	if e == nil {
		return
	}
	if e.Client != nil {
		_ = e.Client.Close()
	}
	if e.mini != nil {
		e.mini.Close()
	}
}
