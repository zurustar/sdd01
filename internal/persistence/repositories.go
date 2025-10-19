package persistence

import "context"
import "time"

// UserRepository exposes CRUD operations for users.
type UserRepository interface {
	CreateUser(ctx context.Context, user User) error
	UpdateUser(ctx context.Context, user User) error
	GetUser(ctx context.Context, id string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	DeleteUser(ctx context.Context, id string) error
}

// RoomRepository exposes CRUD operations for rooms.
type RoomRepository interface {
	CreateRoom(ctx context.Context, room Room) error
	UpdateRoom(ctx context.Context, room Room) error
	GetRoom(ctx context.Context, id string) (Room, error)
	ListRooms(ctx context.Context) ([]Room, error)
	DeleteRoom(ctx context.Context, id string) error
}

// ScheduleFilter narrows schedule queries.
type ScheduleFilter struct {
	ParticipantIDs []string
	StartsAfter    *time.Time
	EndsBefore     *time.Time
}

// ScheduleRepository stores schedule entries and their participants.
type ScheduleRepository interface {
	CreateSchedule(ctx context.Context, schedule Schedule) error
	UpdateSchedule(ctx context.Context, schedule Schedule) error
	GetSchedule(ctx context.Context, id string) (Schedule, error)
	ListSchedules(ctx context.Context, filter ScheduleFilter) ([]Schedule, error)
	DeleteSchedule(ctx context.Context, id string) error
}

// RecurrenceRepository stores recurrence rules attached to schedules.
type RecurrenceRepository interface {
	UpsertRecurrence(ctx context.Context, rule RecurrenceRule) error
	ListRecurrencesForSchedule(ctx context.Context, scheduleID string) ([]RecurrenceRule, error)
	DeleteRecurrence(ctx context.Context, id string) error
	DeleteRecurrencesForSchedule(ctx context.Context, scheduleID string) error
}

// SessionRepository stores authentication session state.
type SessionRepository interface {
	CreateSession(ctx context.Context, session Session) (Session, error)
	GetSession(ctx context.Context, token string) (Session, error)
	UpdateSession(ctx context.Context, session Session) (Session, error)
	RevokeSession(ctx context.Context, token string, revokedAt time.Time) (Session, error)
	DeleteExpiredSessions(ctx context.Context, reference time.Time) error
}
