package application

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// warningCache stores recently computed conflict warnings to avoid repeated
// detector execution for identical list queries while schedules remain
// unchanged.
type warningCache struct {
	mu         sync.RWMutex
	now        func() time.Time
	ttl        time.Duration
	maxEntries int
	entries    map[string]warningCacheEntry
}

type warningCacheEntry struct {
	warnings  []ConflictWarning
	expiresAt time.Time
}

func newWarningCache(ttl time.Duration, maxEntries int, now func() time.Time) *warningCache {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	if maxEntries <= 0 {
		maxEntries = 128
	}
	if now == nil {
		now = time.Now
	}
	return &warningCache{
		now:        now,
		ttl:        ttl,
		maxEntries: maxEntries,
		entries:    make(map[string]warningCacheEntry),
	}
}

func (c *warningCache) Get(key string) ([]ConflictWarning, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if c.now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}
	return cloneWarnings(entry.warnings), true
}

func (c *warningCache) Store(key string, warnings []ConflictWarning) {
	if c == nil {
		return
	}
	cloned := cloneWarnings(warnings)
	expiry := c.now().Add(c.ttl)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanupLocked()
	if len(c.entries) >= c.maxEntries {
		c.evictOneLocked()
	}
	c.entries[key] = warningCacheEntry{warnings: cloned, expiresAt: expiry}
}

func (c *warningCache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries = make(map[string]warningCacheEntry)
	c.mu.Unlock()
}

func (c *warningCache) cleanupLocked() {
	now := c.now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

func (c *warningCache) evictOneLocked() {
	for key := range c.entries {
		delete(c.entries, key)
		return
	}
}

func cloneWarnings(warnings []ConflictWarning) []ConflictWarning {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]ConflictWarning, len(warnings))
	copy(out, warnings)
	return out
}

func buildWarningCacheKey(params ListSchedulesParams, filter ScheduleRepositoryFilter) string {
	participantKey := strings.Join(filter.ParticipantIDs, ",")
	var startsAfter, endsBefore string
	if filter.StartsAfter != nil {
		startsAfter = filter.StartsAfter.UTC().Format(time.RFC3339Nano)
	}
	if filter.EndsBefore != nil {
		endsBefore = filter.EndsBefore.UTC().Format(time.RFC3339Nano)
	}
	participants := make([]string, len(params.ParticipantIDs))
	copy(participants, params.ParticipantIDs)
	sort.Strings(participants)

	builder := strings.Builder{}
	builder.WriteString(params.Principal.UserID)
	builder.WriteString("|")
	if params.Principal.IsAdmin {
		builder.WriteString("admin")
	}
	builder.WriteString("|")
	builder.WriteString(strings.Join(participants, ","))
	builder.WriteString("|")
	builder.WriteString(string(params.Period))
	builder.WriteString("|")
	builder.WriteString(params.PeriodReference.UTC().Format(time.RFC3339Nano))
	builder.WriteString("|")
	builder.WriteString(participantKey)
	builder.WriteString("|")
	builder.WriteString(startsAfter)
	builder.WriteString("|")
	builder.WriteString(endsBefore)
	return builder.String()
}
