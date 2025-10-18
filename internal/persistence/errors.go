package persistence

import "errors"

var (
	// ErrNotFound is returned when the requested record does not exist.
	ErrNotFound = errors.New("persistence: not found")
)
