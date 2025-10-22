package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()

	dir := t.TempDir()
	dsn := filepath.Join(dir, "scheduler.db")
	storage, err := Open(dsn)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}

	t.Cleanup(func() {
		_ = storage.Close()
	})

	if err := storage.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return storage
}

func TestSplitSQLStatements(t *testing.T) {
	script := `-- enable foreign keys
PRAGMA foreign_keys = ON;

/* create users table */
CREATE TABLE users(id TEXT PRIMARY KEY);

INSERT INTO logs(message) VALUES('semicolon; inside string');
`

	statements := splitSQLStatements(script)
	expected := []string{
		"PRAGMA foreign_keys = ON",
		"CREATE TABLE users(id TEXT PRIMARY KEY)",
		"INSERT INTO logs(message) VALUES('semicolon; inside string')",
	}

	if len(statements) != len(expected) {
		t.Fatalf("expected %d statements, got %d: %#v", len(expected), len(statements), statements)
	}
	for i, stmt := range expected {
		if statements[i] != stmt {
			t.Fatalf("statement %d mismatch: got %q want %q", i, statements[i], stmt)
		}
	}
}

func TestUserRepository(t *testing.T) {
	ctx := context.Background()
	storage := newTestStorage(t)

	now := time.Now().UTC().Truncate(time.Second)
	user := persistence.User{
		ID:           "user-1",
		Email:        "alice@example.com",
		DisplayName:  "Alice",
		PasswordHash: "hash",
		IsAdmin:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := storage.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := storage.CreateUser(ctx, user); !errors.Is(err, persistence.ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate on second insert, got %v", err)
	}

	fetched, err := storage.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if fetched.Email != user.Email || !fetched.IsAdmin || fetched.PasswordHash != user.PasswordHash {
		t.Fatalf("unexpected user retrieved: %#v", fetched)
	}

	user.DisplayName = "Alice Updated"
	user.IsAdmin = false
	user.UpdatedAt = now.Add(time.Minute)
	if err := storage.UpdateUser(ctx, user); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	fetched, err = storage.GetUserByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if fetched.DisplayName != "Alice Updated" || fetched.IsAdmin || fetched.PasswordHash != user.PasswordHash {
		t.Fatalf("unexpected user after update: %#v", fetched)
	}

	users, err := storage.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}

	if err := storage.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	if err := storage.DeleteUser(ctx, user.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRoomRepository(t *testing.T) {
	ctx := context.Background()
	storage := newTestStorage(t)

	now := time.Now().UTC().Truncate(time.Second)
	facilities := "Projector"
	room := persistence.Room{
		ID:         "room-1",
		Name:       "Conference Room",
		Location:   "Floor 1",
		Capacity:   10,
		Facilities: &facilities,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := storage.CreateRoom(ctx, room); err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	invalidRoom := room
	invalidRoom.ID = "room-invalid"
	invalidRoom.Capacity = 0
	if err := storage.CreateRoom(ctx, invalidRoom); !errors.Is(err, persistence.ErrConstraintViolation) {
		t.Fatalf("expected ErrConstraintViolation for invalid capacity, got %v", err)
	}

	fetched, err := storage.GetRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetRoom failed: %v", err)
	}
	if fetched.Name != room.Name || fetched.Location != room.Location {
		t.Fatalf("unexpected room data: %#v", fetched)
	}

	room.Name = "Updated Room"
	room.Capacity = 12
	room.UpdatedAt = now.Add(time.Minute)
	if err := storage.UpdateRoom(ctx, room); err != nil {
		t.Fatalf("UpdateRoom failed: %v", err)
	}

	rooms, err := storage.ListRooms(ctx)
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}
	if len(rooms) != 1 || rooms[0].Name != "Updated Room" {
		t.Fatalf("unexpected rooms: %#v", rooms)
	}

	if err := storage.DeleteRoom(ctx, room.ID); err != nil {
		t.Fatalf("DeleteRoom failed: %v", err)
	}

	if err := storage.DeleteRoom(ctx, room.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestWeekdayEncoding(t *testing.T) {
	t.Parallel()

	days := []time.Weekday{time.Monday, time.Wednesday, time.Friday}
	encoded := encodeWeekdays(days)
	decoded := decodeWeekdays(encoded)

	if len(decoded) != len(days) {
		t.Fatalf("expected %d decoded days, got %d", len(days), len(decoded))
	}

	for i, day := range days {
		if decoded[i] != day {
			t.Fatalf("expected day %v at index %d, got %v", day, i, decoded[i])
		}
	}

	if encodeWeekdays(nil) != 0 {
		t.Fatalf("expected zero encoding for nil slice")
	}
	if decoded := decodeWeekdays(0); len(decoded) != 0 {
		t.Fatalf("expected empty slice for zero encoding, got %v", decoded)
	}
}

func TestScheduleRepository(t *testing.T) {
	ctx := context.Background()
	storage := newTestStorage(t)

	now := time.Now().UTC().Truncate(time.Second)

	// Seed users required by foreign keys.
	creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
	attendee := persistence.User{ID: "attendee", Email: "attendee@example.com", DisplayName: "Attendee", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
	if err := storage.CreateUser(ctx, creator); err != nil {
		t.Fatalf("failed to seed creator: %v", err)
	}
	if err := storage.CreateUser(ctx, attendee); err != nil {
		t.Fatalf("failed to seed attendee: %v", err)
	}

	roomName := "Room A"
	room := persistence.Room{ID: "room-a", Name: roomName, Location: "Floor 2", Capacity: 6, CreatedAt: now, UpdatedAt: now}
	if err := storage.CreateRoom(ctx, room); err != nil {
		t.Fatalf("failed to seed room: %v", err)
	}

	start := now.Add(24 * time.Hour)
	end := start.Add(1 * time.Hour)
	memo := "Discuss roadmap"
	schedule := persistence.Schedule{
		ID:           "sched-1",
		Title:        "Roadmap",
		Start:        start,
		End:          end,
		CreatorID:    creator.ID,
		Memo:         &memo,
		Participants: []string{creator.ID, attendee.ID},
		RoomID:       &room.ID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := storage.CreateSchedule(ctx, schedule); err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}

	fetched, err := storage.GetSchedule(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}
	if len(fetched.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %#v", fetched.Participants)
	}

	// Update participants.
	schedule.Participants = []string{attendee.ID}
	schedule.Title = "Updated Meeting"
	schedule.UpdatedAt = now.Add(2 * time.Hour)
	if err := storage.UpdateSchedule(ctx, schedule); err != nil {
		t.Fatalf("UpdateSchedule failed: %v", err)
	}

	fetched, err = storage.GetSchedule(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}
	if fetched.Title != "Updated Meeting" || len(fetched.Participants) != 1 || fetched.Participants[0] != attendee.ID {
		t.Fatalf("unexpected schedule after update: %#v", fetched)
	}

	// Add second schedule to test filtering.
	schedule2 := persistence.Schedule{
		ID:           "sched-2",
		Title:        "Planning",
		Start:        start.Add(2 * time.Hour),
		End:          end.Add(2 * time.Hour),
		CreatorID:    creator.ID,
		Participants: []string{creator.ID},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := storage.CreateSchedule(ctx, schedule2); err != nil {
		t.Fatalf("CreateSchedule schedule2 failed: %v", err)
	}

	filterStart := start.Add(-time.Minute)
	filterEnd := end.Add(time.Minute)
	schedules, err := storage.ListSchedules(ctx, persistence.ScheduleFilter{
		ParticipantIDs: []string{attendee.ID},
		StartsAfter:    &filterStart,
		EndsBefore:     &filterEnd,
	})
	if err != nil {
		t.Fatalf("ListSchedules failed: %v", err)
	}
	if len(schedules) != 1 || schedules[0].ID != schedule.ID {
		t.Fatalf("expected only updated schedule, got %#v", schedules)
	}

	if err := storage.DeleteSchedule(ctx, schedule.ID); err != nil {
		t.Fatalf("DeleteSchedule failed: %v", err)
	}

	if _, err := storage.GetSchedule(ctx, schedule.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	invalid := schedule2
	invalid.ID = "sched-invalid"
	invalid.Start = invalid.End
	if err := storage.CreateSchedule(ctx, invalid); !errors.Is(err, persistence.ErrConstraintViolation) {
		t.Fatalf("expected ErrConstraintViolation for invalid times, got %v", err)
	}

	missingUser := schedule2
	missingUser.ID = "sched-missing"
	missingUser.CreatorID = "does-not-exist"
	missingUser.Participants = []string{}
	if err := storage.CreateSchedule(ctx, missingUser); !errors.Is(err, persistence.ErrForeignKeyViolation) {
		t.Fatalf("expected ErrForeignKeyViolation for missing creator, got %v", err)
	}
}

func TestStorage_normalizesDSNAndEnforcesForeignKeys(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	storage, err := Open("./scheduler.db")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	if err := storage.Migrate(ctx); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	absPath, err := normalizeDSN("./scheduler.db")
	if err != nil {
		t.Fatalf("normalizeDSN failed: %v", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		t.Fatalf("expected database file at %s: %v", absPath, err)
	}

	now := time.Now().UTC()
	invalidSchedule := persistence.Schedule{
		ID:        "fk-test",
		Title:     "Invalid",
		Start:     now,
		End:       now.Add(time.Hour),
		CreatorID: "missing",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := storage.CreateSchedule(ctx, invalidSchedule); !errors.Is(err, persistence.ErrForeignKeyViolation) {
		t.Fatalf("expected ErrForeignKeyViolation, got %v", err)
	}

}

func TestStorage_timestampsRoundTripRFC3339Nano(t *testing.T) {
	ctx := context.Background()
	storage := newTestStorage(t)

	precise := time.Date(2024, time.January, 2, 3, 4, 5, 987654321, time.UTC)
	user := persistence.User{
		ID:           "ts-user",
		Email:        "precision@example.com",
		DisplayName:  "Precision",
		PasswordHash: "hash",
		CreatedAt:    precise,
		UpdatedAt:    precise,
	}
	if err := storage.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	fetchedUser, err := storage.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if !fetchedUser.CreatedAt.Equal(precise) || !fetchedUser.UpdatedAt.Equal(precise) {
		t.Fatalf("expected timestamps to round-trip exactly, got %#v", fetchedUser)
	}

	start := precise.Add(2*time.Hour + 45*time.Minute)
	end := start.Add(time.Hour + time.Nanosecond)
	schedule := persistence.Schedule{
		ID:        "ts-schedule",
		Title:     "Precision Meeting",
		Start:     start,
		End:       end,
		CreatorID: user.ID,
		Participants: []string{
			user.ID,
		},
		CreatedAt: precise,
		UpdatedAt: precise,
	}
	if err := storage.CreateSchedule(ctx, schedule); err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
	fetchedSchedule, err := storage.GetSchedule(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}
	if !fetchedSchedule.Start.Equal(start) || !fetchedSchedule.End.Equal(end) {
		t.Fatalf("expected schedule times to round-trip, got %#v", fetchedSchedule)
	}
	if !fetchedSchedule.CreatedAt.Equal(precise) || !fetchedSchedule.UpdatedAt.Equal(precise) {
		t.Fatalf("expected metadata timestamps to round-trip, got %#v", fetchedSchedule)
	}
}

func TestStorage_weekdayBitmaskRoundTrip(t *testing.T) {
	ctx := context.Background()
	storage := newTestStorage(t)

	now := time.Now().UTC().Truncate(time.Second)
	user := persistence.User{ID: "weekday-user", Email: "weekday@example.com", DisplayName: "Weekday", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
	if err := storage.CreateUser(ctx, user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	schedule := persistence.Schedule{
		ID:           "weekday-schedule",
		Title:        "Weekly",
		Start:        now,
		End:          now.Add(time.Hour),
		CreatorID:    user.ID,
		Participants: []string{user.ID},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := storage.CreateSchedule(ctx, schedule); err != nil {
		t.Fatalf("failed to create schedule: %v", err)
	}

	rule := persistence.RecurrenceRule{
		ID:         "weekday-rule",
		ScheduleID: schedule.ID,
		Frequency:  1,
		Weekdays:   []time.Weekday{time.Monday, time.Wednesday, time.Friday},
		StartsOn:   now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := storage.UpsertRecurrence(ctx, rule); err != nil {
		t.Fatalf("UpsertRecurrence failed: %v", err)
	}
	rules, err := storage.ListRecurrencesForSchedule(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("ListRecurrencesForSchedule failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 recurrence, got %d", len(rules))
	}
	retrieved := rules[0]
	if len(retrieved.Weekdays) != 3 {
		t.Fatalf("expected 3 weekdays, got %#v", retrieved.Weekdays)
	}
	expected := []time.Weekday{time.Monday, time.Wednesday, time.Friday}
	for i, day := range expected {
		if retrieved.Weekdays[i] != day {
			t.Fatalf("weekday mismatch at %d: want %v got %v", i, day, retrieved.Weekdays[i])
		}
	}
}

func TestRecurrenceRepository(t *testing.T) {
	ctx := context.Background()
	storage := newTestStorage(t)

	now := time.Now().UTC().Truncate(time.Second)

	creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
	if err := storage.CreateUser(ctx, creator); err != nil {
		t.Fatalf("failed to seed creator: %v", err)
	}

	schedule := persistence.Schedule{
		ID:        "sched-rec",
		Title:     "Weekly Sync",
		Start:     now.Add(time.Hour),
		End:       now.Add(2 * time.Hour),
		CreatorID: creator.ID,
		Participants: []string{
			creator.ID,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := storage.CreateSchedule(ctx, schedule); err != nil {
		t.Fatalf("failed to create schedule: %v", err)
	}

	rule := persistence.RecurrenceRule{
		ID:         "recur-1",
		ScheduleID: schedule.ID,
		Frequency:  1,
		Weekdays:   []time.Weekday{time.Monday, time.Thursday},
		StartsOn:   now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := storage.UpsertRecurrence(ctx, rule); err != nil {
		t.Fatalf("UpsertRecurrence failed: %v", err)
	}

	rules, err := storage.ListRecurrencesForSchedule(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("ListRecurrencesForSchedule failed: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != rule.ID || len(rules[0].Weekdays) != 2 {
		t.Fatalf("unexpected recurrence rules: %#v", rules)
	}

	// Update rule.
	updatedEndsOn := now.Add(7 * 24 * time.Hour)
	rule.Weekdays = []time.Weekday{time.Wednesday}
	rule.EndsOn = &updatedEndsOn
	rule.UpdatedAt = now.Add(time.Minute)
	if err := storage.UpsertRecurrence(ctx, rule); err != nil {
		t.Fatalf("UpsertRecurrence update failed: %v", err)
	}

	rules, err = storage.ListRecurrencesForSchedule(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("ListRecurrencesForSchedule failed: %v", err)
	}
	if len(rules) != 1 || len(rules[0].Weekdays) != 1 || rules[0].Weekdays[0] != time.Wednesday {
		t.Fatalf("expected updated recurrence, got %#v", rules)
	}
	if rules[0].EndsOn == nil || !rules[0].EndsOn.Equal(updatedEndsOn) {
		t.Fatalf("expected EndsOn to be set: %#v", rules[0])
	}

	if err := storage.DeleteRecurrence(ctx, rule.ID); err != nil {
		t.Fatalf("DeleteRecurrence failed: %v", err)
	}

	if err := storage.DeleteRecurrence(ctx, rule.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on deleting missing recurrence, got %v", err)
	}

	if err := storage.UpsertRecurrence(ctx, rule); err != nil {
		t.Fatalf("UpsertRecurrence failed after delete: %v", err)
	}

	if err := storage.DeleteRecurrencesForSchedule(ctx, schedule.ID); err != nil {
		t.Fatalf("DeleteRecurrencesForSchedule failed: %v", err)
	}

	rules, err = storage.ListRecurrencesForSchedule(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("ListRecurrencesForSchedule failed: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected no recurrence rules after delete all, got %#v", rules)
	}
}

func TestSchemaIncludesSessionsTable(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller returned false")
	}

	path := filepath.Join(filepath.Dir(filename), "migrations", "0001_init.up.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read migration: %v", err)
	}

	contents := string(data)
	if !strings.Contains(contents, "CREATE TABLE IF NOT EXISTS sessions") {
		t.Fatalf("migration missing sessions table definition")
	}
	if !strings.Contains(contents, "idx_sessions_expires") {
		t.Fatalf("migration missing sessions expiry index")
	}
	if !strings.Contains(contents, "idx_sessions_user") {
		t.Fatalf("migration missing sessions user index")
	}
}

func TestMigrateCreatesStateFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "scheduler.db")
	storage, err := Open(dsn)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer func() {
		_ = storage.Close()
	}()

	statePath := storage.migrationStatePath()
	if _, err := os.Stat(statePath); err == nil {
		t.Fatalf("expected no migration state file before migrate")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected stat error: %v", err)
	}

	if err := storage.Migrate(ctx); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read migration state: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("migration state file is empty")
	}

	var state map[string]string
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("failed to decode state: %v", err)
	}
	if _, ok := state["0001"]; !ok {
		t.Fatalf("expected migration 0001 to be recorded, got %#v", state)
	}
}

// TestStorageMigrate_IntegrationWithMigrationSystem tests the integration of Storage.Migrate with the new migration system
func TestStorageMigrate_IntegrationWithMigrationSystem(t *testing.T) {
	ctx := context.Background()
	
	// Create a temporary directory for the test database
	dir := t.TempDir()
	dsn := filepath.Join(dir, "test_migration.db")
	
	// Open storage without calling Migrate yet
	storage, err := Open(dsn)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer storage.Close()
	
	// Test that migration runs successfully
	if err := storage.Migrate(ctx); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	
	// Verify that the database was created and has the expected schema
	// by attempting to create a user (which requires the users table to exist)
	now := time.Now().UTC().Truncate(time.Second)
	user := persistence.User{
		ID:           "test-user",
		Email:        "test@example.com",
		DisplayName:  "Test User",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	
	if err := storage.CreateUser(ctx, user); err != nil {
		t.Fatalf("failed to create user after migration: %v", err)
	}
	
	// Verify user was created successfully
	fetched, err := storage.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to fetch user after migration: %v", err)
	}
	
	if fetched.Email != user.Email {
		t.Fatalf("user data mismatch after migration: expected %s, got %s", user.Email, fetched.Email)
	}
}

// TestStorageMigrate_IdempotentExecution tests that running migrations multiple times is safe
func TestStorageMigrate_IdempotentExecution(t *testing.T) {
	ctx := context.Background()
	
	// Create a temporary directory for the test database
	dir := t.TempDir()
	dsn := filepath.Join(dir, "test_idempotent.db")
	
	// Open storage and run migration first time
	storage, err := Open(dsn)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer storage.Close()
	
	// First migration
	if err := storage.Migrate(ctx); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}
	
	// Create test data to ensure it persists (using the Storage interface)
	now := time.Now().UTC().Truncate(time.Second)
	user := persistence.User{
		ID:           "persistent-user",
		Email:        "persistent@example.com",
		DisplayName:  "Persistent User",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	
	if err := storage.CreateUser(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	
	// Second migration (should be idempotent - no new migrations should be applied)
	if err := storage.Migrate(ctx); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}
	
	// Verify data still exists (this tests that the migration didn't break anything)
	fetched, err := storage.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to fetch user after second migration: %v", err)
	}
	
	if fetched.Email != user.Email {
		t.Fatalf("user data lost after second migration: expected %s, got %s", user.Email, fetched.Email)
	}
	
	// Third migration (should still be safe)
	if err := storage.Migrate(ctx); err != nil {
		t.Fatalf("third migration failed: %v", err)
	}
	
	// Verify data still exists after third migration
	fetched, err = storage.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to fetch user after third migration: %v", err)
	}
	
	if fetched.Email != user.Email {
		t.Fatalf("user data lost after third migration: expected %s, got %s", user.Email, fetched.Email)
	}
}

// TestStorageMigrate_WithMissingMigrationDirectory tests behavior when migration directory doesn't exist
func TestStorageMigrate_WithMissingMigrationDirectory(t *testing.T) {
	ctx := context.Background()
	
	// Create a temporary directory for the test database
	dir := t.TempDir()
	dsn := filepath.Join(dir, "test_missing_dir.db")
	
	// Create a custom storage that will look for migrations in a non-existent location
	// We'll temporarily rename the migrations directory to simulate it being missing
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller returned false")
	}
	packageDir := filepath.Dir(filename)
	migrationsDir := filepath.Join(packageDir, "migrations")
	tempDir := filepath.Join(packageDir, "migrations_backup")
	
	// Rename migrations directory temporarily
	if err := os.Rename(migrationsDir, tempDir); err != nil {
		t.Fatalf("failed to rename migrations directory: %v", err)
	}
	defer func() {
		// Restore migrations directory
		os.Rename(tempDir, migrationsDir)
	}()
	
	// Open storage
	storage, err := Open(dsn)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer storage.Close()
	
	// Migration should fail due to missing directory
	if err := storage.Migrate(ctx); err == nil {
		t.Fatalf("expected migration to fail with missing directory, but it succeeded")
	} else if !strings.Contains(err.Error(), "migration directory does not exist") && 
	          !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("expected directory-related error, got: %v", err)
	}
}

// TestStorageMigrate_DatabaseConnectionFailure tests behavior when database connection fails
func TestStorageMigrate_DatabaseConnectionFailure(t *testing.T) {
	ctx := context.Background()
	
	// Try to open storage with an invalid path that should cause connection issues
	invalidPath := "/invalid/path/that/should/not/exist/test.db"
	
	storage, err := Open(invalidPath)
	if err != nil {
		// This is expected - the path is invalid
		return
	}
	defer storage.Close()
	
	// If Open succeeded (shouldn't happen), migration should fail
	if err := storage.Migrate(ctx); err == nil {
		t.Fatalf("expected migration to fail with invalid database path, but it succeeded")
	}
}
