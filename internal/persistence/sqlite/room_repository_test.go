package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/example/enterprise-scheduler/internal/persistence"
	"github.com/example/enterprise-scheduler/internal/persistence/sqlite/migration"
)

func TestRoomRepository_CreateRoom(t *testing.T) {
	repo, cleanup := setupRoomRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	room := persistence.Room{
		ID:       "room1",
		Name:     "Conference Room A",
		Capacity: 10,
		Location: "Building 1, Floor 2",
	}

	err := repo.CreateRoom(ctx, room)
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// Verify room was created
	retrieved, err := repo.GetRoom(ctx, "room1")
	if err != nil {
		t.Fatalf("GetRoom failed: %v", err)
	}

	if retrieved.Name != "Conference Room A" {
		t.Errorf("Expected name 'Conference Room A', got '%s'", retrieved.Name)
	}
	if retrieved.Capacity != 10 {
		t.Errorf("Expected capacity 10, got %d", retrieved.Capacity)
	}
	if retrieved.Location != "Building 1, Floor 2" {
		t.Errorf("Expected location 'Building 1, Floor 2', got '%s'", retrieved.Location)
	}
}

func TestRoomRepository_CreateRoom_InvalidCapacity(t *testing.T) {
	repo, cleanup := setupRoomRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	room := persistence.Room{
		ID:       "room1",
		Name:     "Conference Room A",
		Capacity: 0, // Invalid capacity
		Location: "Building 1, Floor 2",
	}

	err := repo.CreateRoom(ctx, room)
	if err == nil {
		t.Fatal("Expected constraint violation error for zero capacity, got nil")
	}
}

func TestRoomRepository_UpdateRoom(t *testing.T) {
	repo, cleanup := setupRoomRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	room := persistence.Room{
		ID:       "room1",
		Name:     "Conference Room A",
		Capacity: 10,
		Location: "Building 1, Floor 2",
	}

	err := repo.CreateRoom(ctx, room)
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// Update room
	room.Name = "Updated Conference Room"
	room.Capacity = 15
	room.Location = "Building 2, Floor 1"
	err = repo.UpdateRoom(ctx, room)
	if err != nil {
		t.Fatalf("UpdateRoom failed: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetRoom(ctx, "room1")
	if err != nil {
		t.Fatalf("GetRoom failed: %v", err)
	}

	if retrieved.Name != "Updated Conference Room" {
		t.Errorf("Expected name 'Updated Conference Room', got '%s'", retrieved.Name)
	}
	if retrieved.Capacity != 15 {
		t.Errorf("Expected capacity 15, got %d", retrieved.Capacity)
	}
}

func TestRoomRepository_ListRooms(t *testing.T) {
	repo, cleanup := setupRoomRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple rooms
	rooms := []persistence.Room{
		{
			ID:       "room2",
			Name:     "Conference Room B",
			Capacity: 8,
			Location: "Building 1, Floor 1",
		},
		{
			ID:       "room1",
			Name:     "Conference Room A",
			Capacity: 12,
			Location: "Building 1, Floor 2",
		},
	}

	for _, room := range rooms {
		err := repo.CreateRoom(ctx, room)
		if err != nil {
			t.Fatalf("CreateRoom failed for %s: %v", room.ID, err)
		}
	}

	// List rooms
	retrieved, err := repo.ListRooms(ctx)
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 rooms, got %d", len(retrieved))
	}

	// Verify ordering (should be by name, then ID)
	if retrieved[0].Name != "Conference Room A" {
		t.Errorf("Expected first room to be 'Conference Room A', got '%s'", retrieved[0].Name)
	}
	if retrieved[1].Name != "Conference Room B" {
		t.Errorf("Expected second room to be 'Conference Room B', got '%s'", retrieved[1].Name)
	}
}

func TestRoomRepository_DeleteRoom(t *testing.T) {
	repo, cleanup := setupRoomRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	room := persistence.Room{
		ID:       "room1",
		Name:     "Conference Room A",
		Capacity: 10,
		Location: "Building 1, Floor 2",
	}

	err := repo.CreateRoom(ctx, room)
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// Delete room
	err = repo.DeleteRoom(ctx, "room1")
	if err != nil {
		t.Fatalf("DeleteRoom failed: %v", err)
	}

	// Verify room is deleted
	_, err = repo.GetRoom(ctx, "room1")
	if err == nil {
		t.Fatal("Expected room to be deleted, but GetRoom succeeded")
	}
}

func setupRoomRepositoryTest(t *testing.T) (*RoomRepository, func()) {
	// Create temporary database file
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create connection pool
	config := migration.TempFileTestSQLiteConfig(dbPath)
	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	// Create minimal schema for testing
	ctx := context.Background()
	_, err = pool.DB().ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS rooms (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			capacity INTEGER NOT NULL CHECK (capacity > 0),
			location TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		
		CREATE TABLE IF NOT EXISTS schedules (
			id TEXT PRIMARY KEY,
			room_id TEXT,
			FOREIGN KEY (room_id) REFERENCES rooms(id)
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	repo := NewRoomRepository(pool)

	cleanup := func() {
		pool.Close()
	}

	return repo, cleanup
}