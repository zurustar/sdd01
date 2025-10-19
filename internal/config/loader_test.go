package config

import "testing"

func TestLoader_ParseEnvironment(t *testing.T) {
	t.Parallel()

	t.Run("applies defaults when variables are missing", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure loader falls back to defaults for optional variables")
	})

	t.Run("errors when required values are missing", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure loader reports missing HTTP port, SQLite DSN, and session secret")
	})

	t.Run("parses duration and numeric fields", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure loader parses typed settings like timeouts and capacities")
	})
}
