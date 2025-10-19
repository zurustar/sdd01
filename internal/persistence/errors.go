package persistence

import "errors"

var (
	// ErrNotFound is returned when the requested record does not exist.
	ErrNotFound = errors.New("persistence: not found")
	// ErrDuplicate indicates a unique constraint violation.
	ErrDuplicate = errors.New("persistence: duplicate")
	// ErrForeignKeyViolation indicates that a foreign key constraint was violated.
	ErrForeignKeyViolation = errors.New("persistence: foreign key violation")
	// ErrConstraintViolation is returned when a database-level check constraint is violated.
	ErrConstraintViolation = errors.New("persistence: constraint violation")
)
