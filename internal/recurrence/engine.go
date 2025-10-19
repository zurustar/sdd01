package recurrence

import (
	"errors"
	"time"
)

var jst = time.FixedZone("JST", 9*60*60)

// Frequency represents supported recurrence intervals.
type Frequency int

const (
	// FrequencyUnspecified indicates the rule frequency is not set.
	FrequencyUnspecified Frequency = iota
	// FrequencyDaily generates occurrences for each day within the range.
	FrequencyDaily
	// FrequencyWeekly generates occurrences for the selected weekdays.
	FrequencyWeekly
)

// Rule describes a recurrence configuration for a schedule.
type Rule struct {
	ID         string
	ScheduleID string
	Frequency  Frequency
	Weekdays   []time.Weekday
	StartsOn   time.Time
	EndsOn     *time.Time
}

// GenerateOptions defines optional range bounds for occurrence generation.
type GenerateOptions struct {
	RangeStart *time.Time
	RangeEnd   *time.Time
}

// Occurrence represents a generated instance of a recurrence rule.
type Occurrence struct {
	ScheduleID string
	RuleID     string
	Start      time.Time
	End        time.Time
}

// Engine expands recurrence rules into occurrences.
type Engine struct {
	location *time.Location
}

// NewEngine constructs an Engine that normalizes results to the provided location.
// If loc is nil, Asia/Tokyo (JST) is used.
func NewEngine(loc *time.Location) *Engine {
	if loc == nil {
		loc = jst
	}
	return &Engine{location: loc}
}

// ErrInvalidFrequency indicates the recurrence frequency is not supported.
var ErrInvalidFrequency = errors.New("recurrence: invalid frequency")

// ErrInvalidWindow indicates the generation window is unbounded.
var ErrInvalidWindow = errors.New("recurrence: generation window requires an end bound")

// ErrInvalidDuration indicates the base schedule duration is invalid.
var ErrInvalidDuration = errors.New("recurrence: schedule duration must be positive")

// GenerateOccurrences produces scheduled occurrences within the configured window.
//
// The engine enforces the following semantics:
//   - All timestamps are normalized to the engine's timezone (default JST).
//   - The generation window is bounded by the rule's EndsOn and the optional range end.
//   - Weekday selections are respected for weekly rules; daily rules may optionally
//     filter by weekdays when provided.
func (e *Engine) GenerateOccurrences(rule Rule, baseStart, baseEnd time.Time, opts GenerateOptions) ([]Occurrence, error) {
	loc := e.location
	if loc == nil {
		loc = jst
	}

	baseStart = baseStart.In(loc)
	baseEnd = baseEnd.In(loc)
	if !baseEnd.After(baseStart) {
		return nil, ErrInvalidDuration
	}
	duration := baseEnd.Sub(baseStart)

	ruleStart := rule.StartsOn.In(loc)
	var ruleEnd time.Time
	if rule.EndsOn != nil {
		ruleEnd = rule.EndsOn.In(loc)
	}

	var rangeStart time.Time
	if opts.RangeStart != nil {
		rangeStart = opts.RangeStart.In(loc)
	}
	var rangeEnd time.Time
	if opts.RangeEnd != nil {
		rangeEnd = opts.RangeEnd.In(loc)
	}

	// Determine the inclusive upper bound of the generation window.
	var upperBound time.Time
	hasUpper := false
	if !ruleEnd.IsZero() {
		upperBound = ruleEnd
		hasUpper = true
	}
	if !rangeEnd.IsZero() {
		if !hasUpper || rangeEnd.Before(upperBound) {
			upperBound = rangeEnd
		}
		hasUpper = true
	}
	if !hasUpper {
		return nil, ErrInvalidWindow
	}

	// Determine the lower bound from which to begin evaluation.
	lowerBound := ruleStart
	if !rangeStart.IsZero() && rangeStart.After(lowerBound) {
		lowerBound = rangeStart
	}
	if lowerBound.After(upperBound) {
		return nil, nil
	}

	weekdaySet := make(map[time.Weekday]struct{}, len(rule.Weekdays))
	for _, day := range rule.Weekdays {
		weekdaySet[day] = struct{}{}
	}

	current := firstCandidate(ruleStart, lowerBound, baseStart, loc)
	occurrences := make([]Occurrence, 0)

	for !current.After(upperBound) {
		include, err := shouldInclude(rule.Frequency, weekdaySet, current.Weekday())
		if err != nil {
			return nil, err
		}

		if include {
			occurrences = append(occurrences, Occurrence{
				ScheduleID: rule.ScheduleID,
				RuleID:     rule.ID,
				Start:      current,
				End:        current.Add(duration),
			})
		}

		current = current.Add(24 * time.Hour)
	}

	return occurrences, nil
}

func firstCandidate(ruleStart, lowerBound, template time.Time, loc *time.Location) time.Time {
	target := lowerBound
	if target.Before(ruleStart) {
		target = ruleStart
	}

	candidate := combineDateTime(target, template, loc)
	for candidate.Before(target) || candidate.Before(ruleStart) {
		candidate = candidate.Add(24 * time.Hour)
	}

	return candidate
}

func combineDateTime(dateSource, template time.Time, loc *time.Location) time.Time {
	y, m, d := dateSource.In(loc).Date()
	return time.Date(y, m, d, template.In(loc).Hour(), template.In(loc).Minute(), template.In(loc).Second(), template.In(loc).Nanosecond(), loc)
}

func shouldInclude(freq Frequency, weekdaySet map[time.Weekday]struct{}, day time.Weekday) (bool, error) {
	switch freq {
	case FrequencyDaily:
		if len(weekdaySet) == 0 {
			return true, nil
		}
		_, ok := weekdaySet[day]
		return ok, nil
	case FrequencyWeekly:
		if len(weekdaySet) == 0 {
			return false, nil
		}
		_, ok := weekdaySet[day]
		return ok, nil
	case FrequencyUnspecified:
		fallthrough
	default:
		return false, ErrInvalidFrequency
	}
}
