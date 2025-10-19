package application

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/scheduler"
)

// ScheduleRepository captures the persistence interactions needed by the service.
type ScheduleRepository interface {
	CreateSchedule(ctx context.Context, schedule Schedule) (Schedule, error)
	GetSchedule(ctx context.Context, id string) (Schedule, error)
	UpdateSchedule(ctx context.Context, schedule Schedule) (Schedule, error)
	DeleteSchedule(ctx context.Context, id string) error
	ListSchedules(ctx context.Context) ([]Schedule, error)
}

// UserDirectory exposes user lookup operations.
type UserDirectory interface {
	MissingUserIDs(ctx context.Context, ids []string) ([]string, error)
}

// RoomCatalog exposes room lookup operations.
type RoomCatalog interface {
	RoomExists(ctx context.Context, id string) (bool, error)
}

// ScheduleService orchestrates validation and persistence for schedule operations.
type ScheduleService struct {
	schedules   ScheduleRepository
	users       UserDirectory
	rooms       RoomCatalog
	idGenerator func() string
	now         func() time.Time
}

// NewScheduleService wires dependencies for schedule operations.
func NewScheduleService(schedules ScheduleRepository, users UserDirectory, rooms RoomCatalog, idGenerator func() string, now func() time.Time) *ScheduleService {
	if idGenerator == nil {
		idGenerator = func() string { return "" }
	}
	if now == nil {
		now = time.Now
	}
	return &ScheduleService{
		schedules:   schedules,
		users:       users,
		rooms:       rooms,
		idGenerator: idGenerator,
		now:         now,
	}
}

// CreateSchedule validates the request before delegating to persistence.
func (s *ScheduleService) CreateSchedule(ctx context.Context, params CreateScheduleParams) (Schedule, []ConflictWarning, error) {
	if s == nil {
		return Schedule{}, nil, fmt.Errorf("ScheduleService is nil")
	}
	input := params.Input
	principal := params.Principal

	if input.CreatorID == "" {
		input.CreatorID = principal.UserID
	}

	if input.CreatorID != principal.UserID && !principal.IsAdmin {
		return Schedule{}, nil, ErrUnauthorized
	}

	vErr := &ValidationError{}

	validateScheduleCore(input, vErr)

	if vErr.HasErrors() {
		return Schedule{}, nil, vErr
	}

	if err := s.ensureParticipantsExist(ctx, append(uniqueStrings(input.ParticipantIDs), input.CreatorID)); err != nil {
		var inner *ValidationError
		if errors.As(err, &inner) {
			return Schedule{}, nil, err
		}
		return Schedule{}, nil, err
	}

	if err := s.ensureRoomExists(ctx, input.RoomID); err != nil {
		return Schedule{}, nil, err
	}

	createdAt := s.now()
	schedule := Schedule{
		ID:               s.idGenerator(),
		CreatorID:        input.CreatorID,
		Title:            strings.TrimSpace(input.Title),
		Description:      input.Description,
		Start:            input.Start,
		End:              input.End,
		RoomID:           input.RoomID,
		WebConferenceURL: input.WebConferenceURL,
		ParticipantIDs:   sortStrings(uniqueStrings(input.ParticipantIDs)),
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	}

	if s.schedules == nil {
		return schedule, nil, nil
	}

	warnings, err := s.detectConflicts(ctx, schedule)
	if err != nil {
		return Schedule{}, nil, err
	}

	persisted, err := s.schedules.CreateSchedule(ctx, schedule)
	if err != nil {
		return Schedule{}, nil, err
	}

	return persisted, warnings, nil
}

// UpdateSchedule applies validation and authorization before updating persistence state.
func (s *ScheduleService) UpdateSchedule(ctx context.Context, params UpdateScheduleParams) (Schedule, []ConflictWarning, error) {
	if s == nil {
		return Schedule{}, nil, fmt.Errorf("ScheduleService is nil")
	}
	if s.schedules == nil {
		return Schedule{}, nil, fmt.Errorf("schedule repository not configured")
	}

	existing, err := s.schedules.GetSchedule(ctx, params.ScheduleID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Schedule{}, nil, ErrNotFound
		}
		return Schedule{}, nil, err
	}

	principal := params.Principal
	input := params.Input

	if existing.CreatorID != principal.UserID && !principal.IsAdmin {
		return Schedule{}, nil, ErrUnauthorized
	}

	vErr := &ValidationError{}

	if input.CreatorID != "" && input.CreatorID != existing.CreatorID {
		vErr.add("creator_id", "creator cannot be changed")
	}

	validateScheduleCore(input, vErr)

	if vErr.HasErrors() {
		return Schedule{}, nil, vErr
	}

	if err := s.ensureParticipantsExist(ctx, append(uniqueStrings(input.ParticipantIDs), existing.CreatorID)); err != nil {
		var inner *ValidationError
		if errors.As(err, &inner) {
			return Schedule{}, nil, err
		}
		return Schedule{}, nil, err
	}

	if err := s.ensureRoomExists(ctx, input.RoomID); err != nil {
		return Schedule{}, nil, err
	}

	updated := existing
	updated.Title = strings.TrimSpace(input.Title)
	updated.Description = input.Description
	updated.Start = input.Start
	updated.End = input.End
	updated.RoomID = input.RoomID
	updated.WebConferenceURL = input.WebConferenceURL
	updated.ParticipantIDs = sortStrings(uniqueStrings(input.ParticipantIDs))
	updated.UpdatedAt = s.now()

	warnings, err := s.detectConflicts(ctx, updated)
	if err != nil {
		return Schedule{}, nil, err
	}

	persisted, err := s.schedules.UpdateSchedule(ctx, updated)
	if err != nil {
		return Schedule{}, nil, err
	}

	return persisted, warnings, nil
}

// DeleteSchedule ensures authorization before delegating to persistence.
func (s *ScheduleService) DeleteSchedule(ctx context.Context, principal Principal, scheduleID string) error {
	if s == nil {
		return fmt.Errorf("ScheduleService is nil")
	}
	if s.schedules == nil {
		return fmt.Errorf("schedule repository not configured")
	}

	existing, err := s.schedules.GetSchedule(ctx, scheduleID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if existing.CreatorID != principal.UserID && !principal.IsAdmin {
		return ErrUnauthorized
	}

	return s.schedules.DeleteSchedule(ctx, scheduleID)
}

func (s *ScheduleService) ensureParticipantsExist(ctx context.Context, ids []string) error {
	if s.users == nil {
		return nil
	}
	ids = uniqueStrings(ids)
	missing, err := s.users.MissingUserIDs(ctx, ids)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}
	vErr := &ValidationError{}
	vErr.add("participants", fmt.Sprintf("unknown user ids: %s", strings.Join(missing, ", ")))
	return vErr
}

func (s *ScheduleService) ensureRoomExists(ctx context.Context, roomID *string) error {
	if roomID == nil || s.rooms == nil {
		return nil
	}
	exists, err := s.rooms.RoomExists(ctx, *roomID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	vErr := &ValidationError{}
	vErr.add("room_id", "room does not exist")
	return vErr
}

func (s *ScheduleService) detectConflicts(ctx context.Context, candidate Schedule) ([]ConflictWarning, error) {
	if s == nil || s.schedules == nil {
		return nil, nil
	}

	schedules, err := s.schedules.ListSchedules(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	existing := make([]scheduler.Schedule, 0, len(schedules))
	for _, sched := range schedules {
		existing = append(existing, toSchedulerSchedule(sched))
	}

	conflicts := scheduler.DetectConflicts(existing, toSchedulerSchedule(candidate))
	return toConflictWarnings(conflicts), nil
}

func toSchedulerSchedule(schedule Schedule) scheduler.Schedule {
	participants := make([]string, len(schedule.ParticipantIDs))
	copy(participants, schedule.ParticipantIDs)

	return scheduler.Schedule{
		ID:           schedule.ID,
		Participants: participants,
		RoomID:       schedule.RoomID,
		Start:        schedule.Start,
		End:          schedule.End,
	}
}

func toConflictWarnings(conflicts []scheduler.Conflict) []ConflictWarning {
	if len(conflicts) == 0 {
		return nil
	}

	warnings := make([]ConflictWarning, 0, len(conflicts))
	for _, conflict := range conflicts {
		warning := ConflictWarning{
			ScheduleID: conflict.WithScheduleID,
			Type:       string(conflict.Type),
		}
		if conflict.Participant != "" {
			warning.ParticipantID = conflict.Participant
		}
		if conflict.RoomID != nil {
			roomID := *conflict.RoomID
			warning.RoomID = &roomID
		}
		warnings = append(warnings, warning)
	}
	return warnings
}

func validateScheduleCore(input ScheduleInput, vErr *ValidationError) {
	if strings.TrimSpace(input.Title) == "" {
		vErr.add("title", "title is required")
	}

	if input.Start.IsZero() {
		vErr.add("start", "start is required")
	}

	if input.End.IsZero() {
		vErr.add("end", "end is required")
	}

	if !input.Start.IsZero() && !input.End.IsZero() && !input.Start.Before(input.End) {
		vErr.add("time", "start must be before end")
	}

	if input.WebConferenceURL != "" {
		if _, err := url.ParseRequestURI(input.WebConferenceURL); err != nil {
			vErr.add("web_conference_url", "must be a valid URL")
		}
	}

	if len(input.ParticipantIDs) == 0 {
		vErr.add("participants", "at least one participant is required")
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sortStrings(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	sort.Strings(out)
	return out
}
