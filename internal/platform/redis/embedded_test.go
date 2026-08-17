package redis

import (
	"context"
	"testing"
)

func TestOpenEmbedded(t *testing.T) {
	e, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	ctx := context.Background()
	if err := e.Client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	if err := e.Client.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatal(err)
	}
	got, err := e.Client.Get(ctx, "k").Result()
	if err != nil || got != "v" {
		t.Fatalf("get: %q %v", got, err)
	}
}
