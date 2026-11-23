package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashAndVerify(t *testing.T) {
	h := NewHasher(1000) // low cost: tests only
	enc, err := h.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(enc, "correct horse") {
		t.Fatal("hash must not contain the plaintext")
	}
	if !h.Verify("correct horse battery", enc) {
		t.Error("right password rejected")
	}
	if h.Verify("wrong", enc) {
		t.Error("wrong password accepted")
	}

	other, _ := h.Hash("correct horse battery")
	if enc == other {
		t.Error("two hashes of the same password must differ (random salt)")
	}
}

func TestPasswordVerifyRejectsMalformedHashes(t *testing.T) {
	h := NewHasher(1000)
	for _, bad := range []string{"", "plaintext", "pbkdf2-sha256$x$y$z", "bcrypt$1$aa$bb", "pbkdf2-sha256$0$aa$bb"} {
		if h.Verify("anything", bad) {
			t.Errorf("malformed hash %q verified", bad)
		}
	}
}

func newManager(now *time.Time) *TokenManager {
	return NewTokenManager("test-secret-test-secret-test-secret", time.Hour, func() time.Time { return *now })
}

func TestTokenRoundTrip(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	m := newManager(&now)

	tok, exp, err := m.Issue(42, "agent")
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(now.Add(time.Hour)) {
		t.Errorf("expiry = %v", exp)
	}
	c, err := m.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != 42 || c.Role != "agent" {
		t.Errorf("claims = %+v", c)
	}
}

func TestTokenRejectsTamperingAndExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	m := newManager(&now)
	tok, _, _ := m.Issue(1, "requester")
	parts := strings.Split(tok, ".")

	// Swap the payload for one that claims a different user, keep the old signature.
	forged := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"2","role":"admin","iat":1800000000,"exp":1800003600}`))
	if _, err := m.Parse(parts[0] + "." + forged + "." + parts[2]); err != ErrInvalidToken {
		t.Errorf("forged payload: err = %v, want ErrInvalidToken", err)
	}

	// A token signed with another secret.
	other := NewTokenManager("a-different-secret-a-different-secret", time.Hour, func() time.Time { return now })
	otherTok, _, _ := other.Issue(1, "requester")
	if _, err := m.Parse(otherTok); err != ErrInvalidToken {
		t.Errorf("wrong secret: err = %v, want ErrInvalidToken", err)
	}

	// "alg: none" must never be accepted.
	none := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + parts[1] + "."
	if _, err := m.Parse(none); err != ErrInvalidToken {
		t.Errorf("alg none: err = %v, want ErrInvalidToken", err)
	}

	// Garbage.
	for _, g := range []string{"", "a.b", "a.b.c", "not a token"} {
		if _, err := m.Parse(g); err != ErrInvalidToken {
			t.Errorf("garbage %q: err = %v", g, err)
		}
	}

	// Expiry.
	now = now.Add(time.Hour + time.Second)
	if _, err := m.Parse(tok); err != ErrExpiredToken {
		t.Errorf("expired: err = %v, want ErrExpiredToken", err)
	}
}
