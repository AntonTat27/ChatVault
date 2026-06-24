// Package auth verifies Telegram Login Widget payloads, issues and validates
// dashboard sessions, and provides HTTP middleware that gates API routes on
// authentication and per-chat membership.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maxAuthDateAge bounds how old a Telegram Login Widget payload may be
// before it's rejected, to limit the window for replaying a captured payload.
const maxAuthDateAge = 24 * time.Hour

// VerifyTelegramLoginHash checks a Telegram Login Widget payload's hash per
// https://core.telegram.org/widgets/login#checking-authorization. fields must
// contain every payload field except "hash" (e.g. id, first_name, last_name,
// username, photo_url, auth_date); hash is the value of the "hash" field.
func VerifyTelegramLoginHash(botToken string, fields map[string]string, hash string) bool {
	if hash == "" || botToken == "" {
		return false
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var dataCheckString strings.Builder
	for i, k := range keys {
		if i > 0 {
			dataCheckString.WriteByte('\n')
		}
		dataCheckString.WriteString(k)
		dataCheckString.WriteByte('=')
		dataCheckString.WriteString(fields[k])
	}

	secretKey := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secretKey[:])
	mac.Write([]byte(dataCheckString.String()))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(strings.ToLower(hash)))
}

// IsAuthDateFresh reports whether a Telegram Login Widget auth_date (Unix
// seconds) is recent enough to accept, rejecting stale/replayed payloads.
func IsAuthDateFresh(authDateUnix string) bool {
	seconds, err := strconv.ParseInt(authDateUnix, 10, 64)
	if err != nil {
		return false
	}
	authTime := time.Unix(seconds, 0)
	now := time.Now()
	return now.Sub(authTime) <= maxAuthDateAge && authTime.Before(now.Add(time.Minute))
}
