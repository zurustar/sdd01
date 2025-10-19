package testfixtures

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/example/enterprise-scheduler/internal/persistence"
	"github.com/example/enterprise-scheduler/internal/persistence/sqlite"
)

// SQLiteHarness provides repository access backed by a temporary SQLite storage
// instance for integration-style persistence tests.
type SQLiteHarness struct {
	Users       persistence.UserRepository
	Rooms       persistence.RoomRepository
	Schedules   persistence.ScheduleRepository
	Recurrences persistence.RecurrenceRepository
	Sessions    persistence.SessionRepository

	cleanup func()
}

// Close releases resources associated with the harness.
func (h *SQLiteHarness) Close() {
	if h != nil && h.cleanup != nil {
		h.cleanup()
		h.cleanup = nil
	}
}

// NewSQLiteHarness constructs a SQLiteHarness using a temporary file that is
// migrated automatically. Callers may optionally invoke Close, but the helper
// will also register a cleanup callback with the provided testing.TB.
func NewSQLiteHarness(tb testing.TB) *SQLiteHarness {
	tb.Helper()

	dir := tb.TempDir()
	path := filepath.Join(dir, "scheduler.db")

	storage, err := sqlite.Open(path)
	if err != nil {
		tb.Fatalf("failed to open storage: %v", err)
	}

	if err := storage.Migrate(context.Background()); err != nil {
		_ = storage.Close()
		tb.Fatalf("failed to migrate storage: %v", err)
	}

	harness := &SQLiteHarness{
		Users:       storage,
		Rooms:       storage,
		Schedules:   storage,
		Recurrences: storage,
		Sessions:    storage,
		cleanup: func() {
			_ = storage.Close()
		},
	}

	tb.Cleanup(harness.Close)
	return harness
}
