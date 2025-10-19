package persistence

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestUserRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates, reads, updates, and deletes users", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: exercise user repository CRUD against SQLite fixture")

		ctx := context.Background()
		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		user := User{
			ID:          "user-1",
			Email:       "alice@example.com",
			DisplayName: "Alice",
			IsAdmin:     true,
			CreatedAt:   time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt:   time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		}

		_ = ctx
		_ = harness.Users
		_ = user

		// TODO: persist user, fetch by ID and email, update fields, delete, and ensure cascading cleanup of schedule participants
	})

        t.Run("enforces unique email addresses", func(t *testing.T) {
                t.Parallel()
                t.Skip("TODO: expect duplicate email to map to sentinel error")

                harness := newSQLiteHarness(t)
                defer harness.Cleanup()

                conflictingUser := User{ID: "user-2", Email: "alice@example.com"}
                expected := errors.New("persistence: duplicate email")

                _ = harness.Users
                _ = conflictingUser
                _ = expected

                // TODO: insert duplicate email and assert ErrDuplicateEmail (to be introduced) is returned
        })

	t.Run("performs case-insensitive GetUserByEmail lookups", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure email queries are case-insensitive")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		cases := []struct {
			lookup string
			wantID string
		}{
			{lookup: "ALICE@EXAMPLE.COM", wantID: "user-1"},
			{lookup: "alice@example.com", wantID: "user-1"},
		}

		_ = cases
		_ = harness.Users

		// TODO: ensure repository lowercases/normalizes email before querying
	})

	t.Run("returns users in deterministic order", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListUsers sorts results predictably")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		// TODO: insert multiple users and assert ListUsers orders by created_at then id
		_ = harness.Users
	})
}

func TestRoomRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates, reads, updates, and deletes rooms", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: exercise room repository CRUD against SQLite fixture")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		room := Room{
			ID:        "room-1",
			Name:      "会議室A",
			Location:  "本社3F",
			Capacity:  8,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		_ = harness.Rooms
		_ = room

		// TODO: verify CRUD flow and that deleting room cascades to schedules referencing it
	})

	t.Run("rejects non-positive capacities", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure negative capacity triggers validation error")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		invalid := Room{ID: "room-2", Name: "小会議室", Capacity: 0}

		_ = harness.Rooms
		_ = invalid

		// TODO: expect ErrInvalidRoomCapacity sentinel when capacity <= 0
	})

	t.Run("returns rooms in deterministic order", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListRooms sorts results predictably")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		// TODO: insert rooms and assert ordering by name then id for stable catalogs
		_ = harness.Rooms
	})
}

func TestScheduleRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates schedules with participants and recurrences", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure participants persisted and retrieved with schedules")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		schedule := Schedule{
			ID:           "sched-1",
			Title:        "週次MTG",
			CreatorID:    "user-1",
			Start:        time.Date(2024, 4, 1, 10, 0, 0, 0, time.UTC),
			End:          time.Date(2024, 4, 1, 11, 0, 0, 0, time.UTC),
			Participants: []string{"user-1", "user-2"},
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}

		recurrence := RecurrenceRule{ID: "recur-1", ScheduleID: schedule.ID}

		_ = harness.Schedules
		_ = harness.Recurrences
		_ = schedule
		_ = recurrence

		// TODO: persist schedule and recurrence, ensure participants and rules are hydrated on fetch
	})

	t.Run("filters schedules by participants and time range", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert ListSchedules respects filter fields")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		filter := ScheduleFilter{
			ParticipantIDs: []string{"user-1", "user-3"},
		}
		startsAfter := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
		endsBefore := startsAfter.Add(7 * 24 * time.Hour)
		filter.StartsAfter = &startsAfter
		filter.EndsBefore = &endsBefore

		_ = harness.Schedules
		_ = filter

		// TODO: assert returned schedules fall within range and include only requested participants
	})

	t.Run("orders returned schedules deterministically", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListSchedules sorts by start time then ID")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		// TODO: seed overlapping schedules and assert deterministic ordering by start asc, id asc
		_ = harness.Schedules
	})

	t.Run("rejects schedules where end is not after start", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect sentinel error when end precedes start")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		invalid := Schedule{
			ID:        "sched-invalid",
			Start:     time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC),
			End:       time.Date(2024, 5, 1, 11, 0, 0, 0, time.UTC),
			CreatorID: "user-1",
		}

		_ = harness.Schedules
		_ = invalid

		// TODO: ensure repository surfaces ErrInvalidScheduleTime when End <= Start
	})

	t.Run("deduplicates and sorts participant collections", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure persisted participants are unique and ordered")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		schedule := Schedule{
			ID:           "sched-duplicate",
			CreatorID:    "user-1",
			Start:        time.Now().UTC(),
			End:          time.Now().UTC().Add(time.Hour),
			Participants: []string{"user-2", "user-2", "user-1"},
		}

		_ = harness.Schedules
		_ = schedule

		// TODO: ensure participant join table collapses duplicates and returns sorted slice
	})

	t.Run("cascades participant and recurrence cleanup on delete", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure deleting a schedule removes associated participants and recurrences")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		scheduleID := "sched-delete"

		_ = harness.Schedules
		_ = harness.Recurrences
		_ = scheduleID

		// TODO: delete schedule and assert lookup of participants/recurrences returns ErrNotFound
	})
}

func TestRecurrenceRepository(t *testing.T) {
	t.Parallel()

	t.Run("upserts recurrences preserving CreatedAt on update", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure UpsertRecurrence retains original CreatedAt")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		createdAt := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
		rule := RecurrenceRule{ID: "recur-keep-created", CreatedAt: createdAt}

		_ = harness.Recurrences
		_ = rule

		// TODO: perform upsert twice and ensure CreatedAt remains createdAt while UpdatedAt changes
	})

	t.Run("lists recurrences for a schedule in creation order", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListRecurrencesForSchedule orders by CreatedAt")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		scheduleID := "sched-1"

		_ = harness.Recurrences
		_ = scheduleID

		// TODO: seed multiple rules and assert ordering matches insertion chronology
	})

	t.Run("rejects rules where EndsOn precedes StartsOn", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect validation error for invalid recurrence bounds")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		startsOn := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		endsOn := startsOn.Add(-24 * time.Hour)
		rule := RecurrenceRule{ID: "recur-invalid", StartsOn: startsOn, EndsOn: &endsOn}

		_ = harness.Recurrences
		_ = rule

		// TODO: expect ErrInvalidRecurrenceBounds sentinel
	})
}

func TestSessionRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates and retrieves session tokens", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure session repository stores and fetches tokens")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		session := struct {
			Token     string
			UserID    string
			ExpiresAt time.Time
		}{Token: "token-1", UserID: "user-1", ExpiresAt: time.Now().Add(24 * time.Hour)}

		_ = harness.Sessions
		_ = session

		// TODO: insert session and verify retrieval by token returns matching user and expiry
	})

	t.Run("expires and revokes sessions", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure expired or revoked sessions are not returned")

		harness := newSQLiteHarness(t)
		defer harness.Cleanup()

		revokeToken := "token-revoke"
		expiredToken := "token-expired"

		_ = harness.Sessions
		_ = revokeToken
		_ = expiredToken

		// TODO: mark one session revoked and another expired; ensure lookups return ErrNotFound
	})
}

type sqliteHarness struct {
	Users       UserRepository
	Rooms       RoomRepository
	Schedules   ScheduleRepository
	Recurrences RecurrenceRepository
	Sessions    any
	Cleanup     func()
}

func newSQLiteHarness(t *testing.T) sqliteHarness {
	t.Helper()

	// TODO: open temporary SQLite database, run migrations, and return repositories
	return sqliteHarness{
		Cleanup: func() {},
	}
}
