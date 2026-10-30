// Package gateway implements the reusable player TCP gateway.
package gateway

import (
	"errors"
	"strings"
)

// Session is one authenticated player connection.
type Session struct {
	PlayerID uint64
	WorldID  uint32
	Region   string
}

// Authenticator validates a client token. P1 replaces the dev implementation
// with JWT/session validation.
type Authenticator interface {
	Authenticate(token string) (Session, error)
}

// DevAuthenticator parses "dev:<world>:<player>" for local tests.
type DevAuthenticator struct{}

func (DevAuthenticator) Authenticate(token string) (Session, error) { return DevToken(token) }

// ErrUnauthorized is returned for malformed or missing tokens.
var ErrUnauthorized = errors.New("unauthorized")

// DevToken parses "dev:<world>:<player>".
func DevToken(token string) (Session, error) {
	parts := strings.Split(token, ":")
	if len(parts) != 3 || parts[0] != "dev" {
		return Session{}, ErrUnauthorized
	}
	var world uint32
	var player uint64
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return Session{}, ErrUnauthorized
		}
		world = world*10 + uint32(c-'0')
	}
	for _, c := range parts[2] {
		if c < '0' || c > '9' {
			return Session{}, ErrUnauthorized
		}
		player = player*10 + uint64(c-'0')
	}
	return Session{PlayerID: player, WorldID: world, Region: "dev"}, nil
}
