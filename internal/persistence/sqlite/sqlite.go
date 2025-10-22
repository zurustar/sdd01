package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
	"github.com/example/enterprise-scheduler/internal/persistence/sqlite/migration"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Storage implements persistence repositories using SQLite database
type Storage struct {
	pool *ConnectionPool
	
	// Repository implementations
	userRepo       *UserRepository
	roomRepo       *RoomRepository
	scheduleRepo   *ScheduleRepository
	recurrenceRepo *RecurrenceRepository
	sessionRepo    *SessionRepository
	
	// Legacy fields for backward compatibility during migration
	mu sync.RWMutex
	path string
	users       map[string]persistence.User
	userByEmail map[string]string
	rooms map[string]persistence.Room
	schedules            map[string]persistence.Schedule
	scheduleParticipants map[string][]string
	recurrences        map[string]persistence.RecurrenceRule
	scheduleRecurrence map[string][]string
	sessions       map[string]persistence.Session
	sessionByToken map[string]string
}

// Open initialises the storage using the provided DSN or file path.
func Open(dsn string) (*Storage, error) {
	path, err := normalizeDSN(dsn)
	if err != nil {
		return nil, err
	}

	// Create SQLite configuration
	config := migration.DefaultSQLiteConfig(path)
	
	// Create connection pool
	pool, err := NewConnectionPool(config)
	if err != nil {
		return nil, fmt.Errorf("sqlite: failed to create connection pool: %w", err)
	}
	
	// Create repository instances
	userRepo := NewUserRepository(pool)
	roomRepo := NewRoomRepository(pool)
	scheduleRepo := NewScheduleRepository(pool)
	recurrenceRepo := NewRecurrenceRepository(pool)
	sessionRepo := NewSessionRepository(pool)

	return &Storage{
		pool:           pool,
		userRepo:       userRepo,
		roomRepo:       roomRepo,
		scheduleRepo:   scheduleRepo,
		recurrenceRepo: recurrenceRepo,
		sessionRepo:    sessionRepo,
		path:           path,
		// Initialize legacy maps for backward compatibility
		users:                make(map[string]persistence.User),
		userByEmail:          make(map[string]string),
		rooms:                make(map[string]persistence.Room),
		schedules:            make(map[string]persistence.Schedule),
		scheduleParticipants: make(map[string][]string),
		recurrences:          make(map[string]persistence.RecurrenceRule),
		scheduleRecurrence:   make(map[string][]string),
		sessions:             make(map[string]persistence.Session),
		sessionByToken:       make(map[string]string),
	}, nil
}

// Close releases any held resources.
func (s *Storage) Close() error {
	if s.pool != nil {
		return s.pool.Close()
	}
	return nil
}

// Migrate applies database migrations using the migration system instead of embedded schema.
// This replaces the previous embedded schema approach with a file-based migration system.
func (s *Storage) Migrate(ctx context.Context) error {
	// Set up migration system components
	// Get the absolute path to the migrations directory relative to this package
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("sqlite: failed to determine package location")
	}
	packageDir := filepath.Dir(filename)
	migrationDir := filepath.Join(packageDir, "migrations")

	// Configure SQLite connection for migrations
	sqliteConfig := migration.DefaultSQLiteConfig(s.path)
	connectionManager := migration.NewConnectionManager(sqliteConfig)
	
	// Get database connection
	db, err := connectionManager.GetConnection()
	if err != nil {
		return fmt.Errorf("sqlite: failed to get database connection: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			// Log error but don't fail the migration
		}
	}()

	// Create migration components
	scanner := migration.NewFileScanner()
	executor := migration.NewSQLiteExecutor(db)
	migrationManager := migration.NewMigrationManager(scanner, executor, migrationDir)

	// Execute migrations
	if err := migrationManager.RunMigrations(ctx); err != nil {
		return fmt.Errorf("sqlite: migration execution failed: %w", err)
	}

	return nil
}

type migrationFile struct {
	Version string
	Name    string
	Up      string
	Down    string
}

func loadEmbeddedMigrations(fsys embed.FS) ([]migrationFile, error) {
	paths, err := fs.Glob(fsys, "migrations/*.up.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	migrations := make([]migrationFile, 0, len(paths))
	for _, upPath := range paths {
		contents, err := fs.ReadFile(fsys, upPath)
		if err != nil {
			return nil, fmt.Errorf("sqlite: read migration %s: %w", upPath, err)
		}
		base := filepath.Base(upPath)
		version := strings.SplitN(base, "_", 2)[0]
		downPath := strings.Replace(upPath, ".up.", ".down.", 1)
		var down string
		if data, err := fs.ReadFile(fsys, downPath); err == nil {
			down = string(data)
		}
		migrations = append(migrations, migrationFile{
			Version: version,
			Name:    base,
			Up:      string(contents),
			Down:    down,
		})
	}
	return migrations, nil
}

func (s *Storage) migrationStatePath() string {
	return s.path + ".migrations.json"
}

func loadMigrationState(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: read migration state: %w", err)
	}
	if len(data) == 0 {
		return make(map[string]string), nil
	}
	var state map[string]string
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("sqlite: decode migration state: %w", err)
	}
	if state == nil {
		state = make(map[string]string)
	}
	return state, nil
}

func saveMigrationState(path string, state map[string]string) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("sqlite: encode migration state: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("sqlite: write migration state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("sqlite: replace migration state: %w", err)
	}
	return nil
}

// CreateUser inserts a new user.
func (s *Storage) CreateUser(ctx context.Context, user persistence.User) error {
	return s.userRepo.CreateUser(ctx, user)
}

// UpdateUser updates an existing user.
func (s *Storage) UpdateUser(ctx context.Context, user persistence.User) error {
	return s.userRepo.UpdateUser(ctx, user)
}

// GetUser retrieves a user by ID.
func (s *Storage) GetUser(ctx context.Context, id string) (persistence.User, error) {
	return s.userRepo.GetUser(ctx, id)
}

// GetUserByEmail retrieves a user by email address.
func (s *Storage) GetUserByEmail(ctx context.Context, email string) (persistence.User, error) {
	return s.userRepo.GetUserByEmail(ctx, email)
}

// ListUsers returns users ordered by creation timestamp then ID.
func (s *Storage) ListUsers(ctx context.Context) ([]persistence.User, error) {
	return s.userRepo.ListUsers(ctx)
}

// DeleteUser removes a user by ID.
func (s *Storage) DeleteUser(ctx context.Context, id string) error {
	return s.userRepo.DeleteUser(ctx, id)
}

// CreateRoom stores a new meeting room.
func (s *Storage) CreateRoom(ctx context.Context, room persistence.Room) error {
	return s.roomRepo.CreateRoom(ctx, room)
}

// UpdateRoom updates an existing room.
func (s *Storage) UpdateRoom(ctx context.Context, room persistence.Room) error {
	return s.roomRepo.UpdateRoom(ctx, room)
}

// GetRoom retrieves a room by ID.
func (s *Storage) GetRoom(ctx context.Context, id string) (persistence.Room, error) {
	return s.roomRepo.GetRoom(ctx, id)
}

// ListRooms returns rooms ordered by name then ID.
func (s *Storage) ListRooms(ctx context.Context) ([]persistence.Room, error) {
	return s.roomRepo.ListRooms(ctx)
}

// DeleteRoom deletes a room by ID.
func (s *Storage) DeleteRoom(ctx context.Context, id string) error {
	return s.roomRepo.DeleteRoom(ctx, id)
}

// CreateSchedule stores a schedule with participants.
func (s *Storage) CreateSchedule(ctx context.Context, schedule persistence.Schedule) error {
	return s.scheduleRepo.CreateSchedule(ctx, schedule)
}

// UpdateSchedule updates schedule fields and participants.
func (s *Storage) UpdateSchedule(ctx context.Context, schedule persistence.Schedule) error {
	return s.scheduleRepo.UpdateSchedule(ctx, schedule)
}

// GetSchedule retrieves a schedule by ID.
func (s *Storage) GetSchedule(ctx context.Context, id string) (persistence.Schedule, error) {
	return s.scheduleRepo.GetSchedule(ctx, id)
}

// ListSchedules lists schedules filtered by the provided filter.
func (s *Storage) ListSchedules(ctx context.Context, filter persistence.ScheduleFilter) ([]persistence.Schedule, error) {
	return s.scheduleRepo.ListSchedules(ctx, filter)
}

// DeleteSchedule removes a schedule by ID.
func (s *Storage) DeleteSchedule(ctx context.Context, id string) error {
	return s.scheduleRepo.DeleteSchedule(ctx, id)
}

// UpsertRecurrence creates or updates a recurrence rule.
func (s *Storage) UpsertRecurrence(ctx context.Context, rule persistence.RecurrenceRule) error {
	return s.recurrenceRepo.UpsertRecurrence(ctx, rule)
}

// ListRecurrencesForSchedule lists recurrence rules for a schedule ordered by creation time.
func (s *Storage) ListRecurrencesForSchedule(ctx context.Context, scheduleID string) ([]persistence.RecurrenceRule, error) {
	return s.recurrenceRepo.ListRecurrencesForSchedule(ctx, scheduleID)
}

// DeleteRecurrence deletes a recurrence by ID.
func (s *Storage) DeleteRecurrence(ctx context.Context, id string) error {
	return s.recurrenceRepo.DeleteRecurrence(ctx, id)
}

// DeleteRecurrencesForSchedule deletes recurrences for a schedule.
func (s *Storage) DeleteRecurrencesForSchedule(ctx context.Context, scheduleID string) error {
	return s.recurrenceRepo.DeleteRecurrencesForSchedule(ctx, scheduleID)
}

// CreateSession stores a new session token for a user.
func (s *Storage) CreateSession(ctx context.Context, session persistence.Session) (persistence.Session, error) {
	return s.sessionRepo.CreateSession(ctx, session)
}

// GetSession retrieves a session by its token value.
func (s *Storage) GetSession(ctx context.Context, token string) (persistence.Session, error) {
	return s.sessionRepo.GetSession(ctx, token)
}

// UpdateSession updates mutable fields of an existing session.
func (s *Storage) UpdateSession(ctx context.Context, session persistence.Session) (persistence.Session, error) {
	return s.sessionRepo.UpdateSession(ctx, session)
}

// RevokeSession marks a session as revoked based on its token value.
func (s *Storage) RevokeSession(ctx context.Context, token string, revokedAt time.Time) (persistence.Session, error) {
	return s.sessionRepo.RevokeSession(ctx, token, revokedAt)
}

// DeleteExpiredSessions removes sessions that expired on or before the provided timestamp.
func (s *Storage) DeleteExpiredSessions(ctx context.Context, reference time.Time) error {
	return s.sessionRepo.DeleteExpiredSessions(ctx, reference)
}

func (s *Storage) validateScheduleLocked(schedule persistence.Schedule) (persistence.Schedule, error) {
	if schedule.End.Before(schedule.Start) || schedule.End.Equal(schedule.Start) {
		return persistence.Schedule{}, persistence.ErrConstraintViolation
	}
	if _, ok := s.users[schedule.CreatorID]; !ok {
		return persistence.Schedule{}, persistence.ErrForeignKeyViolation
	}
	if schedule.RoomID != nil {
		if _, ok := s.rooms[*schedule.RoomID]; !ok {
			return persistence.Schedule{}, persistence.ErrForeignKeyViolation
		}
	}
	for _, participant := range schedule.Participants {
		if _, ok := s.users[participant]; !ok {
			return persistence.Schedule{}, persistence.ErrForeignKeyViolation
		}
	}
	schedule.Start = schedule.Start.UTC()
	schedule.End = schedule.End.UTC()
	if schedule.Memo != nil {
		memo := strings.TrimSpace(*schedule.Memo)
		schedule.Memo = &memo
	}
	if schedule.WebConferenceURL != nil {
		url := strings.TrimSpace(*schedule.WebConferenceURL)
		schedule.WebConferenceURL = &url
	}
	if schedule.RoomID != nil {
		roomID := strings.TrimSpace(*schedule.RoomID)
		schedule.RoomID = &roomID
	}
	schedule.Participants = uniqueStrings(schedule.Participants)
	return schedule, nil
}

func filterMatchesSchedule(schedule persistence.Schedule, participants []string, filter persistence.ScheduleFilter) bool {
	if filter.StartsAfter != nil && !schedule.End.After(filter.StartsAfter.UTC()) {
		return false
	}
	if filter.EndsBefore != nil && !schedule.Start.Before(filter.EndsBefore.UTC()) {
		return false
	}
	if len(filter.ParticipantIDs) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(participants)+1)
	for _, participant := range participants {
		set[participant] = struct{}{}
	}
	if schedule.CreatorID != "" {
		set[schedule.CreatorID] = struct{}{}
	}
	for _, participant := range filter.ParticipantIDs {
		if _, ok := set[participant]; ok {
			return true
		}
	}
	return false
}



func normalizeDSN(dsn string) (string, error) {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return "", errors.New("sqlite: empty dsn")
	}
	if strings.HasPrefix(trimmed, "file:") {
		path := strings.TrimPrefix(trimmed, "file:")
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}
		trimmed = path
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("sqlite: normalise dsn: %w", err)
	}
	return abs, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}



func runStatements(ctx context.Context, db *sql.DB, script string) error {
	statements := splitSQLStatements(script)
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("sqlite: execute migration statement: %w", err)
		}
	}
	return nil
}

func splitSQLStatements(script string) []string {
	var (
		statements []string
		builder    strings.Builder
		inString   bool
	)

	for i := 0; i < len(script); i++ {
		ch := script[i]

		if !inString {
			if ch == '-' && i+1 < len(script) && script[i+1] == '-' {
				i += 2
				for i < len(script) && script[i] != '\n' {
					i++
				}
				continue
			}
			if ch == '/' && i+1 < len(script) && script[i+1] == '*' {
				i += 2
				for i < len(script)-1 {
					if script[i] == '*' && script[i+1] == '/' {
						i++
						break
					}
					i++
				}
				continue
			}
		}

		if ch == '\'' {
			builder.WriteByte(ch)
			if inString {
				if i+1 < len(script) && script[i+1] == '\'' {
					builder.WriteByte(script[i+1])
					i++
				} else {
					inString = false
				}
			} else {
				inString = true
			}
			continue
		}

		if ch == ';' && !inString {
			stmt := strings.TrimSpace(builder.String())
			builder.Reset()
			if stmt != "" {
				statements = append(statements, stmt)
			}
			continue
		}

		builder.WriteByte(ch)
	}

	if trailing := strings.TrimSpace(builder.String()); trailing != "" {
		statements = append(statements, trailing)
	}
	return statements
}
