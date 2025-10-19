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
}

func NewUserHandler(service userService, logger *slog.Logger) *UserHandler {
	return &UserHandler{service: service, responder: newResponder(logger)}
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())

	var req userRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errBadRequestBody)
		return
	}

	user, err := h.service.CreateUser(r.Context(), application.CreateUserParams{
		Principal: principal,
		Input:     req.toInput(),
	})
	if err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	h.responder.writeJSON(r.Context(), w, http.StatusCreated, userResponse{User: toUserDTO(user)})
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	userID, ok := UserIDFromContext(r.Context())
	if !ok || strings.TrimSpace(userID) == "" {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errInvalidUserID)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())

	var req userRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errBadRequestBody)
		return
	}

	user, err := h.service.UpdateUser(r.Context(), application.UpdateUserParams{
		Principal: principal,
		UserID:    userID,
		Input:     req.toInput(),
	})
	if err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	h.responder.writeJSON(r.Context(), w, http.StatusOK, userResponse{User: toUserDTO(user)})
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	userID, ok := UserIDFromContext(r.Context())
	if !ok || strings.TrimSpace(userID) == "" {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errInvalidUserID)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())
	if err := h.service.DeleteUser(r.Context(), principal, userID); err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	h.responder.writeJSON(r.Context(), w, http.StatusNoContent, nil)
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())
	users, err := h.service.ListUsers(r.Context(), principal)
	if err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

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
