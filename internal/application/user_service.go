package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"
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
}

// NewUserService wires dependencies for the user service.
func NewUserService(users UserRepository, idGenerator func() string, now func() time.Time) *UserService {
	if idGenerator == nil {
		idGenerator = func() string { return "" }
	}
	if now == nil {
		now = time.Now
	}
	return &UserService{users: users, idGenerator: idGenerator, now: now}
}

// CreateUser validates input and persists a new user for administrators.
func (s *UserService) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	if s == nil {
		return User{}, fmt.Errorf("UserService is nil")
	}
	if !params.Principal.IsAdmin {
		return User{}, ErrUnauthorized
	}

	normalized := normalizeUserInput(params.Input)
	vErr := validateUserInput(normalized)
	if vErr.HasErrors() {
		return User{}, vErr
	}

	user := User{
		ID:          s.idGenerator(),
		Email:       normalized.Email,
		DisplayName: normalized.DisplayName,
		IsAdmin:     normalized.IsAdmin,
		CreatedAt:   s.now(),
	}
	user.UpdatedAt = user.CreatedAt

	if s.users == nil {
		return user, nil
	}

	persisted, err := s.users.CreateUser(ctx, user)
	if err != nil {
		return User{}, err
	}

	return persisted, nil
}

// UpdateUser validates input and updates an existing user for administrators.
func (s *UserService) UpdateUser(ctx context.Context, params UpdateUserParams) (User, error) {
	if s == nil {
		return User{}, fmt.Errorf("UserService is nil")
	}
	if !params.Principal.IsAdmin {
		return User{}, ErrUnauthorized
	}
	if s.users == nil {
		return User{}, fmt.Errorf("user repository not configured")
	}

	existing, err := s.users.GetUser(ctx, params.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}

	normalized := normalizeUserInput(params.Input)
	vErr := validateUserInput(normalized)
	if vErr.HasErrors() {
		return User{}, vErr
	}

	updated := existing
	updated.Email = normalized.Email
	updated.DisplayName = normalized.DisplayName
	updated.IsAdmin = normalized.IsAdmin
	updated.UpdatedAt = s.now()

	persisted, err := s.users.UpdateUser(ctx, updated)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}

	return persisted, nil
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

	if err := s.users.DeleteUser(ctx, userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	return nil
}

// ListUsers returns all users for administrators.
func (s *UserService) ListUsers(ctx context.Context, principal Principal) ([]User, error) {
	if s == nil {
		return nil, fmt.Errorf("UserService is nil")
	}
	if !principal.IsAdmin {
		return nil, ErrUnauthorized
	}
	if s.users == nil {
		return nil, nil
	}

	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]User, len(users))
	copy(out, users)

	sort.Slice(out, func(i, j int) bool {
		if strings.EqualFold(out[i].Email, out[j].Email) {
			return out[i].ID < out[j].ID
		}
		return strings.ToLower(out[i].Email) < strings.ToLower(out[j].Email)
	})

	return out, nil
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
