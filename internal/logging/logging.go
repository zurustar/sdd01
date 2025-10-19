package logging

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// ContextWithLogger returns a derived context that carries the provided logger.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if ctx == nil || logger == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext extracts a logger previously attached to the context.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	logger, _ := ctx.Value(contextKey{}).(*slog.Logger)
	return logger
}
