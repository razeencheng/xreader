package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const defaultStateTTL = 10 * time.Minute

// CookieState provides stateless HMAC-based OAuth CSRF state tokens.
// The state token format is: {nonce_hex}.{timestamp_ms}.{hmac_sha256_signature}
//
// The caller (auth.Handler) sets the generated state as an HttpOnly cookie
// (xreader_oauth_state) in BeginLogin and verifies it in HandleCallback by
// comparing the ?state query param with the cookie value.
type CookieState struct {
	secret []byte
	ttl    time.Duration
}

// NewCookieState creates a CookieState with the default 10-minute TTL.
func NewCookieState(secret []byte) *CookieState {
	return &CookieState{secret: secret, ttl: defaultStateTTL}
}

// NewCookieStateWithTTL creates a CookieState with a custom TTL (for testing).
func NewCookieStateWithTTL(secret []byte, ttl time.Duration) *CookieState {
	return &CookieState{secret: secret, ttl: ttl}
}

// Generate creates a new signed state token.
func (cs *CookieState) Generate() (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	nonceHex := hex.EncodeToString(nonce)
	timestampMS := strconv.FormatInt(time.Now().UnixMilli(), 10)

	payload := nonceHex + "." + timestampMS
	sig := cs.sign(payload)

	return payload + "." + sig, nil
}

// Verify checks that:
//  1. stateParam == cookieValue (binds state to the browser that started the flow)
//  2. HMAC signature is valid
//  3. Token hasn't expired
func (cs *CookieState) Verify(stateParam, cookieValue string) bool {
	// 1. State param must equal the cookie value
	if stateParam != cookieValue {
		return false
	}

	// Parse token: nonce.timestamp.signature
	parts := strings.SplitN(stateParam, ".", 3)
	if len(parts) != 3 {
		return false
	}

	nonce, timestampStr, sig := parts[0], parts[1], parts[2]

	// 2. Verify HMAC signature
	payload := nonce + "." + timestampStr
	expectedSig := cs.sign(payload)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return false
	}

	// 3. Check expiry
	timestampMS, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return false
	}
	created := time.UnixMilli(timestampMS)
	if time.Since(created) > cs.ttl {
		return false
	}

	return true
}

func (cs *CookieState) sign(payload string) string {
	mac := hmac.New(sha256.New, cs.secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
