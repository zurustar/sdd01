package persistence

import "testing"

func TestUserRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates, reads, updates, and deletes users", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: exercise user repository CRUD against SQLite fixture")
	})

	t.Run("enforces unique email addresses", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect duplicate email to map to sentinel error")
	})

	t.Run("performs case-insensitive GetUserByEmail lookups", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure email queries are case-insensitive")
	})

	t.Run("returns users in deterministic order", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListUsers sorts results predictably")
	})
}

func TestRoomRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates, reads, updates, and deletes rooms", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: exercise room repository CRUD against SQLite fixture")
	})

	t.Run("rejects non-positive capacities", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure negative capacity triggers validation error")
	})

	t.Run("returns rooms in deterministic order", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListRooms sorts results predictably")
	})
}

func TestScheduleRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates schedules with participants and recurrences", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure participants persisted and retrieved with schedules")
	})

	t.Run("filters schedules by participants and time range", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert ListSchedules respects filter fields")
	})

	t.Run("orders returned schedules deterministically", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListSchedules sorts by start time then ID")
	})

	t.Run("rejects schedules where end is not after start", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect sentinel error when end precedes start")
	})

	t.Run("deduplicates and sorts participant collections", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure persisted participants are unique and ordered")
	})
}

func TestRecurrenceRepository(t *testing.T) {
	t.Parallel()

	t.Run("upserts recurrences preserving CreatedAt on update", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure UpsertRecurrence retains original CreatedAt")
	})

	t.Run("lists recurrences for a schedule in creation order", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListRecurrencesForSchedule orders by CreatedAt")
	})

	t.Run("rejects rules where EndsOn precedes StartsOn", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect validation error for invalid recurrence bounds")
	})
}

func TestSessionRepository(t *testing.T) {
	t.Parallel()

	t.Run("creates and retrieves session tokens", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure session repository stores and fetches tokens")
	})

	t.Run("expires and revokes sessions", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure expired or revoked sessions are not returned")
	})
}
