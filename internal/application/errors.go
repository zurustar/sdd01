package application

import "errors"

var (
	// ErrUnauthorized is returned when the acting principal lacks permission for an operation.
	ErrUnauthorized = errors.New("application: unauthorized")
	// ErrNotFound is returned when the requested resource does not exist.
	ErrNotFound = errors.New("application: not found")
)

// ValidationError captures field level validation issues that callers can surface to users.
type ValidationError struct {
	FieldErrors map[string]string
}

// Error implements the error interface.
func (v *ValidationError) Error() string {
	if v == nil {
		return ""
	}
	if len(v.FieldErrors) == 0 {
		return "validation failed"
	}
	return "validation failed"
}

// HasErrors reports whether any field level issues were recorded.
func (v *ValidationError) HasErrors() bool {
	return v != nil && len(v.FieldErrors) > 0
}

// add records a field level validation error.
func (v *ValidationError) add(field, message string) {
	if v.FieldErrors == nil {
		v.FieldErrors = make(map[string]string)
	}
	v.FieldErrors[field] = message
}

// merge copies entries from another validation error into the receiver.
func (v *ValidationError) merge(other *ValidationError) {
	if other == nil || len(other.FieldErrors) == 0 {
		return
	}
	for field, msg := range other.FieldErrors {
		v.add(field, msg)
	}
}
