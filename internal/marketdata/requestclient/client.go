// Package requestclient binds legacy provider HTTP clients to a caller lifecycle.
package requestclient

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

// WithContext retains provider request deadlines while also honoring the caller.
// The context remains alive until the response body is closed, not just headers.
func WithContext(ctx context.Context, base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	c := *base
	if c.Timeout == 0 {
		c.Timeout = 15 * time.Second
	}
	t := c.Transport
	if t == nil {
		t = http.DefaultTransport
	}
	c.Transport = &transport{parent: ctx, base: t}
	return &c
}

type transport struct {
	parent context.Context
	base   http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(t.parent, cancel)
	if t.parent.Err() != nil {
		cancel()
	}
	cleanup := func() { stop(); cancel() }
	resp, err := t.base.RoundTrip(req.Clone(ctx))
	if err != nil {
		cleanup()
		return nil, err
	}
	resp.Body = &body{ReadCloser: resp.Body, cleanup: cleanup}
	return resp, nil
}

type body struct {
	io.ReadCloser
	cleanup func()
	once    sync.Once
}

func (b *body) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.cleanup)
	return err
}
