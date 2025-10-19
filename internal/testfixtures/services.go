package testfixtures

import (
	"log/slog"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
)

// ServiceFactory assists tests with constructing application services using
// deterministic identifiers and clocks.
type ServiceFactory struct {
	Clock       *Clock
	IDGenerator *IDGenerator
}

// ServiceFactoryOption configures a ServiceFactory instance.
type ServiceFactoryOption func(*ServiceFactory)

// NewServiceFactory constructs a ServiceFactory with defaults.
func NewServiceFactory(opts ...ServiceFactoryOption) *ServiceFactory {
	factory := &ServiceFactory{
		Clock:       NewClock(time.Time{}),
		IDGenerator: NewIDGenerator("id"),
	}
	for _, opt := range opts {
		opt(factory)
	}
	if factory.Clock == nil {
		factory.Clock = NewClock(time.Time{})
	}
	if factory.IDGenerator == nil {
		factory.IDGenerator = NewIDGenerator("id")
	}
	return factory
}

// WithClock overrides the clock used by the factory.
func WithClock(clock *Clock) ServiceFactoryOption {
	return func(factory *ServiceFactory) {
		factory.Clock = clock
	}
}

// WithIDGenerator overrides the identifier generator used by the factory.
func WithIDGenerator(generator *IDGenerator) ServiceFactoryOption {
	return func(factory *ServiceFactory) {
		factory.IDGenerator = generator
	}
}

// ScheduleServiceDeps captures dependencies for constructing a schedule service.
type ScheduleServiceDeps struct {
	Schedules   application.ScheduleRepository
	Users       application.UserDirectory
	Rooms       application.RoomCatalog
	Recurrences application.RecurrenceRepository
	IDGenerator func() string
	Now         func() time.Time
	Logger      *slog.Logger
}

// NewScheduleService builds a schedule service using the supplied dependencies
// combined with the factory defaults.
func (f *ServiceFactory) NewScheduleService(deps ScheduleServiceDeps) *application.ScheduleService {
	idGen := deps.IDGenerator
	if idGen == nil {
		idGen = f.IDGenerator.NextFunc()
	}
	now := deps.Now
	if now == nil {
		now = f.Clock.NowFunc()
	}
	return application.NewScheduleServiceWithLogger(
		deps.Schedules,
		deps.Users,
		deps.Rooms,
		deps.Recurrences,
		idGen,
		now,
		deps.Logger,
	)
}

// RoomServiceDeps captures dependencies for constructing a room service.
type RoomServiceDeps struct {
	Rooms       application.RoomRepository
	IDGenerator func() string
	Now         func() time.Time
	Logger      *slog.Logger
}

// NewRoomService builds a room service using the supplied dependencies.
func (f *ServiceFactory) NewRoomService(deps RoomServiceDeps) *application.RoomService {
	idGen := deps.IDGenerator
	if idGen == nil {
		idGen = f.IDGenerator.NextFunc()
	}
	now := deps.Now
	if now == nil {
		now = f.Clock.NowFunc()
	}
	return application.NewRoomServiceWithLogger(
		deps.Rooms,
		idGen,
		now,
		deps.Logger,
	)
}

// UserServiceDeps captures dependencies for constructing a user service.
type UserServiceDeps struct {
	Users       application.UserRepository
	IDGenerator func() string
	Now         func() time.Time
	Logger      *slog.Logger
}

// NewUserService builds a user service using the supplied dependencies.
func (f *ServiceFactory) NewUserService(deps UserServiceDeps) *application.UserService {
	idGen := deps.IDGenerator
	if idGen == nil {
		idGen = f.IDGenerator.NextFunc()
	}
	now := deps.Now
	if now == nil {
		now = f.Clock.NowFunc()
	}
	return application.NewUserServiceWithLogger(
		deps.Users,
		idGen,
		now,
		deps.Logger,
	)
}

// AuthServiceDeps captures dependencies for constructing an auth service.
type AuthServiceDeps struct {
	Credentials    application.CredentialStore
	Sessions       application.SessionRepository
	PasswordVerify application.PasswordVerifier
	TokenGenerator func() string
	Now            func() time.Time
	SessionTTL     time.Duration
	Logger         *slog.Logger
}

// NewAuthService builds an auth service using the supplied dependencies.
func (f *ServiceFactory) NewAuthService(deps AuthServiceDeps) *application.AuthService {
	token := deps.TokenGenerator
	if token == nil {
		token = f.IDGenerator.NextFunc()
	}
	now := deps.Now
	if now == nil {
		now = f.Clock.NowFunc()
	}
	return application.NewAuthServiceWithLogger(
		deps.Credentials,
		deps.Sessions,
		deps.PasswordVerify,
		token,
		now,
		deps.SessionTTL,
		deps.Logger,
	)
}
