package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// HMACAuthenticator validates self-contained player tokens.
// Token format: v1:<world>:<player>:<expUnix>:<base64url(hmac)>.
type HMACAuthenticator struct {
	Secret []byte
	Now    func() time.Time
}

func (a HMACAuthenticator) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// Authenticate verifies signature and expiry.
func (a HMACAuthenticator) Authenticate(token string) (Session, error) {
	parts := strings.Split(token, ":")
	if len(parts) != 5 || parts[0] != "v1" {
		return Session{}, ErrUnauthorized
	}
	world, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return Session{}, ErrUnauthorized
	}
	player, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return Session{}, ErrUnauthorized
	}
	exp, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || exp <= a.now().Unix() {
		return Session{}, ErrUnauthorized
	}
	want := signToken(a.Secret, parts[0], parts[1], parts[2], parts[3])
	got, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return Session{}, ErrUnauthorized
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return Session{}, ErrUnauthorized
	}
	return Session{PlayerID: player, WorldID: uint32(world), Region: "global"}, nil
}

func signToken(secret []byte, parts ...string) []byte {
	mac := hmac.New(sha256.New, secret)
	for i, p := range parts {
		if i > 0 {
			mac.Write([]byte(":"))
		}
		mac.Write([]byte(p))
	}
	return mac.Sum(nil)
}

// IssueToken creates a signed player token.
func IssueToken(secret []byte, world uint32, player uint64, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	payload := []string{"v1", strconv.FormatUint(uint64(world), 10), strconv.FormatUint(player, 10), strconv.FormatInt(exp, 10)}
	sig := signToken(secret, payload...)
	return strings.Join(payload, ":") + ":" + base64.RawURLEncoding.EncodeToString(sig)
}

// ParseExpiry is exposed for tests/tools.
func ParseExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ":")
	if len(parts) != 5 {
		return time.Time{}, errors.New("bad token")
	}
	exp, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("bad expiry: %w", err)
	}
	return time.Unix(exp, 0), nil
}
