package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
)

type authService interface {
	Authenticate(ctx context.Context, email, password string) (AuthSession, error)
	RevokeSession(ctx context.Context, token string) error
}

type AuthSession struct {
	Token     string
	ExpiresAt time.Time
	Principal application.Principal
}

type AuthHandler struct {
	service   authService
	responder responder
	logger    *slog.Logger
}

func NewAuthHandler(service authService, logger *slog.Logger) *AuthHandler {
	base := defaultLogger(logger)
	return &AuthHandler{service: service, responder: newResponder(base), logger: base}
}

func (h *AuthHandler) log(ctx context.Context, operation string, attrs ...any) *slog.Logger {
	if h == nil {
		return slog.Default()
	}
	return handlerLogger(ctx, h.logger, "AuthHandler", operation, attrs...)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log(r.Context(), "Login", "error_kind", "bad_request").ErrorContext(r.Context(), "failed to decode login request", "error", err)
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errBadRequestBody)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	logger := h.log(r.Context(), "Login", "email", email)

	session, err := h.service.Authenticate(r.Context(), email, req.Password)
	if err != nil {
		if errors.Is(err, application.ErrUnauthorized) {
			logger.ErrorContext(r.Context(), "authentication rejected", "error", err, "error_kind", application.ErrorKind(err))
			h.responder.writeJSON(r.Context(), w, http.StatusUnauthorized, errorResponse{Message: "メールアドレスまたはパスワードが正しくありません。"})
			return
		}
		logger.ErrorContext(r.Context(), "authentication failed", "error", err, "error_kind", application.ErrorKind(err))
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	setSessionCookie(w, session.Token, session.ExpiresAt)
	w.Header().Set("X-Session-Token", session.Token)

	logger.With(
		"user_id", session.Principal.UserID,
	).InfoContext(r.Context(), "user authenticated")

	h.responder.writeJSON(r.Context(), w, http.StatusOK, loginResponse{
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339Nano),
		Principal: principalDTO{
			UserID:  session.Principal.UserID,
			IsAdmin: session.Principal.IsAdmin,
		},
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	token := extractTokenFromRequest(r)
	if token == "" {
		h.log(r.Context(), "Logout", "error_kind", "bad_request").ErrorContext(r.Context(), "missing session token for logout")
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errMissingSessionToken)
		return
	}

	logger := h.log(r.Context(), "Logout", "token_present", true)

	if err := h.service.RevokeSession(r.Context(), token); err != nil {
		logger.ErrorContext(r.Context(), "failed to revoke session", "error", err, "error_kind", application.ErrorKind(err))
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	clearSessionCookie(w)
	logger.InfoContext(r.Context(), "user logged out")
	h.responder.writeJSON(r.Context(), w, http.StatusNoContent, nil)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string       `json:"token"`
	ExpiresAt string       `json:"expires_at"`
	Principal principalDTO `json:"principal"`
}

type principalDTO struct {
	UserID  string `json:"user_id"`
	IsAdmin bool   `json:"is_admin"`
}

func setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    token,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	}
	if !expires.IsZero() {
		cookie.Expires = expires.UTC()
	}
	http.SetCookie(w, cookie)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
	})
}

func extractTokenFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if header := strings.TrimSpace(r.Header.Get("Authorization")); header != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(header, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(header, prefix))
		}
	}
	if cookie, err := r.Cookie("session_token"); err == nil {
		return cookie.Value
	}
	return ""
}
