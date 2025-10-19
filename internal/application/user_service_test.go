package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

type userRepoStub struct {
	createErr error
	created   User

	getUser User
	getErr  error

	updateErr error
	updated   User

	deleteErr error
	deletedID string

	list    []User
	listErr error
}

func (u *userRepoStub) CreateUser(ctx context.Context, user User) (User, error) {
	if u.createErr != nil {
		return User{}, u.createErr
	}
	u.created = user
	return user, nil
}

func (u *userRepoStub) GetUser(ctx context.Context, id string) (User, error) {
	if u.getErr != nil {
		return User{}, u.getErr
	}
	if u.getUser.ID == "" {
		return User{}, ErrNotFound
	}
	return u.getUser, nil
}

func (u *userRepoStub) UpdateUser(ctx context.Context, user User) (User, error) {
	if u.updateErr != nil {
		return User{}, u.updateErr
	}
	u.updated = user
	return user, nil
}

func (u *userRepoStub) DeleteUser(ctx context.Context, id string) error {
	if u.deleteErr != nil {
		return u.deleteErr
	}
	u.deletedID = id
	return nil
}

func (u *userRepoStub) ListUsers(ctx context.Context) ([]User, error) {
	if u.listErr != nil {
		return nil, u.listErr
	}
	if len(u.list) == 0 {
		return nil, nil
	}
	out := make([]User, len(u.list))
	copy(out, u.list)
	return out, nil
}

func TestUserService_CreateUser(t *testing.T) {
	t.Parallel()

	t.Run("requires administrator privileges", func(t *testing.T) {
		t.Parallel()
		svc := NewUserService(nil, nil, nil)

		_, err := svc.CreateUser(context.Background(), CreateUserParams{
			Principal: Principal{IsAdmin: false},
			Input: UserInput{
				Email:       "employee@example.com",
				DisplayName: "Employee",
			},
		})

		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("validates input fields including email format", func(t *testing.T) {
		t.Parallel()
		svc := NewUserService(nil, nil, nil)

		_, err := svc.CreateUser(context.Background(), CreateUserParams{
			Principal: Principal{IsAdmin: true},
			Input: UserInput{
				Email:       "not-an-email",
				DisplayName: "   ",
			},
		})

		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected ValidationError, got %v", err)
		}

		if _, ok := vErr.FieldErrors["email"]; !ok {
			t.Fatalf("expected email validation error, got %v", vErr.FieldErrors)
		}
		if _, ok := vErr.FieldErrors["display_name"]; !ok {
			t.Fatalf("expected display_name validation error, got %v", vErr.FieldErrors)
		}
	})

	t.Run("persists users for administrators", func(t *testing.T) {
		t.Parallel()
		repo := &userRepoStub{}
		now := time.Date(2024, time.March, 14, 9, 0, 0, 0, time.UTC)
		svc := NewUserService(repo, func() string { return "user-1" }, func() time.Time { return now })

		created, err := svc.CreateUser(context.Background(), CreateUserParams{
			Principal: Principal{IsAdmin: true},
			Input: UserInput{
				Email:       "  ADMIN@Example.com  ",
				DisplayName: "  Alice Admin  ",
				IsAdmin:     true,
			},
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if repo.created.ID != "user-1" {
			t.Fatalf("expected repository to receive generated ID, got %q", repo.created.ID)
		}
		if repo.created.Email != "admin@example.com" {
			t.Fatalf("expected email to be normalised, got %q", repo.created.Email)
		}
		if repo.created.DisplayName != "Alice Admin" {
			t.Fatalf("expected display name to be trimmed, got %q", repo.created.DisplayName)
		}
		if !repo.created.CreatedAt.Equal(now) || !repo.created.UpdatedAt.Equal(now) {
			t.Fatalf("expected timestamps to match injected clock, got created=%v updated=%v", repo.created.CreatedAt, repo.created.UpdatedAt)
		}
		if !repo.created.IsAdmin {
			t.Fatalf("expected admin flag to be preserved")
		}

		if created.ID != "user-1" {
			t.Fatalf("expected returned user to include ID, got %q", created.ID)
		}
	})

	t.Run("maps duplicate email violations to sentinel errors", func(t *testing.T) {
		t.Parallel()
		repo := &userRepoStub{createErr: persistence.ErrDuplicate}
		svc := NewUserService(repo, nil, nil)

		_, err := svc.CreateUser(context.Background(), CreateUserParams{
			Principal: Principal{IsAdmin: true},
			Input: UserInput{
				Email:       "employee@example.com",
				DisplayName: "Employee",
			},
		})

		if !errors.Is(err, ErrAlreadyExists) {
			t.Fatalf("expected ErrAlreadyExists, got %v", err)
		}
	})
}

func TestUserService_UpdateUser(t *testing.T) {
	t.Parallel()

	t.Run("requires administrator privileges", func(t *testing.T) {
		t.Parallel()
		svc := NewUserService(nil, nil, nil)

		_, err := svc.UpdateUser(context.Background(), UpdateUserParams{
			Principal: Principal{IsAdmin: false},
			UserID:    "user-1",
			Input: UserInput{
				Email:       "user@example.com",
				DisplayName: "User",
			},
		})

		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("validates input fields", func(t *testing.T) {
		t.Parallel()
		repo := &userRepoStub{getUser: User{ID: "user-1", Email: "user@example.com", DisplayName: "User"}}
		svc := NewUserService(repo, nil, nil)

		_, err := svc.UpdateUser(context.Background(), UpdateUserParams{
			Principal: Principal{IsAdmin: true},
			UserID:    "user-1",
			Input: UserInput{
				Email:       "invalid",
				DisplayName: "   ",
			},
		})

		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if _, ok := vErr.FieldErrors["email"]; !ok {
			t.Fatalf("expected email validation error, got %v", vErr.FieldErrors)
		}
		if _, ok := vErr.FieldErrors["display_name"]; !ok {
			t.Fatalf("expected display name validation error, got %v", vErr.FieldErrors)
		}
	})

	t.Run("propagates ErrNotFound when the user is missing", func(t *testing.T) {
		t.Parallel()
		repo := &userRepoStub{getErr: persistence.ErrNotFound}
		svc := NewUserService(repo, nil, nil)

		_, err := svc.UpdateUser(context.Background(), UpdateUserParams{
			Principal: Principal{IsAdmin: true},
			UserID:    "missing",
			Input: UserInput{
				Email:       "user@example.com",
				DisplayName: "User",
			},
		})

		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("allows administrators to modify users", func(t *testing.T) {
		t.Parallel()
		existing := User{ID: "user-1", Email: "user@example.com", DisplayName: "User", CreatedAt: time.Now(), UpdatedAt: time.Now()}
		repo := &userRepoStub{getUser: existing}
		now := time.Date(2024, time.March, 15, 9, 0, 0, 0, time.UTC)
		svc := NewUserService(repo, nil, func() time.Time { return now })

		updated, err := svc.UpdateUser(context.Background(), UpdateUserParams{
			Principal: Principal{IsAdmin: true},
			UserID:    "user-1",
			Input: UserInput{
				Email:       "  updated@example.com  ",
				DisplayName: "  Updated User  ",
				IsAdmin:     true,
			},
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if repo.updated.Email != "updated@example.com" {
			t.Fatalf("expected email to be normalised, got %q", repo.updated.Email)
		}
		if repo.updated.DisplayName != "Updated User" {
			t.Fatalf("expected display name to be trimmed, got %q", repo.updated.DisplayName)
		}
		if !repo.updated.UpdatedAt.Equal(now) {
			t.Fatalf("expected updated timestamp to use injected clock, got %v", repo.updated.UpdatedAt)
		}
		if repo.updated.CreatedAt != existing.CreatedAt {
			t.Fatalf("expected created timestamp to remain unchanged")
		}
		if !updated.UpdatedAt.Equal(now) {
			t.Fatalf("expected returned user to include updated timestamp, got %v", updated.UpdatedAt)
		}
	})
}

func TestUserService_ListUsers(t *testing.T) {
	t.Parallel()

	t.Run("requires administrator privileges", func(t *testing.T) {
		t.Parallel()
		svc := NewUserService(nil, nil, nil)

		_, err := svc.ListUsers(context.Background(), Principal{IsAdmin: false})
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("returns users in deterministic order", func(t *testing.T) {
		t.Parallel()
		repo := &userRepoStub{list: []User{
			{ID: "user-2", Email: "zeta@example.com", DisplayName: "Zeta"},
			{ID: "user-3", Email: "Alpha@example.com", DisplayName: "Alpha"},
			{ID: "user-1", Email: "alpha@example.com", DisplayName: "Alpha"},
		}}
		svc := NewUserService(repo, nil, nil)

		users, err := svc.ListUsers(context.Background(), Principal{IsAdmin: true})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if len(users) != 3 {
			t.Fatalf("expected three users, got %d", len(users))
		}

		if users[0].ID != "user-1" || users[1].ID != "user-3" || users[2].ID != "user-2" {
			t.Fatalf("expected case-insensitive ordering, got %+v", users)
		}
	})

	t.Run("supports filtering and pagination", func(t *testing.T) {
		t.Parallel()
		repo := &userRepoStub{list: []User{{ID: "user-1", Email: "user@example.com", DisplayName: "User"}}}
		svc := NewUserService(repo, nil, nil)

		users, err := svc.ListUsers(context.Background(), Principal{IsAdmin: true})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if len(users) != 1 || users[0].ID != "user-1" {
			t.Fatalf("expected repository results to pass through, got %v", users)
		}
	})
}

func TestUserService_DeleteUser(t *testing.T) {
	t.Parallel()

	t.Run("requires administrator privileges", func(t *testing.T) {
		t.Parallel()
		svc := NewUserService(nil, nil, nil)

		err := svc.DeleteUser(context.Background(), Principal{IsAdmin: false}, "user-1")
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("propagates ErrNotFound when the user is missing", func(t *testing.T) {
		t.Parallel()
		repo := &userRepoStub{deleteErr: persistence.ErrNotFound}
		svc := NewUserService(repo, nil, nil)

		err := svc.DeleteUser(context.Background(), Principal{IsAdmin: true}, "missing")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("allows administrators to delete users", func(t *testing.T) {
		t.Parallel()
		repo := &userRepoStub{}
		svc := NewUserService(repo, nil, nil)

		if err := svc.DeleteUser(context.Background(), Principal{IsAdmin: true}, "user-1"); err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if repo.deletedID != "user-1" {
			t.Fatalf("expected repository to receive user ID, got %q", repo.deletedID)
		}
	})
}
