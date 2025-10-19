package testfixtures

import (
	"context"
	"testing"

	"github.com/example/enterprise-scheduler/internal/application"
)

type capturingUserRepo struct {
	created application.User
}

func (c *capturingUserRepo) CreateUser(ctx context.Context, user application.User) (application.User, error) {
	c.created = user
	return user, nil
}

func (c *capturingUserRepo) GetUser(ctx context.Context, id string) (application.User, error) {
	return application.User{}, application.ErrNotFound
}

func (c *capturingUserRepo) UpdateUser(ctx context.Context, user application.User) (application.User, error) {
	return user, nil
}

func (c *capturingUserRepo) DeleteUser(ctx context.Context, id string) error {
	return nil
}

func (c *capturingUserRepo) ListUsers(ctx context.Context) ([]application.User, error) {
	return nil, nil
}

func TestServiceFactoryNewUserService(t *testing.T) {
	factory := NewServiceFactory()
	repo := &capturingUserRepo{}

	svc := factory.NewUserService(UserServiceDeps{Users: repo})
	principal := application.Principal{UserID: "admin", IsAdmin: true}
	input := application.UserInput{Email: "user@example.com", DisplayName: "User"}

	user, err := svc.CreateUser(context.Background(), application.CreateUserParams{Principal: principal, Input: input})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if user.ID != "id-1" {
		t.Fatalf("expected generated ID id-1, got %q", user.ID)
	}
	if repo.created.ID != user.ID {
		t.Fatalf("repository received unexpected ID: %q", repo.created.ID)
	}
	if !user.CreatedAt.Equal(factory.Clock.Current()) {
		t.Fatalf("expected timestamp %v, got %v", factory.Clock.Current(), user.CreatedAt)
	}
}
