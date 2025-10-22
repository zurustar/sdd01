package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
	"github.com/example/enterprise-scheduler/internal/persistence/sqlite/migration"
)

func TestScheduleRepository_CreateSchedule(t *testing.T) {
	repo, cleanup := setupScheduleRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	
	// Create test user first
	createTestUser(t, repo.pool, "user1", "creator@example.com")
	createTestUser(t, repo.pool, "user2", "participant@example.com")

	start := time.Now().UTC().Add(time.Hour)
	end := start.Add(time.Hour)
	memo := "Test meeting"
	
	schedule := persistence.Schedule{
		ID:          "schedule1",
		Title:       "Test Meeting",
		Start:       start,
		End:         end,
		CreatorID:   "user1",
		Memo:        &memo,
		Participants: []string{"user2"},
	}

	err := repo.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}

	// Verify schedule was created
	retrieved, err := repo.GetSchedule(ctx, "schedule1")
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}

	if retrieved.Title != "Test Meeting" {
		t.Errorf("Expected title 'Test Meeting', got '%s'", retrieved.Title)
	}
	if retrieved.CreatorID != "user1" {
		t.Errorf("Expected creator 'user1', got '%s'", retrieved.CreatorID)
	}
	if len(retrieved.Participants) != 1 || retrieved.Participants[0] != "user2" {
		t.Errorf("Expected participants ['user2'], got %v", retrieved.Participants)
	}
}

func TestScheduleRepository_CreateSchedule_InvalidTime(t *testing.T) {
	repo, cleanup := setupScheduleRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	
	// Create test user first
	createTestUser(t, repo.pool, "user1", "creator@example.com")

	start := time.Now().UTC().Add(time.Hour)
	end := start.Add(-time.Hour) // End before start - invalid
	
	schedule := persistence.Schedule{
		ID:        "schedule1",
		Title:     "Test Meeting",
		Start:     start,
		End:       end,
		CreatorID: "user1",
	}

	err := repo.CreateSchedule(ctx, schedule)
	if err == nil {
		t.Fatal("Expected constraint violation error for invalid time range, got nil")
	}
}

func TestScheduleRepository_UpdateSchedule(t *testing.T) {
	repo, cleanup := setupScheduleRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	
	// Create test users
	createTestUser(t, repo.pool, "user1", "creator@example.com")
	createTestUser(t, repo.pool, "user2", "participant1@example.com")
	createTestUser(t, repo.pool, "user3", "participant2@example.com")

	start := time.Now().UTC().Add(time.Hour)
	end := start.Add(time.Hour)
	
	schedule := persistence.Schedule{
		ID:           "schedule1",
		Title:        "Test Meeting",
		Start:        start,
		End:          end,
		CreatorID:    "user1",
		Participants: []string{"user2"},
	}

	err := repo.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}

	// Update schedule
	schedule.Title = "Updated Meeting"
	schedule.Participants = []string{"user2", "user3"}
	err = repo.UpdateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("UpdateSchedule failed: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetSchedule(ctx, "schedule1")
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}

	if retrieved.Title != "Updated Meeting" {
		t.Errorf("Expected title 'Updated Meeting', got '%s'", retrieved.Title)
	}
	if len(retrieved.Participants) != 2 {
		t.Errorf("Expected 2 participants, got %d", len(retrieved.Participants))
	}
}

func TestScheduleRepository_ListSchedules_WithFilter(t *testing.T) {
	repo, cleanup := setupScheduleRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	
	// Create test users
	createTestUser(t, repo.pool, "user1", "creator@example.com")
	createTestUser(t, repo.pool, "user2", "participant@example.com")

	now := time.Now().UTC()
	
	// Create multiple schedules
	schedules := []persistence.Schedule{
		{
			ID:        "schedule1",
			Title:     "Meeting 1",
			Start:     now.Add(time.Hour),
			End:       now.Add(2 * time.Hour),
			CreatorID: "user1",
		},
		{
			ID:           "schedule2",
			Title:        "Meeting 2",
			Start:        now.Add(3 * time.Hour),
			End:          now.Add(4 * time.Hour),
			CreatorID:    "user1",
			Participants: []string{"user2"},
		},
	}

	for _, schedule := range schedules {
		err := repo.CreateSchedule(ctx, schedule)
		if err != nil {
			t.Fatalf("CreateSchedule failed for %s: %v", schedule.ID, err)
		}
	}

	// Test filter by participant
	filter := persistence.ScheduleFilter{
		ParticipantIDs: []string{"user2"},
	}
	
	retrieved, err := repo.ListSchedules(ctx, filter)
	if err != nil {
		t.Fatalf("ListSchedules failed: %v", err)
	}

	if len(retrieved) != 1 {
		t.Errorf("Expected 1 schedule with user2 as participant, got %d", len(retrieved))
	}
	if len(retrieved) > 0 && retrieved[0].ID != "schedule2" {
		t.Errorf("Expected schedule2, got %s", retrieved[0].ID)
	}
}

func TestScheduleRepository_DeleteSchedule(t *testing.T) {
	repo, cleanup := setupScheduleRepositoryTest(t)
	defer cleanup()

	ctx := context.Background()
	
	// Create test user
	createTestUser(t, repo.pool, "user1", "creator@example.com")

	start := time.Now().UTC().Add(time.Hour)
	end := start.Add(time.Hour)
	
	schedule := persistence.Schedule{
		ID:        "schedule1",
		Title:     "Test Meeting",
		Start:     start,
		End:       end,
		CreatorID: "user1",
	}

	err := repo.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}

	// Delete schedule
	err = repo.DeleteSchedule(ctx, "schedule1")
	if err != nil {
		t.Fatalf("DeleteSchedule failed: %v", err)
	}

	// Verify schedule is deleted
	_, err = repo.GetSchedule(ctx, "schedule1")
	if err == nil {
		t.Fatal("Expected schedule to be deleted, but GetSchedule succeeded")
	}
}

func setupScheduleRepositoryTest(t *testing.T) (*ScheduleRepository, func()) {
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
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			is_admin INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		
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
			title TEXT NOT NULL,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			creator_id TEXT NOT NULL,
			room_id TEXT,
			memo TEXT,
			web_conference_url TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (creator_id) REFERENCES users(id),
			FOREIGN KEY (room_id) REFERENCES rooms(id)
		);
		
		CREATE TABLE IF NOT EXISTS schedule_participants (
			schedule_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			PRIMARY KEY (schedule_id, user_id),
			FOREIGN KEY (schedule_id) REFERENCES schedules(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		
		CREATE TABLE IF NOT EXISTS recurrences (
			id TEXT PRIMARY KEY,
			schedule_id TEXT NOT NULL,
			frequency TEXT NOT NULL,
			interval_value INTEGER NOT NULL,
			weekdays INTEGER NOT NULL DEFAULT 0,
			starts_on TEXT NOT NULL,
			ends_on TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (schedule_id) REFERENCES schedules(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	repo := NewScheduleRepository(pool)

	cleanup := func() {
		pool.Close()
	}

	return repo, cleanup
}

func createTestUser(t *testing.T, pool *ConnectionPool, id, email string) {
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339)
	
	_, err := pool.DB().ExecContext(ctx, `
		INSERT INTO users (id, email, display_name, password_hash, is_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, email, "Test User", "hash", 0, now, now)
	
	if err != nil {
		t.Fatalf("Failed to create test user %s: %v", id, err)
	}
}