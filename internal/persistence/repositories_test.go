package persistence_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
	"github.com/example/enterprise-scheduler/internal/testfixtures"
)

func newPersistenceUser(opts ...testfixtures.UserOption) persistence.User {
	return testfixtures.NewUserFixture(opts...).Persistence()
}

func newPersistenceRoom(opts ...testfixtures.RoomOption) persistence.Room {
	return testfixtures.NewRoomFixture(opts...).Persistence()
}

func newPersistenceSchedule(opts ...testfixtures.ScheduleOption) persistence.Schedule {
	return testfixtures.NewScheduleFixture(opts...).Persistence()
}

func newPersistenceRecurrence(opts ...testfixtures.RecurrenceOption) persistence.RecurrenceRule {
	return testfixtures.NewRecurrenceFixture(opts...).Persistence()
}

func newPersistenceSession(opts ...testfixtures.SessionOption) persistence.Session {
	return testfixtures.NewSessionFixture(opts...).Persistence()
}

func TestUserRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates, reads, updates, and deletes users", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		base := testfixtures.ReferenceTime()
		user := newPersistenceUser(
			testfixtures.WithUserID("user-1"),
			testfixtures.WithUserEmail("alice@example.com"),
			testfixtures.WithUserDisplayName("Alice"),
			testfixtures.WithUserPasswordHash("hash"),
			testfixtures.WithUserAdmin(true),
			testfixtures.WithUserTimestamps(base, base),
		)

		if err := harness.Users.CreateUser(ctx, user); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		fetched, err := harness.Users.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUser failed: %v", err)
		}
		if fetched.Email != user.Email || !fetched.IsAdmin || fetched.PasswordHash != user.PasswordHash {
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
		if fetched.DisplayName != "Alice Updated" || fetched.IsAdmin || fetched.PasswordHash != user.PasswordHash {
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := testfixtures.ReferenceTime()
		primary := newPersistenceUser(
			testfixtures.WithUserID("user-1"),
			testfixtures.WithUserEmail("duplicate@example.com"),
			testfixtures.WithUserDisplayName("Primary"),
			testfixtures.WithUserPasswordHash("hash"),
			testfixtures.WithUserTimestamps(now, now),
		)
		if err := harness.Users.CreateUser(ctx, primary); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		conflicting := newPersistenceUser(
			testfixtures.WithUserID("user-2"),
			testfixtures.WithUserEmail("duplicate@example.com"),
			testfixtures.WithUserDisplayName("Conflict"),
			testfixtures.WithUserPasswordHash("hash"),
			testfixtures.WithUserTimestamps(now.Add(time.Minute), now.Add(time.Minute)),
		)
		if err := harness.Users.CreateUser(ctx, conflicting); !errors.Is(err, persistence.ErrDuplicate) {
			t.Fatalf("expected persistence.ErrDuplicate, got %v", err)
		}
	})

	t.Run("performs case-insensitive GetUserByEmail lookups", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := testfixtures.ReferenceTime().Add(2 * time.Minute)
		user := newPersistenceUser(
			testfixtures.WithUserID("user-lookup"),
			testfixtures.WithUserEmail("lookup@example.com"),
			testfixtures.WithUserDisplayName("Lookup"),
			testfixtures.WithUserPasswordHash("hash"),
			testfixtures.WithUserTimestamps(now, now),
		)
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		base := testfixtures.ReferenceTime()
		users := []persistence.User{
			newPersistenceUser(
				testfixtures.WithUserID("user-a"),
				testfixtures.WithUserEmail("a@example.com"),
				testfixtures.WithUserDisplayName("A"),
				testfixtures.WithUserPasswordHash("hash"),
				testfixtures.WithUserTimestamps(base, base),
			),
			newPersistenceUser(
				testfixtures.WithUserID("user-c"),
				testfixtures.WithUserEmail("c@example.com"),
				testfixtures.WithUserDisplayName("C"),
				testfixtures.WithUserPasswordHash("hash"),
				testfixtures.WithUserTimestamps(base.Add(time.Minute), base.Add(time.Minute)),
			),
			newPersistenceUser(
				testfixtures.WithUserID("user-b"),
				testfixtures.WithUserEmail("b@example.com"),
				testfixtures.WithUserDisplayName("B"),
				testfixtures.WithUserPasswordHash("hash"),
				testfixtures.WithUserTimestamps(base, base),
			),
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		// Seed creator to allow schedule references.
		now := testfixtures.ReferenceTime().Truncate(time.Second)
		creator := newPersistenceUser(
			testfixtures.WithUserID("creator"),
			testfixtures.WithUserEmail("creator@example.com"),
			testfixtures.WithUserDisplayName("Creator"),
			testfixtures.WithUserPasswordHash("hash"),
			testfixtures.WithUserTimestamps(now, now),
		)
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}

		room := newPersistenceRoom(
			testfixtures.WithRoomID("room-1"),
			testfixtures.WithRoomName("会議室A"),
			testfixtures.WithRoomLocation("本社3F"),
			testfixtures.WithRoomCapacity(8),
			testfixtures.WithRoomTimestamps(now, now),
		)
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

		schedule := newPersistenceSchedule(
			testfixtures.WithScheduleID("schedule-room"),
			testfixtures.WithScheduleTitle("Room Meeting"),
			testfixtures.WithScheduleCreator(creator.ID),
			testfixtures.WithScheduleStartEnd(now.Add(time.Hour), now.Add(2*time.Hour)),
			testfixtures.WithScheduleParticipants(creator.ID),
			testfixtures.WithScheduleRoomID(room.ID),
			testfixtures.WithScheduleTimestamps(now, now),
		)
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		invalid := persistence.Room{ID: "invalid", Name: "小会議室", Location: "支社", Capacity: 0, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := harness.Rooms.CreateRoom(ctx, invalid); !errors.Is(err, persistence.ErrConstraintViolation) {
			t.Fatalf("expected persistence.ErrConstraintViolation, got %v", err)
		}
	})

	t.Run("returns rooms in deterministic order", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := time.Now().UTC().Truncate(time.Second)
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
		attendee := persistence.User{ID: "attendee", Email: "attendee@example.com", DisplayName: "Attendee", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := time.Now().UTC().Truncate(time.Second)
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
		colleague := persistence.User{ID: "colleague", Email: "colleague@example.com", DisplayName: "Colleague", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
		outsider := persistence.User{ID: "outsider", Email: "outsider@example.com", DisplayName: "Outsider", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
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

	t.Run("includes schedules created by the filtered participant even when they are not listed as attendees", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := time.Now().UTC().Truncate(time.Second)
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
		colleague := persistence.User{ID: "colleague", Email: "colleague@example.com", DisplayName: "Colleague", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
		stranger := persistence.User{ID: "stranger", Email: "stranger@example.com", DisplayName: "Stranger", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
		for _, u := range []persistence.User{creator, colleague, stranger} {
			if err := harness.Users.CreateUser(ctx, u); err != nil {
				t.Fatalf("failed to seed user %s: %v", u.ID, err)
			}
		}

		schedules := []persistence.Schedule{
			{
				ID:           "sched-owned",
				Title:        "Owned",
				CreatorID:    creator.ID,
				Start:        now.Add(2 * time.Hour),
				End:          now.Add(3 * time.Hour),
				Participants: []string{colleague.ID},
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "sched-outside",
				Title:        "Outside window",
				CreatorID:    creator.ID,
				Start:        now.Add(-4 * time.Hour),
				End:          now.Add(-3 * time.Hour),
				Participants: []string{colleague.ID},
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "sched-other",
				Title:        "Other owner",
				CreatorID:    stranger.ID,
				Start:        now.Add(2 * time.Hour),
				End:          now.Add(3 * time.Hour),
				Participants: []string{stranger.ID},
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}
		for _, sched := range schedules {
			if err := harness.Schedules.CreateSchedule(ctx, sched); err != nil {
				t.Fatalf("CreateSchedule(%s) failed: %v", sched.ID, err)
			}
		}

		startsAfter := now.Add(-time.Minute)
		endsBefore := now.Add(4 * time.Hour)
		filtered, err := harness.Schedules.ListSchedules(ctx, persistence.ScheduleFilter{
			ParticipantIDs: []string{creator.ID},
			StartsAfter:    &startsAfter,
			EndsBefore:     &endsBefore,
		})
		if err != nil {
			t.Fatalf("ListSchedules failed: %v", err)
		}

		if len(filtered) != 1 || filtered[0].ID != "sched-owned" {
			t.Fatalf("expected creator-owned schedule to be returned, got %#v", filtered)
		}
	})

	t.Run("supports multi-user views by merging participant matches and ordering chronologically", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := time.Now().UTC().Truncate(time.Second)
		users := []persistence.User{
			{ID: "principal", Email: "principal@example.com", DisplayName: "Principal", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now},
			{ID: "colleague-a", Email: "a@example.com", DisplayName: "A", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now},
			{ID: "colleague-b", Email: "b@example.com", DisplayName: "B", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now},
			{ID: "other", Email: "other@example.com", DisplayName: "Other", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now},
		}
		for _, u := range users {
			if err := harness.Users.CreateUser(ctx, u); err != nil {
				t.Fatalf("failed to seed user %s: %v", u.ID, err)
			}
		}

		schedules := []persistence.Schedule{
			{ID: "sched-a", Title: "Principal", CreatorID: users[0].ID, Start: now.Add(time.Hour), End: now.Add(2 * time.Hour), Participants: []string{users[0].ID}, CreatedAt: now, UpdatedAt: now},
			{ID: "sched-b", Title: "Colleague A", CreatorID: users[1].ID, Start: now.Add(30 * time.Minute), End: now.Add(90 * time.Minute), Participants: []string{users[1].ID}, CreatedAt: now, UpdatedAt: now},
			{ID: "sched-c", Title: "Colleague B", CreatorID: users[2].ID, Start: now.Add(3 * time.Hour), End: now.Add(4 * time.Hour), Participants: []string{users[2].ID}, CreatedAt: now, UpdatedAt: now},
			{ID: "sched-d", Title: "Other", CreatorID: users[3].ID, Start: now.Add(time.Hour), End: now.Add(2 * time.Hour), Participants: []string{users[3].ID}, CreatedAt: now, UpdatedAt: now},
		}
		for _, sched := range schedules {
			if err := harness.Schedules.CreateSchedule(ctx, sched); err != nil {
				t.Fatalf("CreateSchedule(%s) failed: %v", sched.ID, err)
			}
		}

		filterParticipants := []string{users[0].ID, users[1].ID, users[2].ID}
		filtered, err := harness.Schedules.ListSchedules(ctx, persistence.ScheduleFilter{ParticipantIDs: filterParticipants})
		if err != nil {
			t.Fatalf("ListSchedules failed: %v", err)
		}

		ids := []string{}
		for _, sched := range filtered {
			ids = append(ids, sched.ID)
		}
		expected := []string{"sched-b", "sched-a", "sched-c"}
		if !slices.Equal(ids, expected) {
			t.Fatalf("expected schedules %v, got %v", expected, ids)
		}
	})

	t.Run("orders returned schedules deterministically", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := time.Now().UTC().Truncate(time.Second)
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := testfixtures.ReferenceTime()
		creator := newPersistenceUser(
			testfixtures.WithUserID("creator"),
			testfixtures.WithUserEmail("creator@example.com"),
			testfixtures.WithUserDisplayName("Creator"),
			testfixtures.WithUserPasswordHash("hash"),
			testfixtures.WithUserTimestamps(now, now),
		)
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := testfixtures.ReferenceTime()
		users := []persistence.User{
			newPersistenceUser(
				testfixtures.WithUserID("user-a"),
				testfixtures.WithUserEmail("a@example.com"),
				testfixtures.WithUserDisplayName("A"),
				testfixtures.WithUserPasswordHash("hash"),
				testfixtures.WithUserTimestamps(now, now),
			),
			newPersistenceUser(
				testfixtures.WithUserID("user-b"),
				testfixtures.WithUserEmail("b@example.com"),
				testfixtures.WithUserDisplayName("B"),
				testfixtures.WithUserPasswordHash("hash"),
				testfixtures.WithUserTimestamps(now, now),
			),
		}
		for _, u := range users {
			if err := harness.Users.CreateUser(ctx, u); err != nil {
				t.Fatalf("failed to seed user %s: %v", u.ID, err)
			}
		}

		schedule := newPersistenceSchedule(
			testfixtures.WithScheduleID("sched-participants"),
			testfixtures.WithScheduleTitle("Participants"),
			testfixtures.WithScheduleCreator("user-a"),
			testfixtures.WithScheduleStartEnd(now.Add(time.Hour), now.Add(2*time.Hour)),
			testfixtures.WithScheduleParticipants("user-b", "user-b", "user-a"),
			testfixtures.WithScheduleTimestamps(now, now),
		)
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := time.Now().UTC()
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}

		schedule := newPersistenceSchedule(
			testfixtures.WithScheduleID("sched-delete"),
			testfixtures.WithScheduleTitle("Delete"),
			testfixtures.WithScheduleCreator(creator.ID),
			testfixtures.WithScheduleStartEnd(now.Add(time.Hour), now.Add(2*time.Hour)),
			testfixtures.WithScheduleParticipants(creator.ID),
			testfixtures.WithScheduleTimestamps(now, now),
		)
		if err := harness.Schedules.CreateSchedule(ctx, schedule); err != nil {
			t.Fatalf("CreateSchedule failed: %v", err)
		}

		rule := newPersistenceRecurrence(
			testfixtures.WithRecurrenceID("recur-delete"),
			testfixtures.WithRecurrenceScheduleID(schedule.ID),
			testfixtures.WithRecurrenceStartsOn(now.Truncate(24*time.Hour)),
			testfixtures.WithRecurrenceTimestamps(now, now),
		)
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := time.Now().UTC().Truncate(time.Second)
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := time.Now().UTC()
		creator := persistence.User{ID: "creator", Email: "creator@example.com", DisplayName: "Creator", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}
		schedule := persistence.Schedule{ID: "sched", Title: "Recurring", CreatorID: creator.ID, Start: now.Add(time.Hour), End: now.Add(2 * time.Hour), Participants: []string{creator.ID}, CreatedAt: now, UpdatedAt: now}
		if err := harness.Schedules.CreateSchedule(ctx, schedule); err != nil {
			t.Fatalf("CreateSchedule failed: %v", err)
		}

		rules := []persistence.RecurrenceRule{
			newPersistenceRecurrence(
				testfixtures.WithRecurrenceID("rule-1"),
				testfixtures.WithRecurrenceScheduleID(schedule.ID),
				testfixtures.WithRecurrenceFrequency(1),
				testfixtures.WithRecurrenceWeekdays(time.Monday),
				testfixtures.WithRecurrenceStartsOn(now.Truncate(24*time.Hour)),
				testfixtures.WithRecurrenceTimestamps(now, now),
			),
			newPersistenceRecurrence(
				testfixtures.WithRecurrenceID("rule-2"),
				testfixtures.WithRecurrenceScheduleID(schedule.ID),
				testfixtures.WithRecurrenceFrequency(2),
				testfixtures.WithRecurrenceWeekdays(time.Tuesday),
				testfixtures.WithRecurrenceStartsOn(now.Truncate(24*time.Hour)),
				testfixtures.WithRecurrenceTimestamps(now.Add(time.Minute), now.Add(time.Minute)),
			),
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := testfixtures.ReferenceTime()
		creator := newPersistenceUser(
			testfixtures.WithUserID("creator"),
			testfixtures.WithUserEmail("creator@example.com"),
			testfixtures.WithUserDisplayName("Creator"),
			testfixtures.WithUserPasswordHash("hash"),
			testfixtures.WithUserTimestamps(now, now),
		)
		if err := harness.Users.CreateUser(ctx, creator); err != nil {
			t.Fatalf("failed to seed creator: %v", err)
		}
		schedule := newPersistenceSchedule(
			testfixtures.WithScheduleID("sched-invalid"),
			testfixtures.WithScheduleTitle("Invalid"),
			testfixtures.WithScheduleCreator(creator.ID),
			testfixtures.WithScheduleStartEnd(now.Add(time.Hour), now.Add(2*time.Hour)),
			testfixtures.WithScheduleParticipants(creator.ID),
			testfixtures.WithScheduleTimestamps(now, now),
		)
		if err := harness.Schedules.CreateSchedule(ctx, schedule); err != nil {
			t.Fatalf("CreateSchedule failed: %v", err)
		}

		startsOn := now.Truncate(24 * time.Hour)
		endsOn := startsOn.Add(-24 * time.Hour)
		rule := newPersistenceRecurrence(
			testfixtures.WithRecurrenceID("invalid"),
			testfixtures.WithRecurrenceScheduleID(schedule.ID),
			testfixtures.WithRecurrenceFrequency(1),
			testfixtures.WithRecurrenceWeekdays(time.Monday),
			testfixtures.WithRecurrenceStartsOn(startsOn),
			testfixtures.WithRecurrenceEndsOn(endsOn),
			testfixtures.WithRecurrenceTimestamps(now, now),
		)
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
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := testfixtures.ReferenceTime().Truncate(time.Second)
		user := newPersistenceUser(
			testfixtures.WithUserID("user-session"),
			testfixtures.WithUserEmail("session@example.com"),
			testfixtures.WithUserDisplayName("Session"),
			testfixtures.WithUserPasswordHash("hash"),
			testfixtures.WithUserTimestamps(now, now),
		)
		if err := harness.Users.CreateUser(ctx, user); err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}

		session := newPersistenceSession(
			testfixtures.WithSessionID("session-1"),
			testfixtures.WithSessionUserID(user.ID),
			testfixtures.WithSessionToken("token-1"),
			testfixtures.WithSessionFingerprint("fp"),
			testfixtures.WithSessionTimestamps(now, now),
			testfixtures.WithSessionExpiresAt(now.Add(24*time.Hour)),
		)
		created, err := harness.Sessions.CreateSession(ctx, session)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		if created.Token != session.Token || created.ExpiresAt.IsZero() {
			t.Fatalf("unexpected created session: %#v", created)
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
		updatedSession, err := harness.Sessions.UpdateSession(ctx, session)
		if err != nil {
			t.Fatalf("UpdateSession failed: %v", err)
		}
		if updatedSession.Token != newToken || updatedSession.Fingerprint != "fp-2" {
			t.Fatalf("unexpected updated clone: %#v", updatedSession)
		}

		updated, err := harness.Sessions.GetSession(ctx, newToken)
		if err != nil {
			t.Fatalf("GetSession after update failed: %v", err)
		}
		if updated.Token != newToken || updated.Fingerprint != "fp-2" {
			t.Fatalf("unexpected updated session: %#v", updated)
		}

		revoked, err := harness.Sessions.RevokeSession(ctx, newToken, revokedAt)
		if err != nil {
			t.Fatalf("RevokeSession failed: %v", err)
		}
		if revoked.RevokedAt == nil || !revoked.RevokedAt.Equal(revokedAt) {
			t.Fatalf("expected revoked timestamp, got %#v", revoked.RevokedAt)
		}

		prunedAt := now.Add(72 * time.Hour)
		if err := harness.Sessions.DeleteExpiredSessions(ctx, prunedAt); err != nil {
			t.Fatalf("DeleteExpiredSessions failed: %v", err)
		}
		if _, err := harness.Sessions.GetSession(ctx, newToken); !errors.Is(err, persistence.ErrNotFound) {
			t.Fatalf("expected ErrNotFound after pruning, got %v", err)
		}
	})

	t.Run("enforces foreign keys and unique tokens", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		harness := testfixtures.NewSQLiteHarness(t)
		defer harness.Close()

		now := testfixtures.ReferenceTime()
		user := newPersistenceUser(
			testfixtures.WithUserID("user"),
			testfixtures.WithUserEmail("user@example.com"),
			testfixtures.WithUserDisplayName("User"),
			testfixtures.WithUserPasswordHash("hash"),
			testfixtures.WithUserTimestamps(now, now),
		)
		if err := harness.Users.CreateUser(ctx, user); err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}

		session := newPersistenceSession(
			testfixtures.WithSessionID("session"),
			testfixtures.WithSessionUserID(user.ID),
			testfixtures.WithSessionToken("token"),
			testfixtures.WithSessionTimestamps(now, now),
			testfixtures.WithSessionExpiresAt(now.Add(time.Hour)),
		)
		if _, err := harness.Sessions.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		duplicateToken := newPersistenceSession(
			testfixtures.WithSessionID("other-session"),
			testfixtures.WithSessionUserID(user.ID),
			testfixtures.WithSessionToken("token"),
			testfixtures.WithSessionTimestamps(now, now),
			testfixtures.WithSessionExpiresAt(now.Add(time.Hour)),
		)
		if _, err := harness.Sessions.CreateSession(ctx, duplicateToken); !errors.Is(err, persistence.ErrDuplicate) {
			t.Fatalf("expected persistence.ErrDuplicate, got %v", err)
		}

		foreign := newPersistenceSession(
			testfixtures.WithSessionID("foreign"),
			testfixtures.WithSessionUserID("missing"),
			testfixtures.WithSessionToken("token-foreign"),
			testfixtures.WithSessionTimestamps(now, now),
			testfixtures.WithSessionExpiresAt(now.Add(time.Hour)),
		)
		if _, err := harness.Sessions.CreateSession(ctx, foreign); !errors.Is(err, persistence.ErrForeignKeyViolation) {
			t.Fatalf("expected persistence.ErrForeignKeyViolation, got %v", err)
		}

		if _, err := harness.Sessions.UpdateSession(ctx, newPersistenceSession(testfixtures.WithSessionID("missing"), testfixtures.WithSessionToken("does-not-exist"), testfixtures.WithSessionUpdatedAt(now))); !errors.Is(err, persistence.ErrNotFound) {
			t.Fatalf("expected persistence.ErrNotFound on update, got %v", err)
		}

		if _, err := harness.Sessions.RevokeSession(ctx, "unknown", now); !errors.Is(err, persistence.ErrNotFound) {
			t.Fatalf("expected persistence.ErrNotFound on revoke, got %v", err)
		}
	})
}
