package application

import "errors"

var (
	// ErrUnauthorized is returned when the acting principal lacks permission for an operation.
	ErrUnauthorized = errors.New("application: unauthorized")
	// ErrNotFound is returned when the requested resource does not exist.
	ErrNotFound = errors.New("application: not found")
	// ErrAlreadyExists is returned when attempting to create a resource that already exists.
	ErrAlreadyExists = errors.New("application: already exists")
	// ErrInvalidCredentials indicates authentication failed due to incorrect credentials.
	ErrInvalidCredentials = errors.New("application: invalid credentials")
	// ErrAccountDisabled indicates the user account has been disabled.
	ErrAccountDisabled = errors.New("application: account disabled")
	// ErrSessionExpired indicates the session is no longer valid due to expiry.
	ErrSessionExpired = errors.New("application: session expired")
	// ErrSessionRevoked indicates the session has been explicitly revoked.
	ErrSessionRevoked = errors.New("application: session revoked")
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
