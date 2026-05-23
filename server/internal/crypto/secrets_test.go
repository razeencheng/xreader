package crypto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	original := "sk-secret-api-key-12345"
	ciphertext, nonce, err := EncryptSecret(original)
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)
	require.NotEmpty(t, nonce)

	// Ciphertext must not contain the plaintext.
	require.NotContains(t, string(ciphertext), original)

	decrypted, err := DecryptSecret(ciphertext, nonce)
	require.NoError(t, err)
	require.Equal(t, original, decrypted)
}

func TestEncryptSecret_Empty(t *testing.T) {
	for _, input := range []string{"", "   ", "\t\n"} {
		ct, nonce, err := EncryptSecret(input)
		require.NoError(t, err, "input=%q", input)
		require.Nil(t, ct, "input=%q", input)
		require.Nil(t, nonce, "input=%q", input)
	}
}

func TestDecryptSecret_WrongKey(t *testing.T) {
	// Encrypt with the default passphrase.
	ciphertext, nonce, err := EncryptSecret("my-secret")
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)

	// Change the passphrase via the environment variable.
	t.Setenv("XREADER_AI_ENCRYPTION_KEY", "completely-different-key")

	// Decryption with the wrong key must fail.
	_, err = DecryptSecret(ciphertext, nonce)
	require.Error(t, err)
}
