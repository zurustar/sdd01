package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/example/enterprise-scheduler/internal/persistence"
	"github.com/example/enterprise-scheduler/internal/persistence/sqlite/migration"
)

func TestUserRepository_CreateUser(t *testing.T) {
	repo, cleanup := setupUserRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	user := persistence.User{
		ID:           "user1",
		Email:        "test@example.com",
		DisplayName:  "Test User",
		PasswordHash: "hashed_password",
		IsAdmin:      false,
	}

	err := repo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Verify user was created
	retrieved, err := repo.GetUser(ctx, "user1")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if retrieved.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", retrieved.Email)
	}
	if retrieved.DisplayName != "Test User" {
		t.Errorf("Expected display name 'Test User', got '%s'", retrieved.DisplayName)
	}
}

func TestUserRepository_CreateUser_Duplicate(t *testing.T) {
	repo, cleanup := setupUserRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	user := persistence.User{
		ID:           "user1",
		Email:        "test@example.com",
		DisplayName:  "Test User",
		PasswordHash: "hashed_password",
		IsAdmin:      false,
	}

	// Create user first time
	err := repo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("First CreateUser failed: %v", err)
	}

	// Try to create same user again
	err = repo.CreateUser(ctx, user)
	if err == nil {
		t.Fatal("Expected duplicate error, got nil")
	}
}

func TestUserRepository_GetUserByEmail(t *testing.T) {
	repo, cleanup := setupUserRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	user := persistence.User{
		ID:           "user1",
		Email:        "test@example.com",
		DisplayName:  "Test User",
		PasswordHash: "hashed_password",
		IsAdmin:      false,
	}

	err := repo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Test case-insensitive email lookup
	retrieved, err := repo.GetUserByEmail(ctx, "TEST@EXAMPLE.COM")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}

	if retrieved.ID != "user1" {
		t.Errorf("Expected ID 'user1', got '%s'", retrieved.ID)
	}
}

func TestUserRepository_UpdateUser(t *testing.T) {
	repo, cleanup := setupUserRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	user := persistence.User{
		ID:           "user1",
		Email:        "test@example.com",
		DisplayName:  "Test User",
		PasswordHash: "hashed_password",
		IsAdmin:      false,
	}

	err := repo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Update user
	user.DisplayName = "Updated User"
	user.Email = "updated@example.com"
	err = repo.UpdateUser(ctx, user)
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetUser(ctx, "user1")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if retrieved.DisplayName != "Updated User" {
		t.Errorf("Expected display name 'Updated User', got '%s'", retrieved.DisplayName)
	}
	if retrieved.Email != "updated@example.com" {
		t.Errorf("Expected email 'updated@example.com', got '%s'", retrieved.Email)
	}
}

func TestUserRepository_ListUsers(t *testing.T) {
	repo, cleanup := setupUserRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple users
	users := []persistence.User{
		{
			ID:           "user1",
			Email:        "user1@example.com",
			DisplayName:  "User 1",
			PasswordHash: "hash1",
			IsAdmin:      false,
		},
		{
			ID:           "user2",
			Email:        "user2@example.com",
			DisplayName:  "User 2",
			PasswordHash: "hash2",
			IsAdmin:      true,
		},
	}

	for _, user := range users {
		err := repo.CreateUser(ctx, user)
		if err != nil {
			t.Fatalf("CreateUser failed for %s: %v", user.ID, err)
		}
	}

	// List users
	retrieved, err := repo.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 users, got %d", len(retrieved))
	}

	// Verify ordering (should be by created_at, then ID)
	if retrieved[0].ID != "user1" {
		t.Errorf("Expected first user to be 'user1', got '%s'", retrieved[0].ID)
	}
}

func TestUserRepository_DeleteUser(t *testing.T) {
	repo, cleanup := setupUserRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	user := persistence.User{
		ID:           "user1",
		Email:        "test@example.com",
		DisplayName:  "Test User",
		PasswordHash: "hashed_password",
		IsAdmin:      false,
	}

	err := repo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Delete user
	err = repo.DeleteUser(ctx, "user1")
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	// Verify user is deleted
	_, err = repo.GetUser(ctx, "user1")
	if err == nil {
		t.Fatal("Expected user to be deleted, but GetUser succeeded")
	}
}

func setupUserRepositoryTest(t *testing.T) (*UserRepository, func()) {
	// Create temporary database file
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create connection pool
	config := migration.TempFileTestSQLiteConfig(dbPath)
	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	// Run migrations to set up schema
	ctx := context.Background()
	migrationDir := filepath.Join("..", "..", "..", "persistence", "sqlite", "migrations")
	if _, err := os.Stat(migrationDir); os.IsNotExist(err) {
		// Create a minimal schema for testing
		_, err = pool.DB().ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS users (
				id TEXT PRIMARY KEY,
				email TEXT NOT NULL UNIQUE,
				display_name TEXT NOT NULL,
				password_hash TEXT NOT NULL,
				is_admin INTEGER NOT NULL DEFAULT 0,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			);
			
			CREATE TABLE IF NOT EXISTS schedule_participants (
				schedule_id TEXT NOT NULL,
				user_id TEXT NOT NULL,
				PRIMARY KEY (schedule_id, user_id),
				FOREIGN KEY (user_id) REFERENCES users(id)
			);
			
			CREATE TABLE IF NOT EXISTS schedules (
				id TEXT PRIMARY KEY,
				creator_id TEXT NOT NULL,
				FOREIGN KEY (creator_id) REFERENCES users(id)
			);
		`)
		if err != nil {
			t.Fatalf("Failed to create test schema: %v", err)
		}
	}

	repo := NewUserRepository(pool)

	cleanup := func() {
		pool.Close()
	}

	return repo, cleanup
}