package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCookieState_GenerateAndVerify(t *testing.T) {
	cs := NewCookieState([]byte("test-secret-key"))

	token, err := cs.Generate()
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Same value in both params → valid (simulates cookie matching query param).
	ok := cs.Verify(token, token)
	require.True(t, ok)
}

func TestCookieState_Verify_Invalid(t *testing.T) {
	cs := NewCookieState([]byte("test-secret-key"))

	// Garbage input should fail gracefully.
	require.False(t, cs.Verify("garbage", "garbage"))
	require.False(t, cs.Verify("", ""))
	require.False(t, cs.Verify("a.b", "a.b"))       // only 2 parts
	require.False(t, cs.Verify("a.b.c.d", "a.b.c.d")) // won't match HMAC
}

func TestCookieState_Verify_Mismatch(t *testing.T) {
	cs := NewCookieState([]byte("test-secret-key"))

	token1, err := cs.Generate()
	require.NoError(t, err)

	token2, err := cs.Generate()
	require.NoError(t, err)

	// Different stateParam vs cookieValue → rejected.
	require.False(t, cs.Verify(token1, token2))
	require.False(t, cs.Verify(token2, token1))
}

func TestCookieState_Verify_Expired(t *testing.T) {
	// Use a very short TTL.
	cs := NewCookieStateWithTTL([]byte("test-secret-key"), 1*time.Millisecond)

	token, err := cs.Generate()
	require.NoError(t, err)

	// Wait for token to expire.
	time.Sleep(5 * time.Millisecond)

	ok := cs.Verify(token, token)
	require.False(t, ok)
}

func TestCookieState_Verify_WrongSecret(t *testing.T) {
	cs1 := NewCookieState([]byte("secret-one"))
	cs2 := NewCookieState([]byte("secret-two"))

	token, err := cs1.Generate()
	require.NoError(t, err)

	// Token generated with secret-one should fail verification with secret-two.
	ok := cs2.Verify(token, token)
	require.False(t, ok)
}
