package application

import "time"

// Principal represents the authenticated user invoking a service method.
type Principal struct {
	UserID  string
	IsAdmin bool
}

// ScheduleInput captures caller provided schedule fields.
type ScheduleInput struct {
	CreatorID        string
	Title            string
	Description      string
	Start            time.Time
	End              time.Time
	RoomID           *string
	WebConferenceURL string
	ParticipantIDs   []string
}

// Schedule represents a persisted meeting schedule.
type Schedule struct {
	ID               string
	CreatorID        string
	Title            string
	Description      string
	Start            time.Time
	End              time.Time
	RoomID           *string
	WebConferenceURL string
	ParticipantIDs   []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// CreateScheduleParams wraps the data required to create a schedule.
type CreateScheduleParams struct {
	Principal Principal
	Input     ScheduleInput
}

// UpdateScheduleParams wraps the data required to update an existing schedule.
type UpdateScheduleParams struct {
	Principal  Principal
	ScheduleID string
	Input      ScheduleInput
}
