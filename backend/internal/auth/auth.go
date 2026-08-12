// Package auth issues and validates HMAC-signed bearer tokens for the
// admin API. Tokens embed the username and an expiry timestamp, and the
// signature is verified with a shared secret (ADMIN_TOKEN_SECRET).
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Manager signs and verifies bearer tokens.
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager builds a Manager. A nil or empty secret falls back to the
// development default so the server still boots in local development.
func NewManager(secret string, ttl time.Duration) *Manager {
	if secret == "" {
		secret = devDefaultSecret
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Manager{secret: []byte(secret), ttl: ttl}
}

// TokenTTL returns how long issued tokens remain valid.
func (m *Manager) TokenTTL() time.Duration {
	return m.ttl
}

// IssueToken signs a token for the given username. The returned value is
// safe to embed in an Authorization: Bearer <token> header.
func (m *Manager) IssueToken(username string) (string, error) {
	payload := fmt.Sprintf("%s:%d", username, time.Now().Add(m.ttl).Unix())
	sig := m.sign(payload)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(sig), nil
}

// Validate verifies the token signature and expiry. On success it returns
// the embedded username; otherwise it returns an empty string and an error
// describing the failure.
func (m *Manager) Validate(token string) (string, error) {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok || payload == "" || sig == "" {
		return "", errors.New("malformed token")
	}

	rawPayload, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("malformed token payload: %w", err)
	}
	rawSig, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return "", fmt.Errorf("malformed token signature: %w", err)
	}

	if !hmac.Equal(rawSig, m.sign(string(rawPayload))) {
		return "", errors.New("invalid token signature")
	}

	username, expiryRaw, ok := strings.Cut(string(rawPayload), ":")
	if !ok || username == "" {
		return "", errors.New("malformed token payload")
	}
	expiry, err := strconv.ParseInt(expiryRaw, 10, 64)
	if err != nil {
		return "", errors.New("malformed token expiry")
	}
	if time.Now().Unix() > expiry {
		return "", errors.New("token expired")
	}
	return username, nil
}

func (m *Manager) sign(payload string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// devDefaultSecret is the fallback used when ADMIN_TOKEN_SECRET is unset.
// It is intentionally fixed so tokens survive restarts in development; the
// README and .env.example instruct operators to change it.
const devDefaultSecret = "neuralwire-dev-secret-7f3c9a1e4b8d2f6a"
