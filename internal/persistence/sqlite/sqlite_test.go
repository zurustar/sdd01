package sqlite

import (
	"context"
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

	if err := storage.DeleteUser(ctx, user.ID); err != persistence.ErrNotFound {
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

	if err := storage.DeleteRoom(ctx, room.ID); err != persistence.ErrNotFound {
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

	if _, err := storage.GetSchedule(ctx, schedule.ID); err != persistence.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
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

	if err := storage.DeleteRecurrence(ctx, rule.ID); err != persistence.ErrNotFound {
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
