package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
)

type SessionValidator interface {
	ValidateSession(ctx context.Context, token string) (application.Principal, error)
}

func RequireSession(validator SessionValidator, logger *slog.Logger) func(http.Handler) http.Handler {
	responder := newResponder(logger)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractTokenFromRequest(r)
			if token == "" {
				responder.writeError(r.Context(), w, http.StatusUnauthorized, errMissingSessionToken)
				return
			}

			principal, err := validator.ValidateSession(r.Context(), token)
			if err != nil {
				switch {
				case errors.Is(err, application.ErrUnauthorized):
					responder.writeJSON(r.Context(), w, http.StatusUnauthorized, errorResponse{Message: "セッションが無効です。再度ログインしてください。"})
				case errors.Is(err, application.ErrNotFound):
					responder.writeJSON(r.Context(), w, http.StatusUnauthorized, errorResponse{Message: "セッションが見つかりません。再度ログインしてください。"})
				default:
					responder.writeJSON(r.Context(), w, http.StatusInternalServerError, errorResponse{Message: "セッション検証中にエラーが発生しました。"})
				}
				return
			}

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
