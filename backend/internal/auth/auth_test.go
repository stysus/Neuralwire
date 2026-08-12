package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

func TestIssueAndValidate(t *testing.T) {
	m := NewManager("unit-test-secret", time.Hour)

	token, err := m.IssueToken("admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	username, err := m.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if username != "admin" {
		t.Errorf("Validate username = %q, want admin", username)
	}
}

func TestValidateRejectsTampering(t *testing.T) {
	m := NewManager("unit-test-secret", time.Hour)

	valid, err := m.IssueToken("admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	other := NewManager("different-secret", time.Hour)
	wrongSecret, err := other.IssueToken("admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	tests := []struct {
		name  string
		token string
	}{
		{name: "empty", token: ""},
		{name: "garbage", token: "not.a.token"},
		{name: "missing signature", token: valid[:20]},
		{name: "tampered payload", token: valid + "x"},
		{name: "wrong secret", token: wrongSecret},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := m.Validate(tt.token); err == nil {
				t.Errorf("Validate(%q) succeeded, want error", tt.token)
			}
		})
	}
}

func TestValidateRejectsExpiredToken(t *testing.T) {
	m := NewManager("unit-test-secret", time.Hour)

	// Build a correctly signed token whose expiry is already in the past.
	payload := fmt.Sprintf("admin:%d", time.Now().Add(-time.Minute).Unix())
	mac := hmac.New(sha256.New, []byte("unit-test-secret"))
	mac.Write([]byte(payload))
	token := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if _, err := m.Validate(token); err == nil {
		t.Error("Validate(expired token) succeeded, want error")
	}
}

func TestDefaultSecretIsStable(t *testing.T) {
	// Two managers with an empty secret must accept each other's tokens so
	// the dev default survives restarts.
	a := NewManager("", time.Hour)
	b := NewManager("", time.Hour)

	token, err := a.IssueToken("admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := b.Validate(token); err != nil {
		t.Errorf("Validate with default secret: %v", err)
	}
}
