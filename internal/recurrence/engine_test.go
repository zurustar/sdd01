package recurrence

import (
	"testing"
	"time"
)

func TestEngine_GenerateOccurrences(t *testing.T) {
	t.Parallel()

	baseStart := time.Date(2024, time.March, 4, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	baseEnd := baseStart.Add(1 * time.Hour)
	_ = baseEnd

	t.Run("respects weekday selections", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure recurrence generation honors requested weekdays")

		rule := struct {
			id         string
			scheduleID string
			startsOn   time.Time
			endsOn     time.Time
			weekdays   []time.Weekday
			frequency  string
		}{
			id:         "rule-1",
			scheduleID: "schedule-123",
			startsOn:   baseStart,
			endsOn:     baseStart.AddDate(0, 0, 14),
			weekdays:   []time.Weekday{time.Monday, time.Wednesday, time.Friday},
			frequency:  "weekly",
		}

		_ = rule

		// TODO: call Engine.GenerateOccurrences and assert only selected weekdays are produced chronologically
	})

	t.Run("clips occurrences to the requested period", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure generation stops at EndsOn or query boundary")

		opts := struct {
			rangeStart time.Time
			rangeEnd   time.Time
		}{
			rangeStart: baseStart.AddDate(0, 0, 3),
			rangeEnd:   baseStart.AddDate(0, 0, 10),
		}

		rule := struct {
			startsOn time.Time
			endsOn   time.Time
			weekdays []time.Weekday
			freq     string
		}{
			startsOn: baseStart,
			endsOn:   baseStart.AddDate(0, 0, 30),
			weekdays: []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
			freq:     "daily",
		}

		_ = rule
		_ = opts

		// TODO: assert first and last occurrences fall within requested range bounds
	})

	t.Run("handles timezone normalization", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure occurrences are generated in JST and converted correctly")

		jst := time.FixedZone("JST", 9*60*60)
		utc := time.UTC

		rule := struct {
			startsOn time.Time
			endsOn   time.Time
			weekdays []time.Weekday
			freq     string
		}{
			startsOn: baseStart.In(utc),
			endsOn:   baseStart.AddDate(0, 0, 7).In(utc),
			weekdays: []time.Weekday{time.Monday},
			freq:     "weekly",
		}

		_ = jst
		_ = rule

		// TODO: assert generated occurrences are normalized to JST regardless of input timezone
	})

	t.Run("links generated occurrences back to their source schedule", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure occurrences carry parent schedule identifiers")

		rule := struct {
			id         string
			scheduleID string
		}{id: "rule-4", scheduleID: "schedule-789"}

		_ = rule

		// TODO: assert every occurrence references parent schedule and rule identifiers for traceability
	})
}
