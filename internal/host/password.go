package host

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 19 * 1024
	argonIterations  = 2
	argonParallelism = 1
	argonSaltLength  = 16
	argonKeyLength   = 32
	sessionIDLength  = 32
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("host: make password salt: %w", err)
	}
	key := argon2.IDKey(
		[]byte(password),
		salt,
		argonIterations,
		argonMemory,
		argonParallelism,
		argonKeyLength,
	)
	encoding := base64.RawStdEncoding
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonIterations,
		argonParallelism,
		encoding.EncodeToString(salt),
		encoding.EncodeToString(key),
	), nil
}

func newSessionID() (string, error) {
	value := make([]byte, sessionIDLength)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("host: make admin session ID: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
