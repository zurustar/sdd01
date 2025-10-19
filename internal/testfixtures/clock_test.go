package testfixtures

import (
	"testing"
	"time"
)

func TestClockDefaultsToReferenceTime(t *testing.T) {
	clock := NewClock(time.Time{})
	if !clock.Now().Equal(ReferenceTime()) {
		t.Fatalf("expected ReferenceTime, got %v", clock.Now())
	}
}

func TestClockAdvanceAndSet(t *testing.T) {
	start := time.Date(2024, time.March, 14, 9, 26, 0, 0, time.UTC)
	clock := NewClock(start)

	updated := clock.Advance(90 * time.Minute)
	if !updated.Equal(start.Add(90 * time.Minute)) {
		t.Fatalf("advance returned %v", updated)
	}

	clock.Set(start.Add(2 * time.Hour))
	if got := clock.Current(); !got.Equal(start.Add(2 * time.Hour)) {
		t.Fatalf("expected %v, got %v", start.Add(2*time.Hour), got)
	}
}

func TestClockNowFunc(t *testing.T) {
	clock := NewClock(time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC))
	nowFn := clock.NowFunc()

	if got := nowFn(); !got.Equal(clock.Current()) {
		t.Fatalf("expected %v from NowFunc, got %v", clock.Current(), got)
	}

	clock.Advance(time.Minute)
	if got := nowFn(); !got.Equal(clock.Current()) {
		t.Fatalf("expected updated time %v, got %v", clock.Current(), got)
	}
}
