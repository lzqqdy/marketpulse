package channels

import (
	"strings"
	"testing"

	"github.com/lzqqdy/marketpulse/internal/config"
)

func TestResolveFrom_displayNameFallsBackToUsername(t *testing.T) {
	e := NewEmail(config.SMTPConfig{
		Host:     "smtp.163.com",
		Username: "lzqqdy@163.com",
		From:     "萧小咩",
	})
	addr, header, err := e.resolveFrom()
	if err != nil {
		t.Fatal(err)
	}
	if addr != "lzqqdy@163.com" {
		t.Fatalf("addr=%q", addr)
	}
	if !strings.Contains(header, "lzqqdy@163.com") {
		t.Fatalf("header=%q", header)
	}
}

func TestResolveFrom_angleAddress(t *testing.T) {
	e := NewEmail(config.SMTPConfig{
		Host:     "smtp.163.com",
		Username: "lzqqdy@163.com",
		From:     "萧小咩 <lzqqdy@163.com>",
	})
	addr, _, err := e.resolveFrom()
	if err != nil {
		t.Fatal(err)
	}
	if addr != "lzqqdy@163.com" {
		t.Fatalf("addr=%q", addr)
	}
}

func TestEncodeHeader_asciiPassthrough(t *testing.T) {
	if got := encodeHeader("Hello"); got != "Hello" {
		t.Fatalf("got %q", got)
	}
}

func TestEncodeHeader_utf8(t *testing.T) {
	got := encodeHeader("上涨触达")
	if !strings.HasPrefix(got, "=?UTF-8?B?") {
		t.Fatalf("got %q", got)
	}
}
