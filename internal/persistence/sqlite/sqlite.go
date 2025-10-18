package sqlite

/*
#cgo CFLAGS: -DSQLITE_THREADSAFE=1
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>

static int bind_text(sqlite3_stmt* stmt, int idx, const char* text) {
    return sqlite3_bind_text(stmt, idx, text, -1, SQLITE_TRANSIENT);
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	_ "embed"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

//go:embed schema.sql
var schemaSQL string

const timeLayout = time.RFC3339Nano

var errNoRows = errors.New("sqlite: no rows")

// Storage implements persistence repositories backed by SQLite using cgo bindings.
type Storage struct {
	mu sync.Mutex
	db *C.sqlite3
}

// Open initialises a SQLite storage using the provided DSN or file path.
func Open(dsn string) (*Storage, error) {
	path, err := normalizeDSN(dsn)
	if err != nil {
		return nil, err
	}

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var db *C.sqlite3
	flags := C.int(C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE | C.SQLITE_OPEN_FULLMUTEX)
	if rc := C.sqlite3_open_v2(cpath, &db, flags, nil); rc != C.SQLITE_OK {
		msg := C.GoString(C.sqlite3_errmsg(db))
		if db != nil {
			C.sqlite3_close_v2(db)
		}
		return nil, fmt.Errorf("sqlite: open: %s", msg)
	}

	storage := &Storage{db: db}
	if err := storage.execSimple("PRAGMA foreign_keys = ON;"); err != nil {
		_ = storage.Close()
		return nil, err
	}

	return storage, nil
}

// Close releases SQLite resources.
func (s *Storage) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rc := C.sqlite3_close_v2(s.db)
	s.db = nil
	if rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite: close: %s", C.GoString(C.sqlite3_errstr(rc)))
	}
	return nil
}

// Migrate applies the schema to the database.
func (s *Storage) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite: storage not initialised")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execLocked(schemaSQL)
}

// CreateUser inserts a new user.
func (s *Storage) CreateUser(ctx context.Context, user persistence.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`INSERT INTO users(id, email, display_name, is_admin, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, user.ID); err != nil {
		return err
	}
	if err := bindText(stmt, 2, user.Email); err != nil {
		return err
	}
	if err := bindText(stmt, 3, user.DisplayName); err != nil {
		return err
	}
	if err := bindBool(stmt, 4, user.IsAdmin); err != nil {
		return err
	}
	if err := bindTime(stmt, 5, user.CreatedAt); err != nil {
		return err
	}
	if err := bindTime(stmt, 6, user.UpdatedAt); err != nil {
		return err
	}

	if err := stepDone(stmt); err != nil {
		return err
	}
	return nil
}

// UpdateUser updates an existing user.
func (s *Storage) UpdateUser(ctx context.Context, user persistence.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`UPDATE users SET email = ?, display_name = ?, is_admin = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, user.Email); err != nil {
		return err
	}
	if err := bindText(stmt, 2, user.DisplayName); err != nil {
		return err
	}
	if err := bindBool(stmt, 3, user.IsAdmin); err != nil {
		return err
	}
	if err := bindTime(stmt, 4, user.UpdatedAt); err != nil {
		return err
	}
	if err := bindText(stmt, 5, user.ID); err != nil {
		return err
	}

	if err := stepDone(stmt); err != nil {
		return err
	}
	if C.sqlite3_changes(s.db) == 0 {
		return persistence.ErrNotFound
	}
	return nil
}

// GetUser retrieves a user by ID.
func (s *Storage) GetUser(ctx context.Context, id string) (persistence.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`SELECT id, email, display_name, is_admin, created_at, updated_at FROM users WHERE id = ?`)
	if err != nil {
		return persistence.User{}, err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, id); err != nil {
		return persistence.User{}, err
	}

	user, err := scanUser(stmt)
	if errors.Is(err, errNoRows) {
		return persistence.User{}, persistence.ErrNotFound
	}
	if err != nil {
		return persistence.User{}, err
	}
	return user, nil
}

// GetUserByEmail retrieves a user by email address.
func (s *Storage) GetUserByEmail(ctx context.Context, email string) (persistence.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`SELECT id, email, display_name, is_admin, created_at, updated_at FROM users WHERE lower(email) = lower(?)`)
	if err != nil {
		return persistence.User{}, err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, email); err != nil {
		return persistence.User{}, err
	}

	user, err := scanUser(stmt)
	if errors.Is(err, errNoRows) {
		return persistence.User{}, persistence.ErrNotFound
	}
	if err != nil {
		return persistence.User{}, err
	}
	return user, nil
}

// ListUsers returns users ordered by creation timestamp.
func (s *Storage) ListUsers(ctx context.Context) ([]persistence.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`SELECT id, email, display_name, is_admin, created_at, updated_at FROM users ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)

	users := make([]persistence.User, 0)
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, fmt.Errorf("sqlite: list users: %s", s.lastErrorLocked())
		}
		user, err := rowToUser(stmt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// DeleteUser removes a user by ID.
func (s *Storage) DeleteUser(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`DELETE FROM users WHERE id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, id); err != nil {
		return err
	}

	if err := stepDone(stmt); err != nil {
		return err
	}
	if C.sqlite3_changes(s.db) == 0 {
		return persistence.ErrNotFound
	}
	return nil
}

// CreateRoom stores a new meeting room.
func (s *Storage) CreateRoom(ctx context.Context, room persistence.Room) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`INSERT INTO rooms(id, name, location, capacity, facilities, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, room.ID); err != nil {
		return err
	}
	if err := bindText(stmt, 2, room.Name); err != nil {
		return err
	}
	if err := bindText(stmt, 3, room.Location); err != nil {
		return err
	}
	if err := bindInt(stmt, 4, room.Capacity); err != nil {
		return err
	}
	if err := bindOptionalText(stmt, 5, room.Facilities); err != nil {
		return err
	}
	if err := bindTime(stmt, 6, room.CreatedAt); err != nil {
		return err
	}
	if err := bindTime(stmt, 7, room.UpdatedAt); err != nil {
		return err
	}

	return stepDone(stmt)
}

// UpdateRoom updates an existing room.
func (s *Storage) UpdateRoom(ctx context.Context, room persistence.Room) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`UPDATE rooms SET name = ?, location = ?, capacity = ?, facilities = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, room.Name); err != nil {
		return err
	}
	if err := bindText(stmt, 2, room.Location); err != nil {
		return err
	}
	if err := bindInt(stmt, 3, room.Capacity); err != nil {
		return err
	}
	if err := bindOptionalText(stmt, 4, room.Facilities); err != nil {
		return err
	}
	if err := bindTime(stmt, 5, room.UpdatedAt); err != nil {
		return err
	}
	if err := bindText(stmt, 6, room.ID); err != nil {
		return err
	}

	if err := stepDone(stmt); err != nil {
		return err
	}
	if C.sqlite3_changes(s.db) == 0 {
		return persistence.ErrNotFound
	}
	return nil
}

// GetRoom retrieves a room by ID.
func (s *Storage) GetRoom(ctx context.Context, id string) (persistence.Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`SELECT id, name, location, capacity, facilities, created_at, updated_at FROM rooms WHERE id = ?`)
	if err != nil {
		return persistence.Room{}, err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, id); err != nil {
		return persistence.Room{}, err
	}

	room, err := scanRoom(stmt)
	if errors.Is(err, errNoRows) {
		return persistence.Room{}, persistence.ErrNotFound
	}
	if err != nil {
		return persistence.Room{}, err
	}
	return room, nil
}

// ListRooms returns rooms ordered by name then ID.
func (s *Storage) ListRooms(ctx context.Context) ([]persistence.Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`SELECT id, name, location, capacity, facilities, created_at, updated_at FROM rooms ORDER BY name ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)

	rooms := make([]persistence.Room, 0)
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, fmt.Errorf("sqlite: list rooms: %s", s.lastErrorLocked())
		}
		room, err := rowToRoom(stmt)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, nil
}

// DeleteRoom deletes a room by ID.
func (s *Storage) DeleteRoom(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`DELETE FROM rooms WHERE id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, id); err != nil {
		return err
	}
	if err := stepDone(stmt); err != nil {
		return err
	}
	if C.sqlite3_changes(s.db) == 0 {
		return persistence.ErrNotFound
	}
	return nil
}

// CreateSchedule stores a schedule with participants.
func (s *Storage) CreateSchedule(ctx context.Context, schedule persistence.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	participants := uniqueStrings(schedule.Participants)

	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	insertSchedule, err := s.prepareLocked(`INSERT INTO schedules(id, title, start_at, end_at, creator_id, memo, room_id, web_conference_url, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(insertSchedule)

	if err := bindText(insertSchedule, 1, schedule.ID); err != nil {
		return err
	}
	if err := bindText(insertSchedule, 2, schedule.Title); err != nil {
		return err
	}
	if err := bindTime(insertSchedule, 3, schedule.Start); err != nil {
		return err
	}
	if err := bindTime(insertSchedule, 4, schedule.End); err != nil {
		return err
	}
	if err := bindText(insertSchedule, 5, schedule.CreatorID); err != nil {
		return err
	}
	if err := bindOptionalText(insertSchedule, 6, schedule.Memo); err != nil {
		return err
	}
	if err := bindOptionalText(insertSchedule, 7, schedule.RoomID); err != nil {
		return err
	}
	if err := bindOptionalText(insertSchedule, 8, schedule.WebConferenceURL); err != nil {
		return err
	}
	if err := bindTime(insertSchedule, 9, schedule.CreatedAt); err != nil {
		return err
	}
	if err := bindTime(insertSchedule, 10, schedule.UpdatedAt); err != nil {
		return err
	}

	if err := stepDone(insertSchedule); err != nil {
		return err
	}

	if len(participants) > 0 {
		insertParticipant, err := s.prepareLocked(`INSERT INTO schedule_participants(schedule_id, participant_id) VALUES(?, ?)`)
		if err != nil {
			return err
		}
		defer C.sqlite3_finalize(insertParticipant)

		for _, participant := range participants {
			if err := bindText(insertParticipant, 1, schedule.ID); err != nil {
				return err
			}
			if err := bindText(insertParticipant, 2, participant); err != nil {
				return err
			}
			if err := stepDone(insertParticipant); err != nil {
				return err
			}
			C.sqlite3_reset(insertParticipant)
			C.sqlite3_clear_bindings(insertParticipant)
		}
	}

	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// UpdateSchedule updates schedule fields and participants.
func (s *Storage) UpdateSchedule(ctx context.Context, schedule persistence.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	participants := uniqueStrings(schedule.Participants)

	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	existing, err := s.prepareLocked(`SELECT creator_id, created_at FROM schedules WHERE id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(existing)

	if err := bindText(existing, 1, schedule.ID); err != nil {
		return err
	}

	rc := C.sqlite3_step(existing)
	if rc == C.SQLITE_DONE {
		return persistence.ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return fmt.Errorf("sqlite: fetch schedule: %s", s.lastErrorLocked())
	}
	creatorID := columnText(existing, 0)
	createdAt, err := columnTime(existing, 1)
	if err != nil {
		return err
	}

	updateStmt, err := s.prepareLocked(`UPDATE schedules SET title = ?, start_at = ?, end_at = ?, memo = ?, room_id = ?, web_conference_url = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(updateStmt)

	if err := bindText(updateStmt, 1, schedule.Title); err != nil {
		return err
	}
	if err := bindTime(updateStmt, 2, schedule.Start); err != nil {
		return err
	}
	if err := bindTime(updateStmt, 3, schedule.End); err != nil {
		return err
	}
	if err := bindOptionalText(updateStmt, 4, schedule.Memo); err != nil {
		return err
	}
	if err := bindOptionalText(updateStmt, 5, schedule.RoomID); err != nil {
		return err
	}
	if err := bindOptionalText(updateStmt, 6, schedule.WebConferenceURL); err != nil {
		return err
	}
	if err := bindTime(updateStmt, 7, schedule.UpdatedAt); err != nil {
		return err
	}
	if err := bindText(updateStmt, 8, schedule.ID); err != nil {
		return err
	}
	if err := stepDone(updateStmt); err != nil {
		return err
	}

	deleteParticipants, err := s.prepareLocked(`DELETE FROM schedule_participants WHERE schedule_id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(deleteParticipants)

	if err := bindText(deleteParticipants, 1, schedule.ID); err != nil {
		return err
	}
	if err := stepDone(deleteParticipants); err != nil {
		return err
	}

	if len(participants) > 0 {
		insertParticipant, err := s.prepareLocked(`INSERT INTO schedule_participants(schedule_id, participant_id) VALUES(?, ?)`)
		if err != nil {
			return err
		}
		defer C.sqlite3_finalize(insertParticipant)

		for _, participant := range participants {
			if err := bindText(insertParticipant, 1, schedule.ID); err != nil {
				return err
			}
			if err := bindText(insertParticipant, 2, participant); err != nil {
				return err
			}
			if err := stepDone(insertParticipant); err != nil {
				return err
			}
			C.sqlite3_reset(insertParticipant)
			C.sqlite3_clear_bindings(insertParticipant)
		}
	}

	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}

	committed = true
	schedule.CreatorID = creatorID
	schedule.CreatedAt = createdAt
	return nil
}

// GetSchedule retrieves a schedule by ID.
func (s *Storage) GetSchedule(ctx context.Context, id string) (persistence.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	schedule, err := s.fetchScheduleLocked(id)
	if errors.Is(err, persistence.ErrNotFound) {
		return persistence.Schedule{}, err
	}
	if err != nil {
		return persistence.Schedule{}, err
	}
	return schedule, nil
}

// ListSchedules lists schedules filtered by the provided filter.
func (s *Storage) ListSchedules(ctx context.Context, filter persistence.ScheduleFilter) ([]persistence.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id FROM schedules WHERE 1=1`
	binders := make([]func(*C.sqlite3_stmt, int) error, 0)

	if filter.StartsAfter != nil {
		query += ` AND end_at > ?`
		value := filter.StartsAfter.UTC()
		binders = append(binders, func(stmt *C.sqlite3_stmt, idx int) error {
			return bindTime(stmt, idx, value)
		})
	}
	if filter.EndsBefore != nil {
		query += ` AND start_at < ?`
		value := filter.EndsBefore.UTC()
		binders = append(binders, func(stmt *C.sqlite3_stmt, idx int) error {
			return bindTime(stmt, idx, value)
		})
	}
	if len(filter.ParticipantIDs) > 0 {
		placeholders := make([]string, len(filter.ParticipantIDs))
		for i := range filter.ParticipantIDs {
			placeholders[i] = "?"
		}
		query += fmt.Sprintf(` AND id IN (SELECT DISTINCT schedule_id FROM schedule_participants WHERE participant_id IN (%s))`, strings.Join(placeholders, ","))
		for _, participant := range filter.ParticipantIDs {
			participantValue := participant
			binders = append(binders, func(stmt *C.sqlite3_stmt, idx int) error {
				return bindText(stmt, idx, participantValue)
			})
		}
	}

	query += ` ORDER BY start_at ASC, id ASC`

	stmt, err := s.prepareLocked(query)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)

	if len(binders) > 0 {
		paramCount := int(C.sqlite3_bind_parameter_count(stmt))
		if paramCount != len(binders) {
			return nil, fmt.Errorf("sqlite: binder count mismatch: have %d placeholders expected %d", len(binders), paramCount)
		}
		if err := bindArgsSequential(stmt, binders); err != nil {
			return nil, err
		}
	}

	ids := make([]string, 0)
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, fmt.Errorf("sqlite: list schedules: %s", s.lastErrorLocked())
		}
		ids = append(ids, columnText(stmt, 0))
	}

	schedules := make([]persistence.Schedule, 0, len(ids))
	for _, scheduleID := range ids {
		schedule, err := s.fetchScheduleLocked(scheduleID)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// DeleteSchedule removes a schedule by ID.
func (s *Storage) DeleteSchedule(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`DELETE FROM schedules WHERE id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, id); err != nil {
		return err
	}

	if err := stepDone(stmt); err != nil {
		return err
	}
	if C.sqlite3_changes(s.db) == 0 {
		return persistence.ErrNotFound
	}
	return nil
}

// UpsertRecurrence creates or updates a recurrence rule.
func (s *Storage) UpsertRecurrence(ctx context.Context, rule persistence.RecurrenceRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	createdAt := rule.CreatedAt
	selectStmt, err := s.prepareLocked(`SELECT created_at FROM recurrences WHERE id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(selectStmt)

	if err := bindText(selectStmt, 1, rule.ID); err != nil {
		return err
	}
	rc := C.sqlite3_step(selectStmt)
	if rc == C.SQLITE_ROW {
		existingCreatedAt, err := columnTime(selectStmt, 0)
		if err != nil {
			return err
		}
		createdAt = existingCreatedAt
	} else if rc != C.SQLITE_DONE {
		return fmt.Errorf("sqlite: select recurrence: %s", s.lastErrorLocked())
	}

	insertStmt, err := s.prepareLocked(`INSERT INTO recurrences(id, schedule_id, frequency, weekdays, starts_on, ends_on, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(id) DO UPDATE SET schedule_id = excluded.schedule_id, frequency = excluded.frequency, weekdays = excluded.weekdays, starts_on = excluded.starts_on, ends_on = excluded.ends_on, updated_at = excluded.updated_at`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(insertStmt)

	if err := bindText(insertStmt, 1, rule.ID); err != nil {
		return err
	}
	if err := bindText(insertStmt, 2, rule.ScheduleID); err != nil {
		return err
	}
	if err := bindInt(insertStmt, 3, rule.Frequency); err != nil {
		return err
	}
	if err := bindInt64(insertStmt, 4, encodeWeekdays(rule.Weekdays)); err != nil {
		return err
	}
	if err := bindTime(insertStmt, 5, rule.StartsOn); err != nil {
		return err
	}
	if err := bindOptionalTime(insertStmt, 6, rule.EndsOn); err != nil {
		return err
	}
	if err := bindTime(insertStmt, 7, createdAt); err != nil {
		return err
	}
	if err := bindTime(insertStmt, 8, rule.UpdatedAt); err != nil {
		return err
	}

	if err := stepDone(insertStmt); err != nil {
		return err
	}

	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// ListRecurrencesForSchedule lists recurrence rules for a schedule ordered by creation time.
func (s *Storage) ListRecurrencesForSchedule(ctx context.Context, scheduleID string) ([]persistence.RecurrenceRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`SELECT id, schedule_id, frequency, weekdays, starts_on, ends_on, created_at, updated_at FROM recurrences WHERE schedule_id = ? ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, scheduleID); err != nil {
		return nil, err
	}

	rules := make([]persistence.RecurrenceRule, 0)
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, fmt.Errorf("sqlite: list recurrences: %s", s.lastErrorLocked())
		}
		rule, err := rowToRecurrence(stmt)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// DeleteRecurrence deletes a recurrence by ID.
func (s *Storage) DeleteRecurrence(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`DELETE FROM recurrences WHERE id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, id); err != nil {
		return err
	}
	if err := stepDone(stmt); err != nil {
		return err
	}
	if C.sqlite3_changes(s.db) == 0 {
		return persistence.ErrNotFound
	}
	return nil
}

// DeleteRecurrencesForSchedule deletes recurrences for a schedule.
func (s *Storage) DeleteRecurrencesForSchedule(ctx context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`DELETE FROM recurrences WHERE schedule_id = ?`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)

	if err := bindText(stmt, 1, scheduleID); err != nil {
		return err
	}
	if err := stepDone(stmt); err != nil {
		return err
	}
	return nil
}

// --- Helper functions ---

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

func (s *Storage) execSimple(sql string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execLocked(sql)
}

func (s *Storage) execLocked(sql string) error {
	csql := C.CString(sql)
	defer C.free(unsafe.Pointer(csql))
	var errMsg *C.char
	if rc := C.sqlite3_exec(s.db, csql, nil, nil, &errMsg); rc != C.SQLITE_OK {
		defer C.sqlite3_free(unsafe.Pointer(errMsg))
		return fmt.Errorf("sqlite: exec: %s", C.GoString(errMsg))
	}
	return nil
}

func (s *Storage) prepareLocked(query string) (*C.sqlite3_stmt, error) {
	cquery := C.CString(query)
	defer C.free(unsafe.Pointer(cquery))
	var stmt *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(s.db, cquery, -1, &stmt, nil); rc != C.SQLITE_OK {
		return nil, fmt.Errorf("sqlite: prepare: %s", s.lastErrorLocked())
	}
	return stmt, nil
}

func bindText(stmt *C.sqlite3_stmt, index int, value string) error {
	cvalue := C.CString(value)
	defer C.free(unsafe.Pointer(cvalue))
	if rc := C.bind_text(stmt, C.int(index), cvalue); rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite: bind text: %s", C.GoString(C.sqlite3_errstr(rc)))
	}
	return nil
}

func bindOptionalText(stmt *C.sqlite3_stmt, index int, value *string) error {
	if value == nil {
		if rc := C.sqlite3_bind_null(stmt, C.int(index)); rc != C.SQLITE_OK {
			return fmt.Errorf("sqlite: bind null: %s", C.GoString(C.sqlite3_errstr(rc)))
		}
		return nil
	}
	return bindText(stmt, index, *value)
}

func bindBool(stmt *C.sqlite3_stmt, index int, value bool) error {
	var intValue C.int
	if value {
		intValue = 1
	}
	if rc := C.sqlite3_bind_int(stmt, C.int(index), intValue); rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite: bind bool: %s", C.GoString(C.sqlite3_errstr(rc)))
	}
	return nil
}

func bindInt(stmt *C.sqlite3_stmt, index int, value int) error {
	if rc := C.sqlite3_bind_int(stmt, C.int(index), C.int(value)); rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite: bind int: %s", C.GoString(C.sqlite3_errstr(rc)))
	}
	return nil
}

func bindInt64(stmt *C.sqlite3_stmt, index int, value int64) error {
	if rc := C.sqlite3_bind_int64(stmt, C.int(index), C.sqlite3_int64(value)); rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite: bind int64: %s", C.GoString(C.sqlite3_errstr(rc)))
	}
	return nil
}

func bindTime(stmt *C.sqlite3_stmt, index int, value time.Time) error {
	return bindText(stmt, index, value.UTC().Format(timeLayout))
}

func bindOptionalTime(stmt *C.sqlite3_stmt, index int, value *time.Time) error {
	if value == nil {
		if rc := C.sqlite3_bind_null(stmt, C.int(index)); rc != C.SQLITE_OK {
			return fmt.Errorf("sqlite: bind null: %s", C.GoString(C.sqlite3_errstr(rc)))
		}
		return nil
	}
	return bindTime(stmt, index, value.UTC())
}

func stepDone(stmt *C.sqlite3_stmt) error {
	rc := C.sqlite3_step(stmt)
	if rc != C.SQLITE_DONE {
		return fmt.Errorf("sqlite: step: %s", C.GoString(C.sqlite3_errmsg(C.sqlite3_db_handle(stmt))))
	}
	return nil
}

func scanUser(stmt *C.sqlite3_stmt) (persistence.User, error) {
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return persistence.User{}, errNoRows
	}
	if rc != C.SQLITE_ROW {
		return persistence.User{}, fmt.Errorf("sqlite: get user: %s", C.GoString(C.sqlite3_errmsg(C.sqlite3_db_handle(stmt))))
	}
	return rowToUser(stmt)
}

func rowToUser(stmt *C.sqlite3_stmt) (persistence.User, error) {
	createdAt, err := columnTime(stmt, 4)
	if err != nil {
		return persistence.User{}, err
	}
	updatedAt, err := columnTime(stmt, 5)
	if err != nil {
		return persistence.User{}, err
	}
	return persistence.User{
		ID:          columnText(stmt, 0),
		Email:       columnText(stmt, 1),
		DisplayName: columnText(stmt, 2),
		IsAdmin:     columnBool(stmt, 3),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func scanRoom(stmt *C.sqlite3_stmt) (persistence.Room, error) {
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return persistence.Room{}, errNoRows
	}
	if rc != C.SQLITE_ROW {
		return persistence.Room{}, fmt.Errorf("sqlite: get room: %s", C.GoString(C.sqlite3_errmsg(C.sqlite3_db_handle(stmt))))
	}
	return rowToRoom(stmt)
}

func rowToRoom(stmt *C.sqlite3_stmt) (persistence.Room, error) {
	createdAt, err := columnTime(stmt, 5)
	if err != nil {
		return persistence.Room{}, err
	}
	updatedAt, err := columnTime(stmt, 6)
	if err != nil {
		return persistence.Room{}, err
	}
	return persistence.Room{
		ID:         columnText(stmt, 0),
		Name:       columnText(stmt, 1),
		Location:   columnText(stmt, 2),
		Capacity:   int(C.sqlite3_column_int(stmt, 3)),
		Facilities: columnNullableText(stmt, 4),
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}, nil
}

func rowToRecurrence(stmt *C.sqlite3_stmt) (persistence.RecurrenceRule, error) {
	weekdays := int64(C.sqlite3_column_int64(stmt, 3))
	startsOn, err := columnTime(stmt, 4)
	if err != nil {
		return persistence.RecurrenceRule{}, err
	}
	createdAt, err := columnTime(stmt, 6)
	if err != nil {
		return persistence.RecurrenceRule{}, err
	}
	updatedAt, err := columnTime(stmt, 7)
	if err != nil {
		return persistence.RecurrenceRule{}, err
	}
	endsOn, err := columnNullableTime(stmt, 5)
	if err != nil {
		return persistence.RecurrenceRule{}, err
	}
	return persistence.RecurrenceRule{
		ID:         columnText(stmt, 0),
		ScheduleID: columnText(stmt, 1),
		Frequency:  int(C.sqlite3_column_int(stmt, 2)),
		Weekdays:   decodeWeekdays(weekdays),
		StartsOn:   startsOn,
		EndsOn:     endsOn,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}, nil
}

func (s *Storage) fetchScheduleLocked(id string) (persistence.Schedule, error) {
	scheduleStmt, err := s.prepareLocked(`SELECT id, title, start_at, end_at, creator_id, memo, room_id, web_conference_url, created_at, updated_at FROM schedules WHERE id = ?`)
	if err != nil {
		return persistence.Schedule{}, err
	}
	defer C.sqlite3_finalize(scheduleStmt)

	if err := bindText(scheduleStmt, 1, id); err != nil {
		return persistence.Schedule{}, err
	}

	rc := C.sqlite3_step(scheduleStmt)
	if rc == C.SQLITE_DONE {
		return persistence.Schedule{}, persistence.ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return persistence.Schedule{}, fmt.Errorf("sqlite: get schedule: %s", s.lastErrorLocked())
	}

	start, err := columnTime(scheduleStmt, 2)
	if err != nil {
		return persistence.Schedule{}, err
	}
	end, err := columnTime(scheduleStmt, 3)
	if err != nil {
		return persistence.Schedule{}, err
	}
	createdAt, err := columnTime(scheduleStmt, 8)
	if err != nil {
		return persistence.Schedule{}, err
	}
	updatedAt, err := columnTime(scheduleStmt, 9)
	if err != nil {
		return persistence.Schedule{}, err
	}

	schedule := persistence.Schedule{
		ID:               columnText(scheduleStmt, 0),
		Title:            columnText(scheduleStmt, 1),
		Start:            start,
		End:              end,
		CreatorID:        columnText(scheduleStmt, 4),
		Memo:             columnNullableText(scheduleStmt, 5),
		RoomID:           columnNullableText(scheduleStmt, 6),
		WebConferenceURL: columnNullableText(scheduleStmt, 7),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}

	participantsStmt, err := s.prepareLocked(`SELECT participant_id FROM schedule_participants WHERE schedule_id = ? ORDER BY participant_id ASC`)
	if err != nil {
		return persistence.Schedule{}, err
	}
	defer C.sqlite3_finalize(participantsStmt)

	if err := bindText(participantsStmt, 1, id); err != nil {
		return persistence.Schedule{}, err
	}

	participants := make([]string, 0)
	for {
		rc := C.sqlite3_step(participantsStmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return persistence.Schedule{}, fmt.Errorf("sqlite: list participants: %s", s.lastErrorLocked())
		}
		participants = append(participants, columnText(participantsStmt, 0))
	}
	schedule.Participants = participants

	return schedule, nil
}

func columnText(stmt *C.sqlite3_stmt, index int) string {
	text := C.sqlite3_column_text(stmt, C.int(index))
	if text == nil {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(text)))
}

func columnNullableText(stmt *C.sqlite3_stmt, index int) *string {
	if C.sqlite3_column_type(stmt, C.int(index)) == C.SQLITE_NULL {
		return nil
	}
	value := columnText(stmt, index)
	return &value
}

func columnBool(stmt *C.sqlite3_stmt, index int) bool {
	return C.sqlite3_column_int(stmt, C.int(index)) != 0
}

func columnTime(stmt *C.sqlite3_stmt, index int) (time.Time, error) {
	raw := columnText(stmt, index)
	parsed, err := time.Parse(timeLayout, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("sqlite: parse time %q: %w", raw, err)
	}
	return parsed, nil
}

func columnNullableTime(stmt *C.sqlite3_stmt, index int) (*time.Time, error) {
	if C.sqlite3_column_type(stmt, C.int(index)) == C.SQLITE_NULL {
		return nil, nil
	}
	value, err := columnTime(stmt, index)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func bindArgsSequential(stmt *C.sqlite3_stmt, binders []func(*C.sqlite3_stmt, int) error) error {
	for i, binder := range binders {
		if err := binder(stmt, i+1); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) lastErrorLocked() string {
	return C.GoString(C.sqlite3_errmsg(s.db))
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
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
