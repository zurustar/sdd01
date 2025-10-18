package scheduler

import "testing"

func TestDetectConflicts(t *testing.T) {
	t.Run("participant overlap produces conflict", func(t *testing.T) {
		t.Fatalf("TODO: set up participant overlap fixture and expected conflicts")
	})

	t.Run("room overlap produces conflict", func(t *testing.T) {
		t.Fatalf("TODO: set up room overlap fixture and expected conflicts")
	})

	t.Run("non-overlapping schedules yield no conflicts", func(t *testing.T) {
		t.Fatalf("TODO: set up non-overlapping schedules and assert no conflicts")
	})
}
