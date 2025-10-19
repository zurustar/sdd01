package persistence

import "time"

// User represents an employee account in the scheduler domain.
type User struct {
	ID          string
	Email       string
	DisplayName string
	IsAdmin     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Room represents a meeting room catalog entry.
type Room struct {
	ID         string
	Name       string
	Location   string
	Capacity   int
	Facilities *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Schedule represents a calendar entry stored in persistence.
type Schedule struct {
	ID               string
	Title            string
	Start            time.Time
	End              time.Time
	CreatorID        string
	Memo             *string
	Participants     []string
	RoomID           *string
	WebConferenceURL *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// RecurrenceRule represents a weekly recurrence configuration for a schedule.
type RecurrenceRule struct {
	ID         string
	ScheduleID string
	Frequency  int
	Weekdays   []time.Weekday
	StartsOn   time.Time
	EndsOn     *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Session represents an authentication session persisted for a user.
type Session struct {
	ID          string
	UserID      string
	Token       string
	Fingerprint string
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	RevokedAt   *time.Time
}
