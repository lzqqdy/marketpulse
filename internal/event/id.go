package event

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func newID(prefix string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UTC().UnixMilli(), hex.EncodeToString(b[:]))
}

func newEventID() string  { return newID("evt") }
func newSignalID() string { return newID("sig") }
