package scheduler

import (
	"reflect"
	"testing"
	"time"
)

func TestDetectConflicts(t *testing.T) {
	t.Run("participant overlap produces conflict", func(t *testing.T) {
		existing := []Schedule{
			{
				ID:           "existing-1",
				Participants: []string{"alice", "bob"},
				Start:        mustParseTime(t, "2024-03-01T09:00:00+09:00"),
				End:          mustParseTime(t, "2024-03-01T10:00:00+09:00"),
			},
		}

		candidate := Schedule{
			ID:           "candidate",
			Participants: []string{"bob", "carol"},
			Start:        mustParseTime(t, "2024-03-01T09:30:00+09:00"),
			End:          mustParseTime(t, "2024-03-01T10:30:00+09:00"),
		}

		got := DetectConflicts(existing, candidate)
		expect := []Conflict{
			{
				WithScheduleID: "existing-1",
				Type:           ConflictTypeParticipant,
				Participant:    "bob",
			},
		}

		if !reflect.DeepEqual(got, expect) {
			t.Fatalf("expected participant conflict %#v, got %#v", expect, got)
		}
	})

	t.Run("room overlap produces conflict", func(t *testing.T) {
		roomID := "room-123"
		existing := []Schedule{
			{
				ID:     "existing-room",
				RoomID: &roomID,
				Start:  mustParseTime(t, "2024-03-01T11:00:00+09:00"),
				End:    mustParseTime(t, "2024-03-01T12:00:00+09:00"),
			},
		}

		candidate := Schedule{
			ID:     "candidate-room",
			RoomID: &roomID,
			Start:  mustParseTime(t, "2024-03-01T11:30:00+09:00"),
			End:    mustParseTime(t, "2024-03-01T12:30:00+09:00"),
		}

		got := DetectConflicts(existing, candidate)
		expectedRoomID := roomID
		expect := []Conflict{
			{
				WithScheduleID: "existing-room",
				Type:           ConflictTypeRoom,
				RoomID:         &expectedRoomID,
			},
		}

		if !reflect.DeepEqual(got, expect) {
			t.Fatalf("expected room conflict %#v, got %#v", expect, got)
		}
	})

	t.Run("non-overlapping schedules yield no conflicts", func(t *testing.T) {
		existing := []Schedule{
			{
				ID:           "existing-none",
				Participants: []string{"alice"},
				Start:        mustParseTime(t, "2024-03-01T08:00:00+09:00"),
				End:          mustParseTime(t, "2024-03-01T09:00:00+09:00"),
			},
		}

		candidate := Schedule{
			ID:           "candidate-none",
			Participants: []string{"alice"},
			Start:        mustParseTime(t, "2024-03-01T09:00:00+09:00"),
			End:          mustParseTime(t, "2024-03-01T10:00:00+09:00"),
		}

		if conflicts := DetectConflicts(existing, candidate); len(conflicts) != 0 {
			t.Fatalf("expected no conflicts, got %#v", conflicts)
		}
	})
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", value, err)
	}

	return parsed
}
