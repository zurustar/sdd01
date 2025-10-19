package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestUserRepository(t *testing.T) {
	ctx := context.Background()
	storage := newTestStorage(t)

	now := time.Now().UTC().Truncate(time.Second)
	user := persistence.User{
		ID:          "user-1",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		IsAdmin:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
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

	if fetched.Email != user.Email || !fetched.IsAdmin {
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
	if fetched.DisplayName != "Alice Updated" || fetched.IsAdmin {
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

func TestScheduleRepository(t *testing.T) {
	ctx := context.Background()
	storage := newTestStorage(t)

	now := time.Now().UTC().Truncate(time.Second)

	// Seed users required by foreign keys.
	creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
	attendee := persistence.User{ID: "attendee", Email: "attendee@example.com", DisplayName: "Attendee", CreatedAt: now, UpdatedAt: now}
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
		ID:          "ts-user",
		Email:       "precision@example.com",
		DisplayName: "Precision",
		CreatedAt:   precise,
		UpdatedAt:   precise,
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
	user := persistence.User{ID: "weekday-user", Email: "weekday@example.com", DisplayName: "Weekday", CreatedAt: now, UpdatedAt: now}
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

	creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
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
