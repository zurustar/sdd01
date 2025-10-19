package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config captures environment driven configuration values for the scheduler service.
type Config struct {
	HTTPPort        int
	SQLiteDSN       string
	SessionSecret   string
	SessionTTL      time.Duration
	MaxRoomCapacity int
}

// Load parses configuration values from the current process environment.
//
// The loader applies sensible defaults for optional fields while validating
// required values and reporting localized error messages for missing entries.
func Load() (Config, error) {
	cfg := Config{
		HTTPPort:        8080,
		SQLiteDSN:       "file:scheduler.db?_foreign_keys=on",
		SessionTTL:      24 * time.Hour,
		MaxRoomCapacity: 0,
	}

	missing := make([]string, 0, 1)
	invalid := make([]string, 0, 2)

	if portValue := strings.TrimSpace(os.Getenv("SCHEDULER_HTTP_PORT")); portValue != "" {
		port, err := strconv.Atoi(portValue)
		if err != nil || port <= 0 {
			invalid = append(invalid, "SCHEDULER_HTTP_PORT")
		} else {
			cfg.HTTPPort = port
		}
	}

	if dsn := strings.TrimSpace(os.Getenv("SCHEDULER_SQLITE_DSN")); dsn != "" {
		cfg.SQLiteDSN = dsn
	}

	if secret := strings.TrimSpace(os.Getenv("SCHEDULER_SESSION_SECRET")); secret == "" {
		missing = append(missing, "SCHEDULER_SESSION_SECRET")
	} else {
		cfg.SessionSecret = secret
	}

	if ttlValue := strings.TrimSpace(os.Getenv("SCHEDULER_SESSION_TTL")); ttlValue != "" {
		ttl, err := time.ParseDuration(ttlValue)
		if err != nil || ttl <= 0 {
			invalid = append(invalid, "SCHEDULER_SESSION_TTL")
		} else {
			cfg.SessionTTL = ttl
		}
	}

	if capacityValue := strings.TrimSpace(os.Getenv("SCHEDULER_MAX_ROOM_CAPACITY")); capacityValue != "" {
		capacity, err := strconv.Atoi(capacityValue)
		if err != nil || capacity < 0 {
			invalid = append(invalid, "SCHEDULER_MAX_ROOM_CAPACITY")
		} else {
			cfg.MaxRoomCapacity = capacity
		}
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("必須の環境変数が設定されていません: %s", strings.Join(missing, ", "))
	}
	if len(invalid) > 0 {
		return Config{}, fmt.Errorf("環境変数の値が不正です: %s", strings.Join(invalid, ", "))
	}

	return cfg, nil
}
