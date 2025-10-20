package recurrence

import (
	"testing"
	"time"
)

func BenchmarkEngineGenerateOccurrences(b *testing.B) {
	engine := NewEngine(nil)
	baseStart := time.Date(2024, 5, 6, 9, 0, 0, 0, jst)
	baseEnd := baseStart.Add(90 * time.Minute)

	until := baseStart.AddDate(0, 3, 0)
	rule := Rule{
		ID:         "rule-1",
		ScheduleID: "schedule-1",
		Frequency:  FrequencyWeekly,
		Weekdays: []time.Weekday{
			time.Monday,
			time.Tuesday,
			time.Wednesday,
			time.Thursday,
			time.Friday,
		},
		StartsOn: baseStart,
		EndsOn:   &until,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		occurrences, err := engine.GenerateOccurrences(rule, baseStart, baseEnd, GenerateOptions{})
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if len(occurrences) == 0 {
			b.Fatal("expected occurrences to be generated")
		}
	}
}
