package http

import (
	"context"
	"log/slog"
)

func defaultLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}

func handlerLogger(ctx context.Context, fallback *slog.Logger, handlerName, operation string, attrs ...any) *slog.Logger {
	logger := LoggerFromContext(ctx)
	if logger == nil {
		logger = fallback
	}
	if logger == nil {
		logger = slog.Default()
	}

	pairs := []any{"handler", handlerName}
	if operation != "" {
		pairs = append(pairs, "operation", operation)
	}
	if len(attrs) > 0 {
		pairs = append(pairs, attrs...)
	}
	return logger.With(pairs...)
}
