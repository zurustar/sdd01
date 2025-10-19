package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
)

type userService interface {
	CreateUser(ctx context.Context, params application.CreateUserParams) (application.User, error)
	UpdateUser(ctx context.Context, params application.UpdateUserParams) (application.User, error)
	DeleteUser(ctx context.Context, principal application.Principal, userID string) error
	ListUsers(ctx context.Context, principal application.Principal) ([]application.User, error)
}

type UserHandler struct {
	service   userService
	responder responder
	logger    *slog.Logger
}

func NewUserHandler(service userService, logger *slog.Logger) *UserHandler {
	base := defaultLogger(logger)
	return &UserHandler{service: service, responder: newResponder(base), logger: base}
}

func (h *UserHandler) log(ctx context.Context, operation string, attrs ...any) *slog.Logger {
	if h == nil {
		return slog.Default()
	}
	return handlerLogger(ctx, h.logger, "UserHandler", operation, attrs...)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())

	var req userRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log(r.Context(), "Create", "principal_id", principal.UserID, "error_kind", "bad_request").ErrorContext(r.Context(), "failed to decode user request", "error", err)
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errBadRequestBody)
		return
	}

	logger := h.log(r.Context(), "Create", "principal_id", principal.UserID)

	user, err := h.service.CreateUser(r.Context(), application.CreateUserParams{
		Principal: principal,
		Input:     req.toInput(),
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "user creation failed", "error", err, "error_kind", application.ErrorKind(err))
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	logger.With("user_id", user.ID).InfoContext(r.Context(), "user created")
	h.responder.writeJSON(r.Context(), w, http.StatusCreated, userResponse{User: toUserDTO(user)})
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	userID, ok := UserIDFromContext(r.Context())
	if !ok || strings.TrimSpace(userID) == "" {
		h.log(r.Context(), "Update", "error_kind", "bad_request").ErrorContext(r.Context(), "missing user id for update")
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errInvalidUserID)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())

	var req userRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log(r.Context(), "Update", "principal_id", principal.UserID, "user_id", userID, "error_kind", "bad_request").ErrorContext(r.Context(), "failed to decode user update", "error", err)
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errBadRequestBody)
		return
	}

	logger := h.log(r.Context(), "Update", "principal_id", principal.UserID, "user_id", userID)

	user, err := h.service.UpdateUser(r.Context(), application.UpdateUserParams{
		Principal: principal,
		UserID:    userID,
		Input:     req.toInput(),
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "user update failed", "error", err, "error_kind", application.ErrorKind(err))
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	logger.InfoContext(r.Context(), "user updated")
	h.responder.writeJSON(r.Context(), w, http.StatusOK, userResponse{User: toUserDTO(user)})
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	userID, ok := UserIDFromContext(r.Context())
	if !ok || strings.TrimSpace(userID) == "" {
		h.log(r.Context(), "Delete", "error_kind", "bad_request").ErrorContext(r.Context(), "missing user id for delete")
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errInvalidUserID)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())
	logger := h.log(r.Context(), "Delete", "principal_id", principal.UserID, "user_id", userID)
	if err := h.service.DeleteUser(r.Context(), principal, userID); err != nil {
		logger.ErrorContext(r.Context(), "user delete failed", "error", err, "error_kind", application.ErrorKind(err))
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	logger.InfoContext(r.Context(), "user deleted")
	h.responder.writeJSON(r.Context(), w, http.StatusNoContent, nil)
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())
	logger := h.log(r.Context(), "List", "principal_id", principal.UserID)
	users, err := h.service.ListUsers(r.Context(), principal)
	if err != nil {
		logger.ErrorContext(r.Context(), "user list failed", "error", err, "error_kind", application.ErrorKind(err))
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	logger.With("result_count", len(users)).InfoContext(r.Context(), "users listed")
	h.responder.writeJSON(r.Context(), w, http.StatusOK, listUsersResponse{Users: toUserDTOs(users)})
}

type userRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	IsAdmin     bool   `json:"is_admin"`
}

func (r userRequest) toInput() application.UserInput {
	return application.UserInput{
		Email:       strings.TrimSpace(r.Email),
		DisplayName: strings.TrimSpace(r.DisplayName),
		IsAdmin:     r.IsAdmin,
	}
}

type userResponse struct {
	User userDTO `json:"user"`
}

type listUsersResponse struct {
	Users []userDTO `json:"users"`
}

type userDTO struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	IsAdmin     bool   `json:"is_admin"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toUserDTO(user application.User) userDTO {
	return userDTO{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		IsAdmin:     user.IsAdmin,
		CreatedAt:   user.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:   user.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func toUserDTOs(users []application.User) []userDTO {
	if len(users) == 0 {
		return nil
	}
	out := make([]userDTO, 0, len(users))
	for _, user := range users {
		out = append(out, toUserDTO(user))
	}
	return out
}
