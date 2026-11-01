package gateway

import (
	"testing"
	"time"
)

func TestHMACToken(t *testing.T) {
	secret := []byte("test-secret")
	tok := IssueToken(secret, 2, 42, time.Hour)
	s, err := (HMACAuthenticator{Secret: secret}).Authenticate(tok)
	if err != nil || s.WorldID != 2 || s.PlayerID != 42 {
		t.Fatalf("session=%+v err=%v", s, err)
	}
	if _, err := (HMACAuthenticator{Secret: secret}).Authenticate(tok + "x"); err == nil {
		t.Fatal("tampered token accepted")
	}
	if _, err := (HMACAuthenticator{Secret: []byte("other")}).Authenticate(tok); err == nil {
		t.Fatal("wrong secret accepted")
	}
}

func TestHMACTokenExpired(t *testing.T) {
	secret := []byte("s")
	tok := IssueToken(secret, 1, 1, time.Minute)
	a := HMACAuthenticator{Secret: secret, Now: func() time.Time { return time.Now().Add(2 * time.Minute) }}
	if _, err := a.Authenticate(tok); err == nil {
		t.Fatal("expired token accepted")
	}
}
