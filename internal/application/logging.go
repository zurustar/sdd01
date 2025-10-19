package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/example/enterprise-scheduler/internal/logging"
)

func defaultLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}

func serviceLogger(ctx context.Context, base *slog.Logger, serviceName, operation string, attrs ...any) *slog.Logger {
	logger := logging.FromContext(ctx)
	if logger == nil {
		logger = base
	}
	if logger == nil {
		logger = slog.Default()
	}

	pairs := []any{"service", serviceName}
	if operation != "" {
		pairs = append(pairs, "operation", operation)
	}
	if len(attrs) > 0 {
		pairs = append(pairs, attrs...)
	}
	return logger.With(pairs...)
}

// ErrorKind maps sentinel and validation errors to a stable logging label.
func ErrorKind(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrAlreadyExists):
		return "already_exists"
	case errors.Is(err, ErrInvalidCredentials):
		return "invalid_credentials"
	case errors.Is(err, ErrAccountDisabled):
		return "account_disabled"
	case errors.Is(err, ErrSessionExpired):
		return "session_expired"
	case errors.Is(err, ErrSessionRevoked):
		return "session_revoked"
	}

	var vErr *ValidationError
	if errors.As(err, &vErr) {
		return "validation"
	}

	return "unexpected"
}
