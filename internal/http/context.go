package http

import (
	"context"
	"log/slog"

	"github.com/example/enterprise-scheduler/internal/application"
)

type contextKey string

const (
	principalContextKey  contextKey = "principal"
	scheduleIDContextKey contextKey = "schedule_id"
	userIDContextKey     contextKey = "user_id"
	roomIDContextKey     contextKey = "room_id"
	loggerContextKey     contextKey = "logger"
)

// ContextWithPrincipal returns a derived context containing the authenticated principal.
func ContextWithPrincipal(ctx context.Context, principal application.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey, principal)
}

// PrincipalFromContext extracts the authenticated principal from context if available.
func PrincipalFromContext(ctx context.Context) (application.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(application.Principal)
	return principal, ok
}

// ContextWithScheduleID injects the schedule identifier resolved from the request path.
func ContextWithScheduleID(ctx context.Context, scheduleID string) context.Context {
	return context.WithValue(ctx, scheduleIDContextKey, scheduleID)
}

// ScheduleIDFromContext extracts a schedule identifier previously associated with the context.
func ScheduleIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(scheduleIDContextKey).(string)
	return id, ok
}

// ContextWithUserID injects a user identifier extracted from the request path.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// UserIDFromContext extracts a user identifier previously associated with the context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDContextKey).(string)
	return id, ok
}

// ContextWithRoomID injects a room identifier extracted from the request path.
func ContextWithRoomID(ctx context.Context, roomID string) context.Context {
	return context.WithValue(ctx, roomIDContextKey, roomID)
}

// RoomIDFromContext extracts a room identifier previously associated with the context.
func RoomIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(roomIDContextKey).(string)
	return id, ok
}

// ContextWithLogger attaches a request scoped logger to the context.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

// LoggerFromContext retrieves the request scoped logger if present.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	logger, _ := ctx.Value(loggerContextKey).(*slog.Logger)
	return logger
}
