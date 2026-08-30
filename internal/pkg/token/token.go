package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// Generate создаёт opaque refresh-токен и его SHA-256 хэш для хранения в БД.
func Generate() (plain, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(raw)
	hash = Hash(plain)
	return plain, hash, nil
}

// Hash возвращает SHA-256 хэш токена в hex.
func Hash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
