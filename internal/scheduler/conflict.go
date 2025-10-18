package scheduler

import "time"

// Schedule represents a scheduled event in the enterprise scheduler domain.
type Schedule struct {
	ID           string
	Participants []string
	RoomID       *string
	Start        time.Time
	End          time.Time
}

// ConflictType describes the type of conflict detected between schedules.
type ConflictType string

const (
	// ConflictTypeParticipant indicates a participant is double-booked.
	ConflictTypeParticipant ConflictType = "participant"
	// ConflictTypeRoom indicates a room is double-booked.
	ConflictTypeRoom ConflictType = "room"
)

// Conflict details an overlapping schedule relation that callers can present to users.
type Conflict struct {
	WithScheduleID string
	Type           ConflictType
	Participant    string
	RoomID         *string
}

// DetectConflicts identifies conflicts for the candidate schedule against existing ones.
func DetectConflicts(existing []Schedule, candidate Schedule) []Conflict {
	panic("TODO: implement DetectConflicts")
}
