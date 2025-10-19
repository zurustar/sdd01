package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
)

type SessionValidator interface {
	ValidateSession(ctx context.Context, token string) (application.Principal, error)
}

func RequireSession(validator SessionValidator, logger *slog.Logger) func(http.Handler) http.Handler {
	base := defaultLogger(logger)
	responder := newResponder(base)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if validator == nil {
				base.ErrorContext(r.Context(), "session validator not configured", "middleware", "RequireSession")
				responder.writeJSON(r.Context(), w, http.StatusInternalServerError, errorResponse{Message: "セッション検証中にエラーが発生しました。"})
				return
			}

			audit := LoggerFromContext(r.Context())
			if audit == nil {
				audit = base
			}
			audit = audit.With("middleware", "RequireSession")

			token := strings.TrimSpace(extractTokenFromRequest(r))
			if token == "" {
				audit.ErrorContext(r.Context(), "session token missing", "error_kind", "unauthorized")
				responder.writeError(r.Context(), w, http.StatusUnauthorized, errMissingSessionToken)
				return
			}

			principal, err := validator.ValidateSession(r.Context(), token)
			if err != nil {
				switch {
				case errors.Is(err, application.ErrUnauthorized):
					audit.ErrorContext(r.Context(), "session invalid", "error", err, "error_kind", application.ErrorKind(err))
					responder.writeJSON(r.Context(), w, http.StatusUnauthorized, errorResponse{Message: "セッションが無効です。再度ログインしてください。"})
				case errors.Is(err, application.ErrNotFound):
					audit.ErrorContext(r.Context(), "session not found", "error", err, "error_kind", application.ErrorKind(err))
					responder.writeJSON(r.Context(), w, http.StatusUnauthorized, errorResponse{Message: "セッションが見つかりません。再度ログインしてください。"})
				case errors.Is(err, application.ErrSessionExpired):
					audit.ErrorContext(r.Context(), "session expired", "error", err, "error_kind", application.ErrorKind(err))
					responder.writeJSON(r.Context(), w, http.StatusUnauthorized, errorResponse{Message: "セッションの有効期限が切れています。再度ログインしてください。"})
				case errors.Is(err, application.ErrSessionRevoked):
					audit.ErrorContext(r.Context(), "session revoked", "error", err, "error_kind", application.ErrorKind(err))
					responder.writeJSON(r.Context(), w, http.StatusUnauthorized, errorResponse{Message: "セッションが取り消されました。再度ログインしてください。"})
				default:
					audit.ErrorContext(r.Context(), "session validation failed", "error", err, "error_kind", application.ErrorKind(err))
					responder.writeJSON(r.Context(), w, http.StatusInternalServerError, errorResponse{Message: "セッション検証中にエラーが発生しました。"})
				}
				return
			}

			audit.With("principal_id", principal.UserID).InfoContext(r.Context(), "session validated")
			ctx := ContextWithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	if base == nil {
		base = slog.Default()
	}
	var counter atomic.Uint64

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := counter.Add(1)
			logger := base.With(
				"request_id", id,
				"method", r.Method,
				"path", r.URL.Path,
			)

			ctx := ContextWithLogger(r.Context(), logger)
			start := time.Now()
			logger.InfoContext(ctx, "request started")
			next.ServeHTTP(w, r.WithContext(ctx))
			logger.InfoContext(ctx, "request completed", "duration", time.Since(start))
		})
	}
}
