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

// Storage implements persistence repositories using an in-process data store
// that simulates SQLite behaviour without relying on CGO.
type Storage struct {
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("sqlite: ensure directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("sqlite: create file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("sqlite: close file: %w", err)
	}

	return &Storage{
		path:                 path,
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[user.ID]; exists {
		return persistence.ErrDuplicate
	}
	key := normalizeEmail(user.Email)
	if existing, ok := s.userByEmail[key]; ok && existing != user.ID {
		return persistence.ErrDuplicate
	}

	if user.PasswordHash == "" {
		return persistence.ErrConstraintViolation
	}

	user.CreatedAt = user.CreatedAt.UTC()
	user.UpdatedAt = user.UpdatedAt.UTC()
	s.users[user.ID] = user
	s.userByEmail[key] = user.ID
	return nil
}

// UpdateUser updates an existing user.
func (s *Storage) UpdateUser(ctx context.Context, user persistence.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.users[user.ID]
	if !ok {
		return persistence.ErrNotFound
	}

	key := normalizeEmail(user.Email)
	if existing, ok := s.userByEmail[key]; ok && existing != user.ID {
		return persistence.ErrDuplicate
	}

	if current.Email != user.Email {
		delete(s.userByEmail, normalizeEmail(current.Email))
	}

	if user.PasswordHash == "" {
		return persistence.ErrConstraintViolation
	}

	user.CreatedAt = current.CreatedAt
	user.UpdatedAt = user.UpdatedAt.UTC()
	s.users[user.ID] = user
	s.userByEmail[key] = user.ID
	return nil
}

// GetUser retrieves a user by ID.
func (s *Storage) GetUser(ctx context.Context, id string) (persistence.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	if !ok {
		return persistence.User{}, persistence.ErrNotFound
	}
	return user, nil
}

// GetUserByEmail retrieves a user by email address.
func (s *Storage) GetUserByEmail(ctx context.Context, email string) (persistence.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.userByEmail[normalizeEmail(email)]
	if !ok {
		return persistence.User{}, persistence.ErrNotFound
	}
	return s.users[id], nil
}

// ListUsers returns users ordered by creation timestamp then ID.
func (s *Storage) ListUsers(ctx context.Context) ([]persistence.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]persistence.User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].CreatedAt.Equal(users[j].CreatedAt) {
			return users[i].ID < users[j].ID
		}
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	return users, nil
}

// DeleteUser removes a user by ID.
func (s *Storage) DeleteUser(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return persistence.ErrNotFound
	}

	for _, schedule := range s.schedules {
		if schedule.CreatorID == id {
			return persistence.ErrForeignKeyViolation
		}
	}

	delete(s.userByEmail, normalizeEmail(user.Email))
	delete(s.users, id)

	for scheduleID, participants := range s.scheduleParticipants {
		filtered := make([]string, 0, len(participants))
		for _, participant := range participants {
			if participant != id {
				filtered = append(filtered, participant)
			}
		}
		s.scheduleParticipants[scheduleID] = filtered
	}
	return nil
}

// CreateRoom stores a new meeting room.
func (s *Storage) CreateRoom(ctx context.Context, room persistence.Room) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.rooms[room.ID]; exists {
		return persistence.ErrDuplicate
	}
	if room.Capacity <= 0 {
		return persistence.ErrConstraintViolation
	}

	room.CreatedAt = room.CreatedAt.UTC()
	room.UpdatedAt = room.UpdatedAt.UTC()
	s.rooms[room.ID] = room
	return nil
}

// UpdateRoom updates an existing room.
func (s *Storage) UpdateRoom(ctx context.Context, room persistence.Room) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.rooms[room.ID]
	if !ok {
		return persistence.ErrNotFound
	}
	if room.Capacity <= 0 {
		return persistence.ErrConstraintViolation
	}

	room.CreatedAt = current.CreatedAt
	room.UpdatedAt = room.UpdatedAt.UTC()
	s.rooms[room.ID] = room
	return nil
}

// GetRoom retrieves a room by ID.
func (s *Storage) GetRoom(ctx context.Context, id string) (persistence.Room, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	room, ok := s.rooms[id]
	if !ok {
		return persistence.Room{}, persistence.ErrNotFound
	}
	return room, nil
}

// ListRooms returns rooms ordered by name then ID.
func (s *Storage) ListRooms(ctx context.Context) ([]persistence.Room, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rooms := make([]persistence.Room, 0, len(s.rooms))
	for _, room := range s.rooms {
		rooms = append(rooms, room)
	}
	sort.Slice(rooms, func(i, j int) bool {
		if rooms[i].Name == rooms[j].Name {
			return rooms[i].ID < rooms[j].ID
		}
		return rooms[i].Name < rooms[j].Name
	})
	return rooms, nil
}

// DeleteRoom deletes a room by ID.
func (s *Storage) DeleteRoom(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[id]; !ok {
		return persistence.ErrNotFound
	}

	delete(s.rooms, id)

	for scheduleID, schedule := range s.schedules {
		if schedule.RoomID != nil && *schedule.RoomID == id {
			schedule.RoomID = nil
			s.schedules[scheduleID] = schedule
		}
	}
	return nil
}

// CreateSchedule stores a schedule with participants.
func (s *Storage) CreateSchedule(ctx context.Context, schedule persistence.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.schedules[schedule.ID]; exists {
		return persistence.ErrDuplicate
	}
	sanitized, err := s.validateScheduleLocked(schedule)
	if err != nil {
		return err
	}

	sanitized.CreatedAt = sanitized.CreatedAt.UTC()
	sanitized.UpdatedAt = sanitized.UpdatedAt.UTC()
	s.schedules[schedule.ID] = sanitized
	s.scheduleParticipants[schedule.ID] = uniqueStrings(sanitized.Participants)
	return nil
}

// UpdateSchedule updates schedule fields and participants.
func (s *Storage) UpdateSchedule(ctx context.Context, schedule persistence.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.schedules[schedule.ID]
	if !ok {
		return persistence.ErrNotFound
	}

	schedule.CreatorID = current.CreatorID
	schedule.CreatedAt = current.CreatedAt
	sanitized, err := s.validateScheduleLocked(schedule)
	if err != nil {
		return err
	}

	sanitized.UpdatedAt = sanitized.UpdatedAt.UTC()
	sanitized.CreatedAt = current.CreatedAt
	sanitized.CreatorID = current.CreatorID
	s.schedules[schedule.ID] = sanitized
	s.scheduleParticipants[schedule.ID] = uniqueStrings(sanitized.Participants)
	return nil
}

// GetSchedule retrieves a schedule by ID.
func (s *Storage) GetSchedule(ctx context.Context, id string) (persistence.Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, ok := s.schedules[id]
	if !ok {
		return persistence.Schedule{}, persistence.ErrNotFound
	}
	schedule.Participants = append([]string(nil), s.scheduleParticipants[id]...)
	return schedule, nil
}

// ListSchedules lists schedules filtered by the provided filter.
func (s *Storage) ListSchedules(ctx context.Context, filter persistence.ScheduleFilter) ([]persistence.Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]persistence.Schedule, 0)
	for _, schedule := range s.schedules {
		if !filterMatchesSchedule(schedule, s.scheduleParticipants[schedule.ID], filter) {
			continue
		}
		scheduleCopy := schedule
		scheduleCopy.Participants = append([]string(nil), s.scheduleParticipants[schedule.ID]...)
		result = append(result, scheduleCopy)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Start.Equal(result[j].Start) {
			return result[i].ID < result[j].ID
		}
		return result[i].Start.Before(result[j].Start)
	})
	return result, nil
}

// DeleteSchedule removes a schedule by ID.
func (s *Storage) DeleteSchedule(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[id]; !ok {
		return persistence.ErrNotFound
	}

	delete(s.schedules, id)
	delete(s.scheduleParticipants, id)

	if ids, ok := s.scheduleRecurrence[id]; ok {
		for _, recurrenceID := range ids {
			delete(s.recurrences, recurrenceID)
		}
	}
	delete(s.scheduleRecurrence, id)

	return nil
}

// UpsertRecurrence creates or updates a recurrence rule.
func (s *Storage) UpsertRecurrence(ctx context.Context, rule persistence.RecurrenceRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[rule.ScheduleID]; !ok {
		return persistence.ErrForeignKeyViolation
	}
	if err := validateRecurrence(rule); err != nil {
		return err
	}

	rule.Weekdays = uniqueWeekdays(rule.Weekdays)

	rule.StartsOn = rule.StartsOn.UTC()
	if rule.EndsOn != nil {
		ends := rule.EndsOn.UTC()
		rule.EndsOn = &ends
	}
	rule.UpdatedAt = rule.UpdatedAt.UTC()

	if existing, ok := s.recurrences[rule.ID]; ok {
		rule.CreatedAt = existing.CreatedAt
	} else {
		rule.CreatedAt = rule.CreatedAt.UTC()
	}

	s.recurrences[rule.ID] = rule
	ids := s.scheduleRecurrence[rule.ScheduleID]
	found := false
	for _, existingID := range ids {
		if existingID == rule.ID {
			found = true
			break
		}
	}
	if !found {
		ids = append(ids, rule.ID)
	}
	sort.Slice(ids, func(i, j int) bool {
		ri := s.recurrences[ids[i]]
		rj := s.recurrences[ids[j]]
		if ri.CreatedAt.Equal(rj.CreatedAt) {
			return ids[i] < ids[j]
		}
		return ri.CreatedAt.Before(rj.CreatedAt)
	})
	s.scheduleRecurrence[rule.ScheduleID] = ids
	return nil
}

// ListRecurrencesForSchedule lists recurrence rules for a schedule ordered by creation time.
func (s *Storage) ListRecurrencesForSchedule(ctx context.Context, scheduleID string) ([]persistence.RecurrenceRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.scheduleRecurrence[scheduleID]
	rules := make([]persistence.RecurrenceRule, 0, len(ids))
	for _, id := range ids {
		if rule, ok := s.recurrences[id]; ok {
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

// DeleteRecurrence deletes a recurrence by ID.
func (s *Storage) DeleteRecurrence(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, ok := s.recurrences[id]
	if !ok {
		return persistence.ErrNotFound
	}
	delete(s.recurrences, id)
	ids := s.scheduleRecurrence[rule.ScheduleID]
	filtered := make([]string, 0, len(ids))
	for _, existingID := range ids {
		if existingID != id {
			filtered = append(filtered, existingID)
		}
	}
	s.scheduleRecurrence[rule.ScheduleID] = filtered
	return nil
}

// DeleteRecurrencesForSchedule deletes recurrences for a schedule.
func (s *Storage) DeleteRecurrencesForSchedule(ctx context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := s.scheduleRecurrence[scheduleID]
	for _, id := range ids {
		delete(s.recurrences, id)
	}
	delete(s.scheduleRecurrence, scheduleID)
	return nil
}

// CreateSession stores a new session token for a user.
func (s *Storage) CreateSession(ctx context.Context, session persistence.Session) (persistence.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session.ID == "" {
		return persistence.Session{}, persistence.ErrConstraintViolation
	}
	if _, exists := s.sessions[session.ID]; exists {
		return persistence.Session{}, persistence.ErrDuplicate
	}
	if session.UserID == "" {
		return persistence.Session{}, persistence.ErrConstraintViolation
	}
	if _, ok := s.users[session.UserID]; !ok {
		return persistence.Session{}, persistence.ErrForeignKeyViolation
	}

	normalized, err := normalizeSession(session)
	if err != nil {
		return persistence.Session{}, err
	}
	if existingID, ok := s.sessionByToken[normalized.Token]; ok && existingID != normalized.ID {
		return persistence.Session{}, persistence.ErrDuplicate
	}

	s.sessions[normalized.ID] = normalized
	s.sessionByToken[normalized.Token] = normalized.ID
	return cloneSession(normalized), nil
}

// GetSession retrieves a session by its token value.
func (s *Storage) GetSession(ctx context.Context, token string) (persistence.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" {
		return persistence.Session{}, persistence.ErrNotFound
	}
	id, ok := s.sessionByToken[normalizedToken]
	if !ok {
		return persistence.Session{}, persistence.ErrNotFound
	}
	session := s.sessions[id]
	return cloneSession(session), nil
}

// UpdateSession updates mutable fields of an existing session.
func (s *Storage) UpdateSession(ctx context.Context, session persistence.Session) (persistence.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.sessions[session.ID]
	if !ok {
		return persistence.Session{}, persistence.ErrNotFound
	}

	session.ID = current.ID
	session.UserID = current.UserID
	session.CreatedAt = current.CreatedAt

	normalized, err := normalizeSession(session)
	if err != nil {
		return persistence.Session{}, err
	}

	if existingID, ok := s.sessionByToken[normalized.Token]; ok && existingID != normalized.ID {
		return persistence.Session{}, persistence.ErrDuplicate
	}

	if currentToken := strings.TrimSpace(current.Token); currentToken != normalized.Token {
		delete(s.sessionByToken, currentToken)
	}

	s.sessions[normalized.ID] = normalized
	s.sessionByToken[normalized.Token] = normalized.ID
	return cloneSession(normalized), nil
}

// RevokeSession marks a session as revoked based on its token value.
func (s *Storage) RevokeSession(ctx context.Context, token string, revokedAt time.Time) (persistence.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" {
		return persistence.Session{}, persistence.ErrNotFound
	}

	id, ok := s.sessionByToken[normalizedToken]
	if !ok {
		return persistence.Session{}, persistence.ErrNotFound
	}

	session := s.sessions[id]
	revoked := revokedAt.UTC()
	session.RevokedAt = &revoked
	session.UpdatedAt = revoked

	normalized, err := normalizeSession(session)
	if err != nil {
		return persistence.Session{}, err
	}

	s.sessions[normalized.ID] = normalized
	s.sessionByToken[normalized.Token] = normalized.ID
	return cloneSession(normalized), nil
}

// DeleteExpiredSessions removes sessions that expired on or before the provided timestamp.
func (s *Storage) DeleteExpiredSessions(ctx context.Context, reference time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := reference.UTC()
	for id, session := range s.sessions {
		if session.ExpiresAt.IsZero() {
			continue
		}
		if !session.ExpiresAt.After(cutoff) {
			delete(s.sessions, id)
			delete(s.sessionByToken, strings.TrimSpace(session.Token))
		}
	}
	return nil
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

func validateRecurrence(rule persistence.RecurrenceRule) error {
	if rule.EndsOn != nil && rule.EndsOn.Before(rule.StartsOn) {
		return persistence.ErrConstraintViolation
	}
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
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

func uniqueWeekdays(days []time.Weekday) []time.Weekday {
	seen := make(map[time.Weekday]struct{}, len(days))
	result := make([]time.Weekday, 0, len(days))
	for _, day := range days {
		if day < time.Sunday || day > time.Saturday {
			continue
		}
		if _, ok := seen[day]; ok {
			continue
		}
		seen[day] = struct{}{}
		result = append(result, day)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func encodeWeekdays(days []time.Weekday) int64 {
	unique := uniqueWeekdays(days)
	var mask int64
	for _, day := range unique {
		mask |= 1 << uint(day)
	}
	return mask
}

func decodeWeekdays(mask int64) []time.Weekday {
	result := make([]time.Weekday, 0)
	for day := time.Sunday; day <= time.Saturday; day++ {
		if mask&(1<<uint(day)) != 0 {
			result = append(result, day)
		}
	}
	return result
}

func normalizeSession(session persistence.Session) (persistence.Session, error) {
	if session.ID == "" {
		return persistence.Session{}, persistence.ErrConstraintViolation
	}
	session.Token = strings.TrimSpace(session.Token)
	if session.Token == "" {
		return persistence.Session{}, persistence.ErrConstraintViolation
	}
	session.Fingerprint = strings.TrimSpace(session.Fingerprint)
	session.CreatedAt = session.CreatedAt.UTC()
	session.UpdatedAt = session.UpdatedAt.UTC()
	session.ExpiresAt = session.ExpiresAt.UTC()
	if session.RevokedAt != nil {
		revoked := session.RevokedAt.UTC()
		session.RevokedAt = &revoked
	}
	return session, nil
}

func cloneSession(session persistence.Session) persistence.Session {
	clone := session
	if session.RevokedAt != nil {
		revoked := session.RevokedAt.UTC()
		clone.RevokedAt = &revoked
	}
	return clone
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
