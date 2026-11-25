package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

func keyFrom(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// EncryptSecret encrypts plaintext with AES-256-GCM. aad binds the ciphertext
// to a context (for MFA secrets this is the user ID).
func EncryptSecret(secret string, aad []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(keyFrom(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), aad)
	return base64.StdEncoding.EncodeToString(out), nil
}

func DecryptSecret(secret string, aad []byte, enc string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(keyFrom(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return "", fmt.Errorf("invalid secret")
	}
	pt, err := gcm.Open(nil, data[:ns], data[ns:], aad)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// DecryptSecretLegacy decrypts data encrypted with the old format (JWT secret,
// no associated data). It is only used to migrate existing MFA secrets.
func DecryptSecretLegacy(secret, enc string) (string, error) {
	return DecryptSecret(secret, nil, enc)
}
