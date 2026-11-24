package db

import (
	"crypto/rand"
	"encoding/base64"
)

func generateInitialPassword() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "ChangeMe!2026"
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
