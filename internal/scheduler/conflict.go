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
	conflicts := make([]Conflict, 0)

	for _, sched := range existing {
		if !overlaps(sched, candidate) {
			continue
		}

		participantConflicts := detectParticipantConflicts(sched, candidate)
		conflicts = append(conflicts, participantConflicts...)

		if roomConflict := detectRoomConflict(sched, candidate); roomConflict != nil {
			conflicts = append(conflicts, *roomConflict)
		}
	}

	return conflicts
}

func overlaps(a, b Schedule) bool {
	return a.Start.Before(b.End) && b.Start.Before(a.End)
}

func detectParticipantConflicts(existing Schedule, candidate Schedule) []Conflict {
	if len(existing.Participants) == 0 || len(candidate.Participants) == 0 {
		return nil
	}

	existingSet := make(map[string]struct{}, len(existing.Participants))
	for _, p := range existing.Participants {
		existingSet[p] = struct{}{}
	}

	conflicts := make([]Conflict, 0)
	for _, p := range candidate.Participants {
		if _, ok := existingSet[p]; ok {
			conflicts = append(conflicts, Conflict{
				WithScheduleID: existing.ID,
				Type:           ConflictTypeParticipant,
				Participant:    p,
			})
		}
	}

	return conflicts
}

func detectRoomConflict(existing Schedule, candidate Schedule) *Conflict {
	if existing.RoomID == nil || candidate.RoomID == nil {
		return nil
	}

	if *existing.RoomID != *candidate.RoomID {
		return nil
	}

	roomID := *candidate.RoomID
	return &Conflict{
		WithScheduleID: existing.ID,
		Type:           ConflictTypeRoom,
		RoomID:         &roomID,
	}
}
