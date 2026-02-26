package dal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrNotFound         = errors.New("entity not found")
	ErrOperationBlocked = errors.New("operation is blocked")
)

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T {
	return &v
}

// GenerateUID creates a secure, prefixed identifier (e.g., user_3f8b9a2...).
func GenerateUID(prefix string) string {
	// Generate 12 random bytes (yields a 24-character hex string)
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// Fallback to panic only if the system's cryptographically secure RNG fails
		panic(fmt.Errorf("failed to read random bytes for uid: %w", err))
	}

	randomStr := hex.EncodeToString(b)
	return fmt.Sprintf("%s_%s", prefix, randomStr)
}
