package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrInvalidToken is returned for any malformed, tampered or wrongly-signed token.
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken is returned when the token's exp claim is in the past.
	ErrExpiredToken = errors.New("token expired")
)

// Claims are the values carried in an access token.
type Claims struct {
	UserID    int64
	Role      string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type payload struct {
	Sub  string `json:"sub"`
	Role string `json:"role"`
	Iat  int64  `json:"iat"`
	Exp  int64  `json:"exp"`
}

// TokenManager issues and verifies HS256 JSON Web Tokens (RFC 7519).
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewTokenManager creates a manager. The clock is injectable for tests.
func NewTokenManager(secret string, ttl time.Duration, now func() time.Time) *TokenManager {
	if now == nil {
		now = time.Now
	}
	return &TokenManager{secret: []byte(secret), ttl: ttl, now: now}
}

var b64 = base64.RawURLEncoding

// Issue signs a new token for the user and returns it with its expiry.
func (m *TokenManager) Issue(userID int64, role string) (string, time.Time, error) {
	now := m.now()
	exp := now.Add(m.ttl)

	h, err := json.Marshal(header{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", time.Time{}, err
	}
	p, err := json.Marshal(payload{Sub: strconv.FormatInt(userID, 10), Role: role, Iat: now.Unix(), Exp: exp.Unix()})
	if err != nil {
		return "", time.Time{}, err
	}
	signingInput := b64.EncodeToString(h) + "." + b64.EncodeToString(p)
	return signingInput + "." + b64.EncodeToString(m.sign(signingInput)), exp, nil
}

// Parse verifies the signature and expiry and returns the claims.
// Only HS256 is accepted, so "alg: none" and algorithm-confusion attacks fail.
func (m *TokenManager) Parse(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	var h header
	if err := decodeSegment(parts[0], &h); err != nil || h.Alg != "HS256" {
		return nil, ErrInvalidToken
	}

	sig, err := b64.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, m.sign(parts[0]+"."+parts[1])) {
		return nil, ErrInvalidToken
	}

	var p payload
	if err := decodeSegment(parts[1], &p); err != nil {
		return nil, ErrInvalidToken
	}
	id, err := strconv.ParseInt(p.Sub, 10, 64)
	if err != nil || id <= 0 {
		return nil, ErrInvalidToken
	}
	exp := time.Unix(p.Exp, 0)
	if !m.now().Before(exp) {
		return nil, ErrExpiredToken
	}
	return &Claims{UserID: id, Role: p.Role, IssuedAt: time.Unix(p.Iat, 0), ExpiresAt: exp}, nil
}

func (m *TokenManager) sign(input string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(input))
	return mac.Sum(nil)
}

func decodeSegment(seg string, v any) error {
	raw, err := b64.DecodeString(seg)
	if err != nil {
		return fmt.Errorf("decode segment: %w", err)
	}
	return json.Unmarshal(raw, v)
}
