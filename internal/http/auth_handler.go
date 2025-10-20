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
	Authenticate(ctx context.Context, params application.AuthenticateParams) (application.AuthenticateResult, error)
	RevokeSession(ctx context.Context, token string) error
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

func (h *AuthHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log(r.Context(), "CreateSession", "error_kind", "bad_request").ErrorContext(r.Context(), "failed to decode session request", "error", err)
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errBadRequestBody)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	logger := h.log(r.Context(), "CreateSession", "email", email)

	result, err := h.service.Authenticate(r.Context(), application.AuthenticateParams{
		Email:    email,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, application.ErrInvalidCredentials) {
			logger.ErrorContext(r.Context(), "authentication rejected", "error", err, "error_kind", application.ErrorKind(err))
			errResp := errorResponse{
				ErrorCode: "AUTH_INVALID_CREDENTIALS",
				Message:   "メールアドレスまたはパスワードが正しくありません",
			}
			h.responder.writeJSON(r.Context(), w, http.StatusUnauthorized, errResp)
			return
		}
		logger.ErrorContext(r.Context(), "authentication failed", "error", err, "error_kind", application.ErrorKind(err))
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	setSessionCookie(w, result.Session.Token, result.Session.ExpiresAt)
	w.Header().Set("X-Session-Token", result.Session.Token)

	logger.With(
		"user_id", result.User.ID,
	).InfoContext(r.Context(), "user authenticated")

	h.responder.writeJSON(r.Context(), w, http.StatusCreated, loginResponse{
		Token:     result.Session.Token,
		ExpiresAt: result.Session.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
}

func (h *AuthHandler) DeleteCurrentSession(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	token := extractTokenFromRequest(r)
	if token == "" {
		h.log(r.Context(), "DeleteCurrentSession", "error_kind", "unauthorized").ErrorContext(r.Context(), "missing session token for current session revocation")
		h.responder.writeJSON(r.Context(), w, http.StatusUnauthorized, errorResponse{
			ErrorCode: "AUTH_SESSION_EXPIRED",
			Message:   errMissingSessionToken.Error(),
		})
		return
	}

	logger := h.log(r.Context(), "DeleteCurrentSession", "token_present", true)

	if err := h.service.RevokeSession(r.Context(), token); err != nil {
		logger.ErrorContext(r.Context(), "failed to revoke session", "error", err, "error_kind", application.ErrorKind(err))
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	clearSessionCookie(w)
	logger.InfoContext(r.Context(), "session revoked for current principal")
	h.responder.writeJSON(r.Context(), w, http.StatusNoContent, nil)
}

func (h *AuthHandler) DeleteSession(w http.ResponseWriter, r *http.Request, token string) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	principal, ok := PrincipalFromContext(r.Context())
	if !ok || !principal.IsAdmin {
		h.log(r.Context(), "DeleteSession", "error_kind", "forbidden").ErrorContext(r.Context(), "non-administrator attempted session revocation")
		h.responder.writeJSON(r.Context(), w, http.StatusForbidden, errorResponse{
			ErrorCode: "AUTH_FORBIDDEN",
			Message:   "この操作を実行する権限がありません。",
		})
		return
	}

	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		h.log(r.Context(), "DeleteSession", "error_kind", "bad_request").ErrorContext(r.Context(), "empty token provided for admin revocation")
		h.responder.writeJSON(r.Context(), w, http.StatusBadRequest, errorResponse{Message: "失効対象のトークンを指定してください。"})
		return
	}

	logger := h.log(r.Context(), "DeleteSession", "token_present", true, "actor_id", principal.UserID)

	if err := h.service.RevokeSession(r.Context(), trimmed); err != nil {
		logger.ErrorContext(r.Context(), "failed to revoke session", "error", err, "error_kind", application.ErrorKind(err))
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	logger.InfoContext(r.Context(), "session revoked by administrator")
	h.responder.writeJSON(r.Context(), w, http.StatusNoContent, nil)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
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
