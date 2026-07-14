package users

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := hashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if !checkPassword(hash, "secret123") {
		t.Fatal("expected password match")
	}
	if checkPassword(hash, "wrong") {
		t.Fatal("expected password mismatch")
	}
}

func TestNormalizePhone(t *testing.T) {
	if got := normalizePhone(" 13800138000 "); got != "13800138000" {
		t.Fatalf("got %q", got)
	}
}
