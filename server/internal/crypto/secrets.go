package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

var warnFallbackOnce sync.Once

// encryptionPassphrase returns the passphrase used for AES-256-GCM key
// derivation. Priority: XREADER_AI_ENCRYPTION_KEY → SESSION_SECRET.
// SESSION_SECRET is required in production (enforced by main), making the
// encryption key deployment-unique without extra configuration.
// If neither is set (e.g. during setup wizard before config is complete),
// a warning is logged and SESSION_SECRET "change-me" dev fallback is used.
func encryptionPassphrase() string {
	if v := os.Getenv("XREADER_AI_ENCRYPTION_KEY"); v != "" {
		return v
	}
	if v := os.Getenv("SESSION_SECRET"); v != "" {
		return "xreader-encrypt:" + v
	}
	warnFallbackOnce.Do(func() {
		log.Println("WARNING: No XREADER_AI_ENCRYPTION_KEY or SESSION_SECRET set. " +
			"AI settings encryption is using an insecure default. " +
			"Set SESSION_SECRET to a strong random value for production use.")
	})
	return "xreader-encrypt:change-me"
}

// secretCipher builds an AES-256-GCM AEAD from the derived key.
func secretCipher() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(encryptionPassphrase()))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// EncryptSecret encrypts plaintext using AES-256-GCM. It returns the
// ciphertext and nonce separately. If plaintext is empty or whitespace-only,
// it returns (nil, nil, nil).
func EncryptSecret(plaintext string) (ciphertext []byte, nonce []byte, err error) {
	trimmed := strings.TrimSpace(plaintext)
	if trimmed == "" {
		return nil, nil, nil
	}
	gcm, err := secretCipher()
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, []byte(trimmed), nil), nonce, nil
}

// DecryptSecret decrypts ciphertext produced by EncryptSecret using the
// matching nonce. The same encryption passphrase must be available in the
// environment (or the default is used).
func DecryptSecret(ciphertext, nonce []byte) (string, error) {
	gcm, err := secretCipher()
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
