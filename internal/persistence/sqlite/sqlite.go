package sqlite

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

// Storage provides an in-memory SQLite-like persistence layer implementation.
type Storage struct {
	mu          sync.RWMutex
	users       map[string]persistence.User
	rooms       map[string]persistence.Room
	schedules   map[string]persistence.Schedule
	recurrences map[string]persistence.RecurrenceRule
}

// Open returns a new Storage instance. The dsn is accepted for API compatibility.
func Open(_ string) (*Storage, error) {
	return &Storage{
		users:       make(map[string]persistence.User),
		rooms:       make(map[string]persistence.Room),
		schedules:   make(map[string]persistence.Schedule),
		recurrences: make(map[string]persistence.RecurrenceRule),
	}, nil
}

// Close releases resources held by the storage. No-op for the in-memory implementation.
func (s *Storage) Close() error {
	return nil
}

// Migrate initialises the storage. No-op for the in-memory implementation.
func (s *Storage) Migrate(context.Context) error {
	return nil
}

// --- UserRepository implementation ---

// CreateUser stores a new user.
func (s *Storage) CreateUser(ctx context.Context, user persistence.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[user.ID]; ok {
		return fmt.Errorf("sqlite: user %s already exists", user.ID)
	}

	if err := s.ensureUniqueEmailLocked(user.ID, user.Email); err != nil {
		return err
	}

	s.users[user.ID] = cloneUser(user)
	return nil
}

// UpdateUser updates an existing user.
func (s *Storage) UpdateUser(ctx context.Context, user persistence.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[user.ID]; !ok {
		return persistence.ErrNotFound
	}

	if err := s.ensureUniqueEmailLocked(user.ID, user.Email); err != nil {
		return err
	}

	s.users[user.ID] = cloneUser(user)
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

	return cloneUser(user), nil
}

// GetUserByEmail retrieves a user by email address.
func (s *Storage) GetUserByEmail(ctx context.Context, email string) (persistence.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(email)
	for _, user := range s.users {
		if strings.ToLower(user.Email) == lower {
			return cloneUser(user), nil
		}
	}

	return persistence.User{}, persistence.ErrNotFound
}

// ListUsers returns all users ordered by CreatedAt ascending.
func (s *Storage) ListUsers(ctx context.Context) ([]persistence.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]persistence.User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, cloneUser(user))
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

	if _, ok := s.users[id]; !ok {
		return persistence.ErrNotFound
	}

	delete(s.users, id)

	// Remove user from participants if present.
	for scheduleID, sched := range s.schedules {
		updatedParticipants := removeString(sched.Participants, id)
		if len(updatedParticipants) != len(sched.Participants) {
			sched.Participants = updatedParticipants
			s.schedules[scheduleID] = sched
		}
	}

	return nil
}

func (s *Storage) ensureUniqueEmailLocked(id, email string) error {
	lower := strings.ToLower(email)
	for existingID, user := range s.users {
		if existingID == id {
			continue
		}
		if strings.ToLower(user.Email) == lower {
			return fmt.Errorf("sqlite: email %s already exists", email)
		}
	}
	return nil
}

// --- RoomRepository implementation ---

// CreateRoom stores a new meeting room.
func (s *Storage) CreateRoom(ctx context.Context, room persistence.Room) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[room.ID]; ok {
		return fmt.Errorf("sqlite: room %s already exists", room.ID)
	}

	s.rooms[room.ID] = cloneRoom(room)
	return nil
}

// UpdateRoom updates an existing meeting room.
func (s *Storage) UpdateRoom(ctx context.Context, room persistence.Room) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[room.ID]; !ok {
		return persistence.ErrNotFound
	}

	s.rooms[room.ID] = cloneRoom(room)
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

	return cloneRoom(room), nil
}

// ListRooms returns all rooms ordered by name.
func (s *Storage) ListRooms(ctx context.Context) ([]persistence.Room, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rooms := make([]persistence.Room, 0, len(s.rooms))
	for _, room := range s.rooms {
		rooms = append(rooms, cloneRoom(room))
	}

	sort.Slice(rooms, func(i, j int) bool {
		if rooms[i].Name == rooms[j].Name {
			return rooms[i].ID < rooms[j].ID
		}
		return rooms[i].Name < rooms[j].Name
	})

	return rooms, nil
}

// DeleteRoom removes a room by ID and clears the association from schedules.
func (s *Storage) DeleteRoom(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[id]; !ok {
		return persistence.ErrNotFound
	}

	delete(s.rooms, id)

	for scheduleID, sched := range s.schedules {
		if sched.RoomID != nil && *sched.RoomID == id {
			sched.RoomID = nil
			s.schedules[scheduleID] = sched
		}
	}

	return nil
}

// --- ScheduleRepository implementation ---

// CreateSchedule stores a new schedule with its participants.
func (s *Storage) CreateSchedule(ctx context.Context, schedule persistence.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[schedule.ID]; ok {
		return fmt.Errorf("sqlite: schedule %s already exists", schedule.ID)
	}

	if _, ok := s.users[schedule.CreatorID]; !ok {
		return fmt.Errorf("sqlite: creator %s does not exist", schedule.CreatorID)
	}

	for _, participant := range schedule.Participants {
		if _, ok := s.users[participant]; !ok {
			return fmt.Errorf("sqlite: participant %s does not exist", participant)
		}
	}

	if schedule.RoomID != nil {
		if _, ok := s.rooms[*schedule.RoomID]; !ok {
			return fmt.Errorf("sqlite: room %s does not exist", *schedule.RoomID)
		}
	}

	schedule.Participants = uniqueStrings(schedule.Participants)
	s.schedules[schedule.ID] = cloneSchedule(schedule)
	return nil
}

// UpdateSchedule updates an existing schedule and its participants.
func (s *Storage) UpdateSchedule(ctx context.Context, schedule persistence.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[schedule.ID]; !ok {
		return persistence.ErrNotFound
	}

	for _, participant := range schedule.Participants {
		if _, ok := s.users[participant]; !ok {
			return fmt.Errorf("sqlite: participant %s does not exist", participant)
		}
	}

	if schedule.RoomID != nil {
		if _, ok := s.rooms[*schedule.RoomID]; !ok {
			return fmt.Errorf("sqlite: room %s does not exist", *schedule.RoomID)
		}
	}

	schedule.Participants = uniqueStrings(schedule.Participants)
	existing := s.schedules[schedule.ID]
	schedule.CreatorID = existing.CreatorID
	schedule.CreatedAt = existing.CreatedAt
	s.schedules[schedule.ID] = cloneSchedule(schedule)
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

	return cloneSchedule(schedule), nil
}

// ListSchedules returns schedules filtered by participants and time.
func (s *Storage) ListSchedules(ctx context.Context, filter persistence.ScheduleFilter) ([]persistence.Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedules := make([]persistence.Schedule, 0)
	for _, schedule := range s.schedules {
		if !matchesScheduleFilter(schedule, filter) {
			continue
		}
		schedules = append(schedules, cloneSchedule(schedule))
	}

	sort.Slice(schedules, func(i, j int) bool {
		if schedules[i].Start.Equal(schedules[j].Start) {
			return schedules[i].ID < schedules[j].ID
		}
		return schedules[i].Start.Before(schedules[j].Start)
	})

	return schedules, nil
}

// DeleteSchedule removes a schedule and its recurrences.
func (s *Storage) DeleteSchedule(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[id]; !ok {
		return persistence.ErrNotFound
	}

	delete(s.schedules, id)

	for recurrenceID, rule := range s.recurrences {
		if rule.ScheduleID == id {
			delete(s.recurrences, recurrenceID)
		}
	}

	return nil
}

// --- RecurrenceRepository implementation ---

// UpsertRecurrence creates or updates a recurrence rule.
func (s *Storage) UpsertRecurrence(ctx context.Context, rule persistence.RecurrenceRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[rule.ScheduleID]; !ok {
		return fmt.Errorf("sqlite: schedule %s does not exist", rule.ScheduleID)
	}

	rule.Weekdays = uniqueWeekdays(rule.Weekdays)
	existing, ok := s.recurrences[rule.ID]
	if ok {
		rule.CreatedAt = existing.CreatedAt
	}

	s.recurrences[rule.ID] = cloneRecurrence(rule)
	return nil
}

// ListRecurrencesForSchedule returns recurrence rules for a schedule ordered by CreatedAt.
func (s *Storage) ListRecurrencesForSchedule(ctx context.Context, scheduleID string) ([]persistence.RecurrenceRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rules := make([]persistence.RecurrenceRule, 0)
	for _, rule := range s.recurrences {
		if rule.ScheduleID != scheduleID {
			continue
		}
		rules = append(rules, cloneRecurrence(rule))
	}

	sort.Slice(rules, func(i, j int) bool {
		if rules[i].CreatedAt.Equal(rules[j].CreatedAt) {
			return rules[i].ID < rules[j].ID
		}
		return rules[i].CreatedAt.Before(rules[j].CreatedAt)
	})

	return rules, nil
}

// DeleteRecurrence removes a recurrence rule by ID.
func (s *Storage) DeleteRecurrence(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.recurrences[id]; !ok {
		return persistence.ErrNotFound
	}

	delete(s.recurrences, id)
	return nil
}

// DeleteRecurrencesForSchedule removes all recurrence rules for a schedule.
func (s *Storage) DeleteRecurrencesForSchedule(ctx context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, rule := range s.recurrences {
		if rule.ScheduleID == scheduleID {
			delete(s.recurrences, id)
		}
	}

	return nil
}

// --- Helpers ---

func cloneUser(user persistence.User) persistence.User {
	return persistence.User{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		IsAdmin:     user.IsAdmin,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}

func cloneRoom(room persistence.Room) persistence.Room {
	var facilities *string
	if room.Facilities != nil {
		copy := *room.Facilities
		facilities = &copy
	}

	return persistence.Room{
		ID:         room.ID,
		Name:       room.Name,
		Location:   room.Location,
		Capacity:   room.Capacity,
		Facilities: facilities,
		CreatedAt:  room.CreatedAt,
		UpdatedAt:  room.UpdatedAt,
	}
}

func cloneSchedule(schedule persistence.Schedule) persistence.Schedule {
	var memo *string
	if schedule.Memo != nil {
		copy := *schedule.Memo
		memo = &copy
	}

	var roomID *string
	if schedule.RoomID != nil {
		copy := *schedule.RoomID
		roomID = &copy
	}

	var url *string
	if schedule.WebConferenceURL != nil {
		copy := *schedule.WebConferenceURL
		url = &copy
	}

	participants := make([]string, len(schedule.Participants))
	copy(participants, schedule.Participants)

	return persistence.Schedule{
		ID:               schedule.ID,
		Title:            schedule.Title,
		Start:            schedule.Start,
		End:              schedule.End,
		CreatorID:        schedule.CreatorID,
		Memo:             memo,
		Participants:     participants,
		RoomID:           roomID,
		WebConferenceURL: url,
		CreatedAt:        schedule.CreatedAt,
		UpdatedAt:        schedule.UpdatedAt,
	}
}

func cloneRecurrence(rule persistence.RecurrenceRule) persistence.RecurrenceRule {
	var endsOn *time.Time
	if rule.EndsOn != nil {
		copy := *rule.EndsOn
		endsOn = &copy
	}

	weekdays := make([]time.Weekday, len(rule.Weekdays))
	copy(weekdays, rule.Weekdays)

	return persistence.RecurrenceRule{
		ID:         rule.ID,
		ScheduleID: rule.ScheduleID,
		Frequency:  rule.Frequency,
		Weekdays:   weekdays,
		StartsOn:   rule.StartsOn,
		EndsOn:     endsOn,
		CreatedAt:  rule.CreatedAt,
		UpdatedAt:  rule.UpdatedAt,
	}
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
	return result
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == target {
			continue
		}
		result = append(result, value)
	}
	return result
}

func matchesScheduleFilter(schedule persistence.Schedule, filter persistence.ScheduleFilter) bool {
	if filter.StartsAfter != nil && !schedule.End.After(*filter.StartsAfter) {
		return false
	}

	if filter.EndsBefore != nil && !schedule.Start.Before(*filter.EndsBefore) {
		return false
	}

	if len(filter.ParticipantIDs) > 0 {
		if !intersects(schedule.Participants, filter.ParticipantIDs) {
			return false
		}
	}

	return true
}

func intersects(values []string, targets []string) bool {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	for _, target := range targets {
		if _, ok := set[target]; ok {
			return true
		}
	}
	return false
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

	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	return result
}
