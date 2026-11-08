package auth

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("rahasia123")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyPassword("rahasia123", h)
	if err != nil || !ok {
		t.Fatalf("verify failed: %v", err)
	}
	if ok, _ := VerifyPassword("salah", h); ok {
		t.Fatal("wrong password accepted")
	}
	if _, err := VerifyPassword("x", "garbage"); err == nil {
		t.Fatal("garbage hash accepted")
	}
}

func TestToken(t *testing.T) {
	tok, hash, err := NewToken()
	if err != nil || len(tok) < 40 || len(hash) != 32 {
		t.Fatalf("token %q hash %d err %v", tok, len(hash), err)
	}
	if string(HashToken(tok)) != string(hash) {
		t.Fatal("hash mismatch")
	}
	p, _ := TempPassword()
	if len(p) != 10 {
		t.Fatal("temp password length")
	}
}
