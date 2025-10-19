package http

import (
	"context"

	"github.com/example/enterprise-scheduler/internal/application"
)

type contextKey string

const (
	principalContextKey  contextKey = "principal"
	scheduleIDContextKey contextKey = "schedule_id"
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
