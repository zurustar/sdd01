package persistence_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
	"github.com/example/enterprise-scheduler/internal/persistence/sqlite"
)

func TestUserRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates, reads, updates, and deletes users", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		user := persistence.User{
			ID:          "user-1",
			Email:       "alice@example.com",
			DisplayName: "Alice",
			IsAdmin:     true,
			CreatedAt:   time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt:   time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		}

		if err := harness.Users.CreateUser(ctx, user); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		fetched, err := harness.Users.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUser failed: %v", err)
		}
		if fetched.Email != user.Email || !fetched.IsAdmin {
			t.Fatalf("unexpected user data: %#v", fetched)
		}

		user.DisplayName = "Alice Updated"
		user.IsAdmin = false
		user.UpdatedAt = user.UpdatedAt.Add(time.Hour)
		if err := harness.Users.UpdateUser(ctx, user); err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		fetched, err = harness.Users.GetUserByEmail(ctx, "ALICE@EXAMPLE.COM")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}
		if fetched.DisplayName != "Alice Updated" || fetched.IsAdmin {
			t.Fatalf("unexpected updated user: %#v", fetched)
		}

		users, err := harness.Users.ListUsers(ctx)
		if err != nil {
			t.Fatalf("ListUsers failed: %v", err)
		}
		if len(users) != 1 || users[0].ID != user.ID {
			t.Fatalf("expected single user, got %#v", users)
		}

		if err := harness.Users.DeleteUser(ctx, user.ID); err != nil {
			t.Fatalf("DeleteUser failed: %v", err)
		}
		if err := harness.Users.DeleteUser(ctx, user.ID); !errors.Is(err, persistence.ErrNotFound) {
			t.Fatalf("expected persistence.ErrNotFound, got %v", err)
		}
	})

	t.Run("enforces unique email addresses", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		primary := persistence.User{ID: "user-1", Email: "duplicate@example.com", DisplayName: "Primary", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := harness.Users.CreateUser(ctx, primary); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		conflicting := persistence.User{ID: "user-2", Email: "duplicate@example.com", DisplayName: "Conflict", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := harness.Users.CreateUser(ctx, conflicting); !errors.Is(err, persistence.ErrDuplicate) {
			t.Fatalf("expected persistence.ErrDuplicate, got %v", err)
		}
	})

	t.Run("performs case-insensitive GetUserByEmail lookups", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		user := persistence.User{ID: "user-lookup", Email: "lookup@example.com", DisplayName: "Lookup", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := harness.Users.CreateUser(ctx, user); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		fetched, err := harness.Users.GetUserByEmail(ctx, "LOOKUP@EXAMPLE.COM")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}
		if fetched.ID != user.ID {
			t.Fatalf("expected %s, got %#v", user.ID, fetched)
		}
	})

	t.Run("returns users in deterministic order", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		base := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
		users := []persistence.User{
			{ID: "user-a", Email: "a@example.com", DisplayName: "A", CreatedAt: base, UpdatedAt: base},
			{ID: "user-c", Email: "c@example.com", DisplayName: "C", CreatedAt: base.Add(time.Minute), UpdatedAt: base.Add(time.Minute)},
			{ID: "user-b", Email: "b@example.com", DisplayName: "B", CreatedAt: base, UpdatedAt: base},
		}
		for _, u := range users {
			if err := harness.Users.CreateUser(ctx, u); err != nil {
				t.Fatalf("CreateUser(%s) failed: %v", u.ID, err)
			}
		}

		listed, err := harness.Users.ListUsers(ctx)
		if err != nil {
			t.Fatalf("ListUsers failed: %v", err)
		}
		order := []string{listed[0].ID, listed[1].ID, listed[2].ID}
		expected := []string{"user-a", "user-b", "user-c"}
		if !slices.Equal(order, expected) {
			t.Fatalf("unexpected order: got %v want %v", order, expected)
		}
	})
}

func TestRoomRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates, reads, updates, and deletes rooms", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		// Seed creator to allow schedule references.
		now := time.Now().UTC().Truncate(time.Second)
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}

		room := persistence.Room{ID: "room-1", Name: "会議室A", Location: "本社3F", Capacity: 8, CreatedAt: now, UpdatedAt: now}
		if err := harness.Rooms.CreateRoom(ctx, room); err != nil {
			t.Fatalf("CreateRoom failed: %v", err)
		}

		fetched, err := harness.Rooms.GetRoom(ctx, room.ID)
		if err != nil {
			t.Fatalf("GetRoom failed: %v", err)
		}
		if fetched.Name != room.Name {
			t.Fatalf("unexpected room: %#v", fetched)
		}

		room.Name = "会議室B"
		room.Capacity = 10
		room.UpdatedAt = room.UpdatedAt.Add(time.Hour)
		facilities := "Projector"
		room.Facilities = &facilities
		if err := harness.Rooms.UpdateRoom(ctx, room); err != nil {
			t.Fatalf("UpdateRoom failed: %v", err)
		}

		rooms, err := harness.Rooms.ListRooms(ctx)
		if err != nil {
			t.Fatalf("ListRooms failed: %v", err)
		}
		if len(rooms) != 1 || rooms[0].Name != "会議室B" {
			t.Fatalf("unexpected rooms: %#v", rooms)
		}

		schedule := persistence.Schedule{
			ID:        "schedule-room",
			Title:     "Room Meeting",
			CreatorID: creator.ID,
			Start:     now.Add(time.Hour),
			End:       now.Add(2 * time.Hour),
			Participants: []string{
				creator.ID,
			},
			RoomID:    &room.ID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := harness.Schedules.CreateSchedule(ctx, schedule); err != nil {
			t.Fatalf("CreateSchedule failed: %v", err)
		}

		if err := harness.Rooms.DeleteRoom(ctx, room.ID); err != nil {
			t.Fatalf("DeleteRoom failed: %v", err)
		}
		if err := harness.Rooms.DeleteRoom(ctx, room.ID); !errors.Is(err, persistence.ErrNotFound) {
			t.Fatalf("expected persistence.ErrNotFound, got %v", err)
		}

		updatedSchedule, err := harness.Schedules.GetSchedule(ctx, schedule.ID)
		if err != nil {
			t.Fatalf("GetSchedule after room delete failed: %v", err)
		}
		if updatedSchedule.RoomID != nil {
			t.Fatalf("expected room reference cleared, got %#v", updatedSchedule.RoomID)
		}
	})

	t.Run("rejects non-positive capacities", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		invalid := persistence.Room{ID: "invalid", Name: "小会議室", Location: "支社", Capacity: 0, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := harness.Rooms.CreateRoom(ctx, invalid); !errors.Is(err, persistence.ErrConstraintViolation) {
			t.Fatalf("expected persistence.ErrConstraintViolation, got %v", err)
		}
	})

	t.Run("returns rooms in deterministic order", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC()
		rooms := []persistence.Room{
			{ID: "room-b", Name: "会議室B", Location: "1F", Capacity: 6, CreatedAt: now, UpdatedAt: now},
			{ID: "room-a", Name: "会議室A", Location: "2F", Capacity: 4, CreatedAt: now, UpdatedAt: now},
			{ID: "room-a-2", Name: "会議室A", Location: "3F", Capacity: 10, CreatedAt: now, UpdatedAt: now},
		}
		for _, r := range rooms {
			if err := harness.Rooms.CreateRoom(ctx, r); err != nil {
				t.Fatalf("CreateRoom(%s) failed: %v", r.ID, err)
			}
		}

		listed, err := harness.Rooms.ListRooms(ctx)
		if err != nil {
			t.Fatalf("ListRooms failed: %v", err)
		}
		order := []string{listed[0].ID, listed[1].ID, listed[2].ID}
		expected := []string{"room-a", "room-a-2", "room-b"}
		if !slices.Equal(order, expected) {
			t.Fatalf("unexpected order: got %v want %v", order, expected)
		}
	})
}

func TestScheduleRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates schedules with participants", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC().Truncate(time.Second)
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
		attendee := persistence.User{ID: "attendee", Email: "attendee@example.com", DisplayName: "Attendee", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}
		if err := harness.Users.CreateUser(ctx, attendee); err != nil {
			t.Fatalf("failed to seed attendee: %v", err)
		}

		schedule := persistence.Schedule{
			ID:           "sched-1",
			Title:        "週次MTG",
			CreatorID:    creator.ID,
			Start:        now.Add(time.Hour),
			End:          now.Add(2 * time.Hour),
			Participants: []string{attendee.ID, creator.ID},
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := harness.Schedules.CreateSchedule(ctx, schedule); err != nil {
			t.Fatalf("CreateSchedule failed: %v", err)
		}

		fetched, err := harness.Schedules.GetSchedule(ctx, schedule.ID)
		if err != nil {
			t.Fatalf("GetSchedule failed: %v", err)
		}
		if !slices.Equal(fetched.Participants, []string{attendee.ID, creator.ID}) {
			t.Fatalf("unexpected participants: %#v", fetched.Participants)
		}
	})

	t.Run("filters schedules by participants and time range", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC().Truncate(time.Second)
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
		colleague := persistence.User{ID: "colleague", Email: "colleague@example.com", DisplayName: "Colleague", CreatedAt: now, UpdatedAt: now}
		outsider := persistence.User{ID: "outsider", Email: "outsider@example.com", DisplayName: "Outsider", CreatedAt: now, UpdatedAt: now}
		for _, u := range []persistence.User{creator, colleague, outsider} {
			if err := harness.Users.CreateUser(ctx, u); err != nil {
				t.Fatalf("failed to seed user %s: %v", u.ID, err)
			}
		}

		schedules := []persistence.Schedule{
			{
				ID:           "sched-a",
				Title:        "Planning",
				CreatorID:    creator.ID,
				Start:        now.Add(2 * time.Hour),
				End:          now.Add(3 * time.Hour),
				Participants: []string{creator.ID, colleague.ID},
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "sched-b",
				Title:        "1:1",
				CreatorID:    colleague.ID,
				Start:        now.Add(25 * time.Hour),
				End:          now.Add(26 * time.Hour),
				Participants: []string{colleague.ID},
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "sched-c",
				Title:        "External",
				CreatorID:    outsider.ID,
				Start:        now.Add(2 * time.Hour),
				End:          now.Add(3 * time.Hour),
				Participants: []string{outsider.ID},
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}
		for _, sched := range schedules {
			if err := harness.Schedules.CreateSchedule(ctx, sched); err != nil {
				t.Fatalf("CreateSchedule(%s) failed: %v", sched.ID, err)
			}
		}

		startsAfter := now
		endsBefore := now.Add(24 * time.Hour)
		filtered, err := harness.Schedules.ListSchedules(ctx, persistence.ScheduleFilter{
			ParticipantIDs: []string{colleague.ID},
			StartsAfter:    &startsAfter,
			EndsBefore:     &endsBefore,
		})
		if err != nil {
			t.Fatalf("ListSchedules failed: %v", err)
		}
		if len(filtered) != 1 || filtered[0].ID != "sched-a" {
			t.Fatalf("unexpected filtered schedules: %#v", filtered)
		}
	})

	t.Run("orders returned schedules deterministically", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC().Truncate(time.Second)
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}

		schedules := []persistence.Schedule{
			{ID: "sched-2", Title: "B", CreatorID: creator.ID, Start: now.Add(2 * time.Hour), End: now.Add(3 * time.Hour), Participants: []string{creator.ID}, CreatedAt: now, UpdatedAt: now},
			{ID: "sched-1", Title: "A", CreatorID: creator.ID, Start: now.Add(time.Hour), End: now.Add(2 * time.Hour), Participants: []string{creator.ID}, CreatedAt: now, UpdatedAt: now},
			{ID: "sched-3", Title: "C", CreatorID: creator.ID, Start: now.Add(time.Hour), End: now.Add(90 * time.Minute), Participants: []string{creator.ID}, CreatedAt: now, UpdatedAt: now},
		}
		for _, sched := range schedules {
			if err := harness.Schedules.CreateSchedule(ctx, sched); err != nil {
				t.Fatalf("CreateSchedule(%s) failed: %v", sched.ID, err)
			}
		}

		listed, err := harness.Schedules.ListSchedules(ctx, persistence.ScheduleFilter{})
		if err != nil {
			t.Fatalf("ListSchedules failed: %v", err)
		}
		order := []string{listed[0].ID, listed[1].ID, listed[2].ID}
		expected := []string{"sched-1", "sched-3", "sched-2"}
		if !slices.Equal(order, expected) {
			t.Fatalf("unexpected order: got %v want %v", order, expected)
		}
	})

	t.Run("rejects schedules where end is not after start", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC()
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}

		invalid := persistence.Schedule{ID: "invalid", CreatorID: creator.ID, Start: now, End: now, CreatedAt: now, UpdatedAt: now}
		if err := harness.Schedules.CreateSchedule(ctx, invalid); !errors.Is(err, persistence.ErrConstraintViolation) {
			t.Fatalf("expected persistence.ErrConstraintViolation, got %v", err)
		}
	})

	t.Run("deduplicates and sorts participant collections", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC()
		users := []persistence.User{
			{ID: "user-a", Email: "a@example.com", DisplayName: "A", CreatedAt: now, UpdatedAt: now},
			{ID: "user-b", Email: "b@example.com", DisplayName: "B", CreatedAt: now, UpdatedAt: now},
		}
		for _, u := range users {
			if err := harness.Users.CreateUser(ctx, u); err != nil {
				t.Fatalf("failed to seed user %s: %v", u.ID, err)
			}
		}

		schedule := persistence.Schedule{
			ID:           "sched-participants",
			Title:        "Participants",
			CreatorID:    "user-a",
			Start:        now.Add(time.Hour),
			End:          now.Add(2 * time.Hour),
			Participants: []string{"user-b", "user-b", "user-a"},
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := harness.Schedules.CreateSchedule(ctx, schedule); err != nil {
			t.Fatalf("CreateSchedule failed: %v", err)
		}

		fetched, err := harness.Schedules.GetSchedule(ctx, schedule.ID)
		if err != nil {
			t.Fatalf("GetSchedule failed: %v", err)
		}
		if !slices.Equal(fetched.Participants, []string{"user-a", "user-b"}) {
			t.Fatalf("expected sorted unique participants, got %#v", fetched.Participants)
		}
	})

	t.Run("cascades participant and recurrence cleanup on delete", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC()
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}

		schedule := persistence.Schedule{ID: "sched-delete", Title: "Delete", CreatorID: creator.ID, Start: now.Add(time.Hour), End: now.Add(2 * time.Hour), Participants: []string{creator.ID}, CreatedAt: now, UpdatedAt: now}
		if err := harness.Schedules.CreateSchedule(ctx, schedule); err != nil {
			t.Fatalf("CreateSchedule failed: %v", err)
		}

		rule := persistence.RecurrenceRule{ID: "recur-delete", ScheduleID: schedule.ID, Frequency: 1, Weekdays: []time.Weekday{time.Monday}, StartsOn: now.Truncate(24 * time.Hour), CreatedAt: now, UpdatedAt: now}
		if err := harness.Recurrences.UpsertRecurrence(ctx, rule); err != nil {
			t.Fatalf("UpsertRecurrence failed: %v", err)
		}

		if err := harness.Schedules.DeleteSchedule(ctx, schedule.ID); err != nil {
			t.Fatalf("DeleteSchedule failed: %v", err)
		}

		if _, err := harness.Schedules.GetSchedule(ctx, schedule.ID); !errors.Is(err, persistence.ErrNotFound) {
			t.Fatalf("expected persistence.ErrNotFound, got %v", err)
		}
		rules, err := harness.Recurrences.ListRecurrencesForSchedule(ctx, schedule.ID)
		if err != nil {
			t.Fatalf("ListRecurrencesForSchedule failed: %v", err)
		}
		if len(rules) != 0 {
			t.Fatalf("expected recurrences cleaned up, got %#v", rules)
		}
	})
}

func TestRecurrenceRepository(t *testing.T) {
	t.Parallel()

	t.Run("upserts recurrences preserving CreatedAt on update", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC().Truncate(time.Second)
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}
		schedule := persistence.Schedule{ID: "sched-recur", Title: "Recurring", CreatorID: creator.ID, Start: now.Add(time.Hour), End: now.Add(2 * time.Hour), Participants: []string{creator.ID}, CreatedAt: now, UpdatedAt: now}
		if err := harness.Schedules.CreateSchedule(ctx, schedule); err != nil {
			t.Fatalf("CreateSchedule failed: %v", err)
		}

		createdAt := now
		rule := persistence.RecurrenceRule{ID: "rule-1", ScheduleID: schedule.ID, Frequency: 1, Weekdays: []time.Weekday{time.Monday}, StartsOn: now.Truncate(24 * time.Hour), CreatedAt: createdAt, UpdatedAt: createdAt}
		if err := harness.Recurrences.UpsertRecurrence(ctx, rule); err != nil {
			t.Fatalf("UpsertRecurrence failed: %v", err)
		}

		updatedEnds := rule.StartsOn.Add(7 * 24 * time.Hour)
		rule.Weekdays = []time.Weekday{time.Wednesday}
		rule.EndsOn = &updatedEnds
		rule.UpdatedAt = rule.UpdatedAt.Add(time.Hour)
		if err := harness.Recurrences.UpsertRecurrence(ctx, rule); err != nil {
			t.Fatalf("UpsertRecurrence update failed: %v", err)
		}

		rules, err := harness.Recurrences.ListRecurrencesForSchedule(ctx, schedule.ID)
		if err != nil {
			t.Fatalf("ListRecurrencesForSchedule failed: %v", err)
		}
		if len(rules) != 1 {
			t.Fatalf("expected single rule, got %#v", rules)
		}
		got := rules[0]
		if !got.CreatedAt.Equal(createdAt) {
			t.Fatalf("expected CreatedAt to remain %v, got %v", createdAt, got.CreatedAt)
		}
		if got.EndsOn == nil || !got.EndsOn.Equal(updatedEnds) {
			t.Fatalf("expected EndsOn updated, got %#v", got.EndsOn)
		}
		if len(got.Weekdays) != 1 || got.Weekdays[0] != time.Wednesday {
			t.Fatalf("expected weekday Wednesday, got %#v", got.Weekdays)
		}
	})

	t.Run("lists recurrences for a schedule in creation order", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC()
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}
		schedule := persistence.Schedule{ID: "sched", Title: "Recurring", CreatorID: creator.ID, Start: now.Add(time.Hour), End: now.Add(2 * time.Hour), Participants: []string{creator.ID}, CreatedAt: now, UpdatedAt: now}
		if err := harness.Schedules.CreateSchedule(ctx, schedule); err != nil {
			t.Fatalf("CreateSchedule failed: %v", err)
		}

		rules := []persistence.RecurrenceRule{
			{ID: "rule-1", ScheduleID: schedule.ID, Frequency: 1, Weekdays: []time.Weekday{time.Monday}, StartsOn: now.Truncate(24 * time.Hour), CreatedAt: now, UpdatedAt: now},
			{ID: "rule-2", ScheduleID: schedule.ID, Frequency: 2, Weekdays: []time.Weekday{time.Tuesday}, StartsOn: now.Truncate(24 * time.Hour), CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute)},
		}
		for _, rule := range rules {
			if err := harness.Recurrences.UpsertRecurrence(ctx, rule); err != nil {
				t.Fatalf("UpsertRecurrence(%s) failed: %v", rule.ID, err)
			}
		}

		listed, err := harness.Recurrences.ListRecurrencesForSchedule(ctx, schedule.ID)
		if err != nil {
			t.Fatalf("ListRecurrencesForSchedule failed: %v", err)
		}
		order := []string{listed[0].ID, listed[1].ID}
		expected := []string{"rule-1", "rule-2"}
		if !slices.Equal(order, expected) {
			t.Fatalf("unexpected order: got %v want %v", order, expected)
		}
	})

	t.Run("rejects rules where EndsOn precedes StartsOn", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC()
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}
		schedule := persistence.Schedule{ID: "sched-invalid", Title: "Invalid", CreatorID: creator.ID, Start: now.Add(time.Hour), End: now.Add(2 * time.Hour), Participants: []string{creator.ID}, CreatedAt: now, UpdatedAt: now}
		if err := harness.Schedules.CreateSchedule(ctx, schedule); err != nil {
			t.Fatalf("CreateSchedule failed: %v", err)
		}

		startsOn := now.Truncate(24 * time.Hour)
		endsOn := startsOn.Add(-24 * time.Hour)
		rule := persistence.RecurrenceRule{ID: "invalid", ScheduleID: schedule.ID, Frequency: 1, Weekdays: []time.Weekday{time.Monday}, StartsOn: startsOn, EndsOn: &endsOn, CreatedAt: now, UpdatedAt: now}
		if err := harness.Recurrences.UpsertRecurrence(ctx, rule); !errors.Is(err, persistence.ErrConstraintViolation) {
			t.Fatalf("expected persistence.ErrConstraintViolation, got %v", err)
		}
	})
}

func TestSessionRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates and retrieves session tokens", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC().Truncate(time.Second)
		user := persistence.User{ID: "user-session", Email: "session@example.com", DisplayName: "Session", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, user); err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}

		session := persistence.Session{ID: "session-1", UserID: user.ID, Token: "token-1", Fingerprint: "fp", CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}
		if err := harness.Sessions.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		fetched, err := harness.Sessions.GetSession(ctx, session.Token)
		if err != nil {
			t.Fatalf("GetSession failed: %v", err)
		}
		if fetched.ID != session.ID || fetched.UserID != user.ID {
			t.Fatalf("unexpected session: %#v", fetched)
		}

		newToken := "token-2"
		revokedAt := now.Add(12 * time.Hour)
		session.Token = newToken
		session.Fingerprint = "fp-2"
		session.UpdatedAt = now.Add(6 * time.Hour)
		session.ExpiresAt = now.Add(48 * time.Hour)
		session.RevokedAt = &revokedAt
		if err := harness.Sessions.UpdateSession(ctx, session); err != nil {
			t.Fatalf("UpdateSession failed: %v", err)
		}

		updated, err := harness.Sessions.GetSession(ctx, newToken)
		if err != nil {
			t.Fatalf("GetSession after update failed: %v", err)
		}
		if updated.Token != newToken || updated.Fingerprint != "fp-2" {
			t.Fatalf("unexpected updated session: %#v", updated)
		}
		if updated.RevokedAt == nil || !updated.RevokedAt.Equal(revokedAt) {
			t.Fatalf("expected revoked timestamp, got %#v", updated.RevokedAt)
		}
	})

	t.Run("enforces foreign keys and unique tokens", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		now := time.Now().UTC()
		user := persistence.User{ID: "user", Email: "user@example.com", DisplayName: "User", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, user); err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}

		session := persistence.Session{ID: "session", UserID: user.ID, Token: "token", CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(time.Hour)}
		if err := harness.Sessions.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		duplicateToken := persistence.Session{ID: "other-session", UserID: user.ID, Token: "token", CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(time.Hour)}
		if err := harness.Sessions.CreateSession(ctx, duplicateToken); !errors.Is(err, persistence.ErrDuplicate) {
			t.Fatalf("expected persistence.ErrDuplicate, got %v", err)
		}

		foreign := persistence.Session{ID: "foreign", UserID: "missing", Token: "token-foreign", CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(time.Hour)}
		if err := harness.Sessions.CreateSession(ctx, foreign); !errors.Is(err, persistence.ErrForeignKeyViolation) {
			t.Fatalf("expected persistence.ErrForeignKeyViolation, got %v", err)
		}

		if err := harness.Sessions.UpdateSession(ctx, persistence.Session{ID: "missing", Token: "does-not-exist", UpdatedAt: now}); !errors.Is(err, persistence.ErrNotFound) {
			t.Fatalf("expected persistence.ErrNotFound on update, got %v", err)
		}
	})
}

type sqliteHarness struct {
	Users       persistence.UserRepository
	Rooms       persistence.RoomRepository
	Schedules   persistence.ScheduleRepository
	Recurrences persistence.RecurrenceRepository
	Sessions    persistence.SessionRepository
	Cleanup     func()
}

func newSQLiteHarness(t *testing.T) sqliteHarness {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "scheduler.db")

	storage, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}

	if err := storage.Migrate(context.Background()); err != nil {
		storage.Close()
		t.Fatalf("failed to migrate: %v", err)
	}

	return sqliteHarness{
		Users:       storage,
		Rooms:       storage,
		Schedules:   storage,
		Recurrences: storage,
		Sessions:    storage,
		Cleanup: func() {
			_ = storage.Close()
		},
	}
}
