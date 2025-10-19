package recurrence

import (
	"testing"
	"time"
)

func TestEngine_GenerateOccurrences(t *testing.T) {
	t.Parallel()

	baseStart := time.Date(2024, time.March, 4, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	baseEnd := baseStart.Add(1 * time.Hour)

	t.Run("respects weekday selections", func(t *testing.T) {
		t.Parallel()

		engine := NewEngine(nil)
		rule := Rule{
			ID:         "rule-1",
			ScheduleID: "schedule-123",
			Frequency:  FrequencyWeekly,
			Weekdays:   []time.Weekday{time.Monday, time.Wednesday, time.Friday},
			StartsOn:   baseStart,
			EndsOn: func() *time.Time {
				end := baseStart.AddDate(0, 0, 14)
				return &end
			}(),
		}

		occurrences, err := engine.GenerateOccurrences(rule, baseStart, baseEnd, GenerateOptions{})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		weekdays := make([]time.Weekday, 0, len(occurrences))
		last := time.Time{}
		for _, occurrence := range occurrences {
			if occurrence.Start.Before(last) {
				t.Fatalf("expected chronological ordering, got %v before %v", occurrence.Start, last)
			}
			if occurrence.RuleID != rule.ID {
				t.Fatalf("expected occurrence to reference rule %q, got %q", rule.ID, occurrence.RuleID)
			}
			weekdays = append(weekdays, occurrence.Start.Weekday())
			last = occurrence.Start
		}

		expected := []time.Weekday{
			time.Monday,
			time.Wednesday,
			time.Friday,
			time.Monday,
			time.Wednesday,
			time.Friday,
			time.Monday,
		}
		if len(weekdays) != len(expected) {
			t.Fatalf("expected %d occurrences, got %d (%v)", len(expected), len(weekdays), weekdays)
		}
		for i := range weekdays {
			if weekdays[i] != expected[i] {
				t.Fatalf("unexpected weekday at %d: want %v got %v", i, expected[i], weekdays[i])
			}
		}
	})

	t.Run("clips occurrences to the requested period", func(t *testing.T) {
		t.Parallel()

		engine := NewEngine(nil)
		rangeStart := baseStart.AddDate(0, 0, 3)
		rangeEnd := baseStart.AddDate(0, 0, 10)

		rule := Rule{
			ID:         "rule-2",
			ScheduleID: "schedule-456",
			Frequency:  FrequencyDaily,
			Weekdays:   []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
			StartsOn:   baseStart,
			EndsOn: func() *time.Time {
				end := baseStart.AddDate(0, 0, 30)
				return &end
			}(),
		}

		occurrences, err := engine.GenerateOccurrences(rule, baseStart, baseEnd, GenerateOptions{
			RangeStart: &rangeStart,
			RangeEnd:   &rangeEnd,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(occurrences) == 0 {
			t.Fatalf("expected occurrences within range")
		}

		if first := occurrences[0].Start; first.Before(rangeStart) {
			t.Fatalf("expected first occurrence >= range start %v, got %v", rangeStart, first)
		}
		if last := occurrences[len(occurrences)-1].Start; last.After(rangeEnd) {
			t.Fatalf("expected last occurrence <= range end %v, got %v", rangeEnd, last)
		}
	})

	t.Run("handles timezone normalization", func(t *testing.T) {
		t.Parallel()

		engine := NewEngine(nil)
		baseStartUTC := baseStart.In(time.UTC)
		baseEndUTC := baseEnd.In(time.UTC)

		rule := Rule{
			ID:         "rule-3",
			ScheduleID: "schedule-789",
			Frequency:  FrequencyWeekly,
			Weekdays:   []time.Weekday{time.Monday},
			StartsOn:   baseStartUTC,
			EndsOn: func() *time.Time {
				end := baseStartUTC.AddDate(0, 0, 7)
				return &end
			}(),
		}

		occurrences, err := engine.GenerateOccurrences(rule, baseStartUTC, baseEndUTC, GenerateOptions{})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(occurrences) == 0 {
			t.Fatalf("expected at least one occurrence")
		}

		for _, occurrence := range occurrences {
			if occurrence.Start.Location().String() != "JST" {
				t.Fatalf("expected occurrence start in JST, got %v", occurrence.Start.Location())
			}
			if occurrence.End.Location().String() != "JST" {
				t.Fatalf("expected occurrence end in JST, got %v", occurrence.End.Location())
			}
		}
	})

	t.Run("links generated occurrences back to their source schedule", func(t *testing.T) {
		t.Parallel()

		engine := NewEngine(nil)
		rule := Rule{
			ID:         "rule-4",
			ScheduleID: "schedule-789",
			Frequency:  FrequencyWeekly,
			Weekdays:   []time.Weekday{time.Monday},
			StartsOn:   baseStart,
			EndsOn: func() *time.Time {
				end := baseStart.AddDate(0, 0, 7)
				return &end
			}(),
		}

		occurrences, err := engine.GenerateOccurrences(rule, baseStart, baseEnd, GenerateOptions{})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		for _, occurrence := range occurrences {
			if occurrence.ScheduleID != rule.ScheduleID {
				t.Fatalf("expected schedule ID %q, got %q", rule.ScheduleID, occurrence.ScheduleID)
			}
			if occurrence.RuleID != rule.ID {
				t.Fatalf("expected rule ID %q, got %q", rule.ID, occurrence.RuleID)
			}
		}
	})
}
