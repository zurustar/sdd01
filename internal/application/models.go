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
	Occurrences      []ScheduleOccurrence
}

// ScheduleOccurrence represents an expanded occurrence generated from a recurrence rule.
type ScheduleOccurrence struct {
	ScheduleID string
	RuleID     string
	Start      time.Time
	End        time.Time
}

// ConflictWarning describes a scheduling conflict that should be surfaced to callers.
type ConflictWarning struct {
	ScheduleID    string
	Type          string
	ParticipantID string
	RoomID        *string
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

// ListPeriod identifies the range preset requested for schedule listings.
type ListPeriod string

const (
	// ListPeriodNone indicates no preset; caller supplied explicit bounds.
	ListPeriodNone ListPeriod = ""
	// ListPeriodDay constrains results to a single day.
	ListPeriodDay ListPeriod = "day"
	// ListPeriodWeek constrains results to the Monday-start week containing the reference time.
	ListPeriodWeek ListPeriod = "week"
	// ListPeriodMonth constrains results to the month containing the reference time.
	ListPeriodMonth ListPeriod = "month"
)

// ListSchedulesParams wraps the data required to list schedules.
type ListSchedulesParams struct {
	Principal       Principal
	ParticipantIDs  []string
	StartsAfter     *time.Time
	EndsBefore      *time.Time
	Period          ListPeriod
	PeriodReference time.Time
}

// RoomInput captures caller provided room fields.
type RoomInput struct {
	Name       string
	Location   string
	Capacity   int
	Facilities *string
}

// Room represents a catalog entry for a physical meeting room.
type Room struct {
	ID         string
	Name       string
	Location   string
	Capacity   int
	Facilities *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CreateRoomParams wraps the data required to create a room.
type CreateRoomParams struct {
	Principal Principal
	Input     RoomInput
}

// UpdateRoomParams wraps the data required to update a room.
type UpdateRoomParams struct {
	Principal Principal
	RoomID    string
	Input     RoomInput
}

// UserInput captures caller provided user attributes.
type UserInput struct {
	Email       string
	DisplayName string
	IsAdmin     bool
}

// User represents an employee account exposed by the application services.
type User struct {
	ID          string
	Email       string
	DisplayName string
	IsAdmin     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateUserParams wraps the data required to create a user.
type CreateUserParams struct {
	Principal Principal
	Input     UserInput
}

// UpdateUserParams wraps the data required to update a user.
type UpdateUserParams struct {
	Principal Principal
	UserID    string
	Input     UserInput
}

// UserCredentials models the authentication attributes persisted for a user.
type UserCredentials struct {
	User           User
	PasswordHash   string
	Disabled       bool
	FailedAttempts int
	LastFailedAt   *time.Time
}

// Session represents an authenticated session issued to a user.
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

// AuthenticateParams captures the data required to authenticate a user.
type AuthenticateParams struct {
	Email       string
	Password    string
	Fingerprint string
}

// AuthenticateResult captures the outcome of a successful authentication attempt.
type AuthenticateResult struct {
	User    User
	Session Session
}

// RefreshSessionParams captures the data required to refresh an existing session.
type RefreshSessionParams struct {
	Token       string
	Fingerprint string
}

// RefreshSessionResult captures the outcome of rotating a session token.
type RefreshSessionResult struct {
	Session Session
}
