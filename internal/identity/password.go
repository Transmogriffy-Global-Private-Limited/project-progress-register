package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 19 * 1024
	argonIterations  = 2
	argonParallelism = 1
	argonSaltLength  = 16
	argonKeyLength   = 32
	passwordWorkers  = 4
)

type passwordHasher struct {
	slots chan struct{}
}

func newPasswordHasher() *passwordHasher {
	return &passwordHasher{slots: make(chan struct{}, passwordWorkers)}
}

func (h *passwordHasher) Hash(ctx context.Context, password string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return encodePasswordHash(salt, key), nil
}

func (h *passwordHasher) Verify(ctx context.Context, encoded, password string) (bool, error) {
	salt, expected, err := decodePasswordHash(encoded)
	if err != nil {
		return false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	defer h.release()
	actual := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func (h *passwordHasher) acquire(ctx context.Context) error {
	select {
	case h.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *passwordHasher) release() { <-h.slots }

func encodePasswordHash(salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

func decodePasswordHash(encoded string) ([]byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return nil, nil, fmt.Errorf("unsupported password hash format")
	}
	if parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", argonMemory, argonIterations, argonParallelism) {
		return nil, nil, fmt.Errorf("unsupported password hash parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != argonSaltLength {
		return nil, nil, fmt.Errorf("invalid password hash salt")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) != argonKeyLength {
		return nil, nil, fmt.Errorf("invalid password hash key")
	}
	return salt, key, nil
}
