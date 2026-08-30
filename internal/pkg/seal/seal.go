package seal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// Seal шифрует строку AES-256-GCM. key — произвольная строка, внутри хэшируется до 32 байт.
func Seal(key, plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Open расшифровывает blob, созданный Seal.
func Open(key string, blob []byte) (string, error) {
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(blob) < nonceSize {
		return "", fmt.Errorf("blob too short")
	}

	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

func deriveKey(key string) []byte {
	sum := sha256.Sum256([]byte(key))
	return sum[:]
}

// EncodeState кодирует подписанное oauth-state значение (base64url).
func EncodeState(key string, payload []byte) (string, error) {
	sealed, err := Seal(key, string(payload))
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// DecodeState декодирует oauth-state.
func DecodeState(key, encoded string) ([]byte, error) {
	blob, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}
	plain, err := Open(key, blob)
	if err != nil {
		return nil, fmt.Errorf("open state: %w", err)
	}
	return []byte(plain), nil
}
