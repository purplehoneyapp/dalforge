package dal

import "errors"

var (
	ErrNotFound         = errors.New("entity not found")
	ErrOperationBlocked = errors.New("operation is blocked")
)

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T {
	return &v
}
