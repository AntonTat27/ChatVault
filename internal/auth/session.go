package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// SessionCookieName is the cookie holding the raw (unhashed) dashboard
// session token. Only its sha256 hash is ever persisted server-side.
const SessionCookieName = "chatvault_session"

// SessionTTL is how long a dashboard session remains valid after login.
const SessionTTL = 30 * 24 * time.Hour

// sessionTokenBytes is the size of the random session token before hex encoding.
const sessionTokenBytes = 32

// GenerateSessionToken creates a new random session token, returning both the
// raw value (to set as the cookie) and its sha256 hash (to store in the DB).
func GenerateSessionToken() (raw string, hash string, err error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	raw = hex.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// HashToken returns the sha256 hex digest of a raw session token, used as
// the lookup key in dashboard_sessions so a leaked DB never exposes usable
// raw tokens.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
