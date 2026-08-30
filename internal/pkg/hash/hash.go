package hash

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen  = 16
	keyLen   = 32
	timeCost = 3
	memCost  = 64 * 1024 // KiB
	threads  = 2
)

var ErrMismatch = errors.New("password mismatch")

// Hash возвращает строку в формате PHC: $argon2id$v=19$m=...,t=...,p=...$salt$hash.
func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	sum := argon2.IDKey([]byte(password), salt, timeCost, memCost, threads, keyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(sum)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memCost, timeCost, threads, b64Salt, b64Hash), nil
}

// Verify сверяет пароль с сохранённым хэшем.
func Verify(password, encoded string) error {
	mem, time, threads, salt, sum, err := decode(encoded)
	if err != nil {
		return err
	}

	other := argon2.IDKey([]byte(password), salt, time, mem, threads, uint32(len(sum)))
	if subtle.ConstantTimeCompare(sum, other) != 1 {
		return ErrMismatch
	}
	return nil
}

func decode(encoded string) (mem, time uint32, threads uint8, salt, sum []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid hash format")
	}

	var p uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &time, &p); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("parse params: %w", err)
	}
	threads = uint8(p)

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("decode salt: %w", err)
	}

	sum, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("decode hash: %w", err)
	}
	return mem, time, threads, salt, sum, nil
}
