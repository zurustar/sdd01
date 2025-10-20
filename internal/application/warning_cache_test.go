package application

import (
	"testing"
	"time"
)

func TestWarningCacheStoresAndReturnsCopies(t *testing.T) {
	fixed := time.Date(2024, 5, 1, 9, 0, 0, 0, time.UTC)
	current := fixed
	cache := newWarningCache(time.Minute, 4, func() time.Time { return current })

	original := []ConflictWarning{{ScheduleID: "schedule-1", Type: "participant"}}
	cache.Store("key", original)

	// Mutating the original slice should not affect the cached copy.
	original[0].ScheduleID = "mutated"

	cached, ok := cache.Get("key")
	if !ok {
		t.Fatalf("expected cache hit")
	}
	if cached[0].ScheduleID != "schedule-1" {
		t.Fatalf("expected cached schedule id to remain unchanged, got %s", cached[0].ScheduleID)
	}

	// Mutating the returned slice should not be visible on subsequent reads.
	cached[0].ScheduleID = "changed"
	cachedAgain, ok := cache.Get("key")
	if !ok {
		t.Fatalf("expected cache hit on second read")
	}
	if cachedAgain[0].ScheduleID != "schedule-1" {
		t.Fatalf("expected cache to return independent copy, got %s", cachedAgain[0].ScheduleID)
	}
}

func TestWarningCacheExpiresEntries(t *testing.T) {
	fixed := time.Date(2024, 5, 1, 9, 0, 0, 0, time.UTC)
	current := fixed
	cache := newWarningCache(time.Second, 4, func() time.Time { return current })

	cache.Store("key", []ConflictWarning{{ScheduleID: "schedule-1"}})
	if _, ok := cache.Get("key"); !ok {
		t.Fatalf("expected cache hit before expiry")
	}

	current = current.Add(2 * time.Second)
	if _, ok := cache.Get("key"); ok {
		t.Fatalf("expected cache entry to expire")
	}
}

func TestWarningCacheInvalidate(t *testing.T) {
	cache := newWarningCache(time.Minute, 4, time.Now)
	cache.Store("key", []ConflictWarning{{ScheduleID: "schedule-1"}})
	cache.Invalidate()
	if _, ok := cache.Get("key"); ok {
		t.Fatalf("expected cache to be empty after invalidation")
	}
}
