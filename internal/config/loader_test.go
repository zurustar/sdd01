package config

import (
	"os"
	"testing"
	"time"
)

func TestLoader_ParseEnvironment(t *testing.T) {
	t.Parallel()

	t.Run("applies defaults when variables are missing", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure loader falls back to defaults for optional variables")

		unset := []string{"SCHEDULER_HTTP_PORT", "SCHEDULER_SQLITE_DSN"}
		for _, key := range unset {
			os.Unsetenv(key)
		}

		_ = unset

		// TODO: assert loader returns default port 8080 and derived DSN path when env is empty
	})

	t.Run("errors when required values are missing", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure loader reports missing HTTP port, SQLite DSN, and session secret")

		// TODO: call loader with empty env map and ensure error aggregates missing variables for Japanese localization
	})

	t.Run("parses duration and numeric fields", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure loader parses typed settings like timeouts and capacities")

		os.Setenv("SCHEDULER_SESSION_TTL", "24h")
		os.Setenv("SCHEDULER_MAX_ROOM_CAPACITY", "50")
		defer func() {
			os.Unsetenv("SCHEDULER_SESSION_TTL")
			os.Unsetenv("SCHEDULER_MAX_ROOM_CAPACITY")
		}()

		ttl := 24 * time.Hour
		capacity := 50

		_ = ttl
		_ = capacity

		// TODO: assert loader returns Config with typed fields parsed accordingly
	})
}
