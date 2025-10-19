package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

// UserRepository captures the persistence operations needed by the user service.
type UserRepository interface {
	CreateUser(ctx context.Context, user User) (User, error)
	GetUser(ctx context.Context, id string) (User, error)
	UpdateUser(ctx context.Context, user User) (User, error)
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context) ([]User, error)
}

// UserService orchestrates validation, authorization, and persistence for users.
type UserService struct {
	users       UserRepository
	idGenerator func() string
	now         func() time.Time
	logger      *slog.Logger
}

// NewUserService wires dependencies for the user service.
func NewUserService(users UserRepository, idGenerator func() string, now func() time.Time) *UserService {
	return NewUserServiceWithLogger(users, idGenerator, now, nil)
}

// NewUserServiceWithLogger wires dependencies for the user service and accepts a logger.
func NewUserServiceWithLogger(users UserRepository, idGenerator func() string, now func() time.Time, logger *slog.Logger) *UserService {
	if idGenerator == nil {
		idGenerator = func() string { return "" }
	}
	if now == nil {
		now = time.Now
	}
	return &UserService{users: users, idGenerator: idGenerator, now: now, logger: defaultLogger(logger)}
}

func (s *UserService) loggerWith(ctx context.Context, operation string, attrs ...any) *slog.Logger {
	return serviceLogger(ctx, s.logger, "UserService", operation, attrs...)
}

// CreateUser validates input and persists a new user for administrators.
func (s *UserService) CreateUser(ctx context.Context, params CreateUserParams) (user User, err error) {
	if s == nil {
		err = fmt.Errorf("UserService is nil")
		return
	}
	logger := s.loggerWith(ctx, "CreateUser",
		"principal_id", params.Principal.UserID,
	)
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "failed to create user", "error", err, "error_kind", ErrorKind(err))
			return
		}
		logger.With("user_id", user.ID).InfoContext(ctx, "user created")
	}()

	if !params.Principal.IsAdmin {
		err = ErrUnauthorized
		return
	}

	normalized := normalizeUserInput(params.Input)
	vErr := validateUserInput(normalized)
	if vErr.HasErrors() {
		err = vErr
		return
	}

	user = User{
		ID:          s.idGenerator(),
		Email:       normalized.Email,
		DisplayName: normalized.DisplayName,
		IsAdmin:     normalized.IsAdmin,
		CreatedAt:   s.now(),
	}
	user.UpdatedAt = user.CreatedAt

	if s.users == nil {
		return
	}

	var persisted User
	persisted, err = s.users.CreateUser(ctx, user)
	if err != nil {
		err = mapUserRepoError(err)
		return
	}

	user = persisted
	return
}

// UpdateUser validates input and updates an existing user for administrators.
func (s *UserService) UpdateUser(ctx context.Context, params UpdateUserParams) (user User, err error) {
	if s == nil {
		err = fmt.Errorf("UserService is nil")
		return
	}
	if !params.Principal.IsAdmin {
		err = ErrUnauthorized
		return
	}
	if s.users == nil {
		err = fmt.Errorf("user repository not configured")
		return
	}

	logger := s.loggerWith(ctx, "UpdateUser",
		"principal_id", params.Principal.UserID,
		"user_id", params.UserID,
	)
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "failed to update user", "error", err, "error_kind", ErrorKind(err))
			return
		}
		logger.With("user_id", user.ID).InfoContext(ctx, "user updated")
	}()

	user, err = s.users.GetUser(ctx, params.UserID)
	if err != nil {
		err = mapUserRepoError(err)
		return
	}

	normalized := normalizeUserInput(params.Input)
	vErr := validateUserInput(normalized)
	if vErr.HasErrors() {
		err = vErr
		return
	}

	user.Email = normalized.Email
	user.DisplayName = normalized.DisplayName
	user.IsAdmin = normalized.IsAdmin
	user.UpdatedAt = s.now()

	user, err = s.users.UpdateUser(ctx, user)
	if err != nil {
		err = mapUserRepoError(err)
		return
	}

	return
}

// DeleteUser removes a user when requested by an administrator.
func (s *UserService) DeleteUser(ctx context.Context, principal Principal, userID string) error {
	if s == nil {
		return fmt.Errorf("UserService is nil")
	}
	if !principal.IsAdmin {
		return ErrUnauthorized
	}
	if s.users == nil {
		return fmt.Errorf("user repository not configured")
	}

	logger := s.loggerWith(ctx, "DeleteUser",
		"principal_id", principal.UserID,
		"user_id", userID,
	)

	if err := s.users.DeleteUser(ctx, userID); err != nil {
		err = mapUserRepoError(err)
		logger.ErrorContext(ctx, "failed to delete user", "error", err, "error_kind", ErrorKind(err))
		return err
	}

	logger.InfoContext(ctx, "user deleted")
	return nil
}

// ListUsers returns all users for administrators.
func (s *UserService) ListUsers(ctx context.Context, principal Principal) (users []User, err error) {
	if s == nil {
		err = fmt.Errorf("UserService is nil")
		return
	}
	if !principal.IsAdmin {
		err = ErrUnauthorized
		return
	}
	if s.users == nil {
		return nil, nil
	}

	logger := s.loggerWith(ctx, "ListUsers",
		"principal_id", principal.UserID,
	)
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "failed to list users", "error", err, "error_kind", ErrorKind(err))
			return
		}
		logger.With("result_count", len(users)).InfoContext(ctx, "users listed")
	}()

	var raw []User
	raw, err = s.users.ListUsers(ctx)
	if err != nil {
		return
	}

	users = make([]User, len(raw))
	copy(users, raw)

	sort.Slice(users, func(i, j int) bool {
		if strings.EqualFold(users[i].Email, users[j].Email) {
			return users[i].ID < users[j].ID
		}
		return strings.ToLower(users[i].Email) < strings.ToLower(users[j].Email)
	})

	return
}

func normalizeUserInput(input UserInput) UserInput {
	email := strings.TrimSpace(input.Email)
	email = strings.ToLower(email)

	displayName := strings.TrimSpace(input.DisplayName)

	return UserInput{
		Email:       email,
		DisplayName: displayName,
		IsAdmin:     input.IsAdmin,
	}
}

func validateUserInput(input UserInput) *ValidationError {
	vErr := &ValidationError{}

	if input.Email == "" {
		vErr.add("email", "email is required")
	} else if _, err := mail.ParseAddress(input.Email); err != nil {
		vErr.add("email", "email is invalid")
	}

	if input.DisplayName == "" {
		vErr.add("display_name", "display name is required")
	}

	return vErr
}

func mapUserRepoError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, persistence.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, persistence.ErrDuplicate) {
		return ErrAlreadyExists
	}
	return err
}
