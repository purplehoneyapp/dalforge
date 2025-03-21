package dal

import "errors"

var (
	ErrNotFound         = errors.New("entity not found")
	ErrOperationBlocked = errors.New("operation is blocked")
)
