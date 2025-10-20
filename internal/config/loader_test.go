package config

import (
	"os"
	"testing"
	"time"
)

func TestLoader_ParseEnvironment(t *testing.T) {

	t.Run("applies defaults when variables are missing", func(t *testing.T) {
		unset := []string{
			"SCHEDULER_HTTP_PORT",
			"SCHEDULER_SQLITE_DSN",
			"SCHEDULER_SESSION_TTL",
			"SCHEDULER_MAX_ROOM_CAPACITY",
		}
		for _, key := range unset {
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("failed to unset %s: %v", key, err)
			}
		}

		const secret = "super-secret"
		t.Setenv("SCHEDULER_SESSION_SECRET", secret)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}

		if cfg.HTTPPort != 8080 {
			t.Fatalf("expected default HTTP port 8080, got %d", cfg.HTTPPort)
		}
		if cfg.SQLiteDSN != "file:scheduler.db?_foreign_keys=on" {
			t.Fatalf("unexpected default DSN: %q", cfg.SQLiteDSN)
		}
		if cfg.SessionSecret != secret {
			t.Fatalf("expected session secret to be %q, got %q", secret, cfg.SessionSecret)
		}
	})

	t.Run("errors when required values are missing", func(t *testing.T) {
		for _, key := range []string{
			"SCHEDULER_SESSION_SECRET",
			"SCHEDULER_HTTP_PORT",
			"SCHEDULER_SQLITE_DSN",
		} {
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("failed to unset %s: %v", key, err)
			}
		}

		_, err := Load()
		if err == nil {
			t.Fatalf("expected error when required values are missing")
		}
		expected := "必須の環境変数が設定されていません: SCHEDULER_SESSION_SECRET"
		if err.Error() != expected {
			t.Fatalf("unexpected error message: %q", err.Error())
		}
	})

	t.Run("parses duration and numeric fields", func(t *testing.T) {
		t.Setenv("SCHEDULER_SESSION_SECRET", "secret-value")
		t.Setenv("SCHEDULER_HTTP_PORT", "9090")
		t.Setenv("SCHEDULER_SQLITE_DSN", "file:/tmp/scheduler.db")
		t.Setenv("SCHEDULER_SESSION_TTL", "24h")
		t.Setenv("SCHEDULER_MAX_ROOM_CAPACITY", "50")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}

		if cfg.SessionTTL != 24*time.Hour {
			t.Fatalf("expected session TTL 24h, got %s", cfg.SessionTTL)
		}
		if cfg.MaxRoomCapacity != 50 {
			t.Fatalf("expected max room capacity 50, got %d", cfg.MaxRoomCapacity)
		}
		if cfg.HTTPPort != 9090 {
			t.Fatalf("expected HTTP port 9090, got %d", cfg.HTTPPort)
		}
		if cfg.SQLiteDSN != "file:/tmp/scheduler.db" {
			t.Fatalf("unexpected DSN: %q", cfg.SQLiteDSN)
		}
	})
}
