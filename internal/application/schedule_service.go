package application

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
	"github.com/example/enterprise-scheduler/internal/scheduler"
)

// ScheduleRepository captures the persistence interactions needed by the service.
type ScheduleRepository interface {
	CreateSchedule(ctx context.Context, schedule Schedule) (Schedule, error)
	GetSchedule(ctx context.Context, id string) (Schedule, error)
	UpdateSchedule(ctx context.Context, schedule Schedule) (Schedule, error)
	DeleteSchedule(ctx context.Context, id string) error
	ListSchedules(ctx context.Context, filter ScheduleRepositoryFilter) ([]Schedule, error)
}

// ScheduleRepositoryFilter narrows queries issued to the schedule repository.
type ScheduleRepositoryFilter struct {
	ParticipantIDs []string
	StartsAfter    *time.Time
	EndsBefore     *time.Time
}

// UserDirectory exposes user lookup operations.
type UserDirectory interface {
	MissingUserIDs(ctx context.Context, ids []string) ([]string, error)
}

// RoomCatalog exposes room lookup operations.
type RoomCatalog interface {
	RoomExists(ctx context.Context, id string) (bool, error)
}

// RecurrenceRepository exposes recurrence cleanup operations.
type RecurrenceRepository interface {
	DeleteRecurrencesForSchedule(ctx context.Context, scheduleID string) error
}

// ScheduleService orchestrates validation and persistence for schedule operations.
type ScheduleService struct {
	schedules   ScheduleRepository
	users       UserDirectory
	rooms       RoomCatalog
	recurrences RecurrenceRepository
	idGenerator func() string
	now         func() time.Time
}

// NewScheduleService wires dependencies for schedule operations.
func NewScheduleService(schedules ScheduleRepository, users UserDirectory, rooms RoomCatalog, recurrences RecurrenceRepository, idGenerator func() string, now func() time.Time) *ScheduleService {
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
		recurrences: recurrences,
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
		return Schedule{}, nil, mapScheduleRepoError(err)
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
		return Schedule{}, nil, mapScheduleRepoError(err)
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

	cleanupNeeded := needsRecurrenceCleanup(existing, updated)

	warnings, err := s.detectConflicts(ctx, updated)
	if err != nil {
		return Schedule{}, nil, err
	}

	persisted, err := s.schedules.UpdateSchedule(ctx, updated)
	if err != nil {
		return Schedule{}, nil, mapScheduleRepoError(err)
	}

	if cleanupNeeded && s.recurrences != nil {
		if err := s.recurrences.DeleteRecurrencesForSchedule(ctx, persisted.ID); err != nil {
			return Schedule{}, nil, err
		}
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
		return mapScheduleRepoError(err)
	}

	if existing.CreatorID != principal.UserID && !principal.IsAdmin {
		return ErrUnauthorized
	}

	if err := s.schedules.DeleteSchedule(ctx, scheduleID); err != nil {
		return mapScheduleRepoError(err)
	}

	if s.recurrences != nil {
		if err := s.recurrences.DeleteRecurrencesForSchedule(ctx, scheduleID); err != nil {
			return err
		}
	}

	return nil
}

// ListSchedules enumerates schedules visible to the requesting principal.
func (s *ScheduleService) ListSchedules(ctx context.Context, params ListSchedulesParams) ([]Schedule, []ConflictWarning, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("ScheduleService is nil")
	}
	if s.schedules == nil {
		return nil, nil, fmt.Errorf("schedule repository not configured")
	}

	filter := s.buildListFilter(params)

	schedules, err := s.schedules.ListSchedules(ctx, filter)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	ordered := make([]Schedule, len(schedules))
	copy(ordered, schedules)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Start.Equal(ordered[j].Start) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Start.Before(ordered[j].Start)
	})

	warnings := detectListConflicts(ordered)

	return ordered, warnings, nil
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

	schedules, err := s.schedules.ListSchedules(ctx, ScheduleRepositoryFilter{})
	if err != nil {
		if isNotFoundError(err) {
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
	} else if !isJapanStandardTime(input.Start) {
		vErr.add("start", "start must be in Asia/Tokyo (JST)")
	}

	if input.End.IsZero() {
		vErr.add("end", "end is required")
	} else if !isJapanStandardTime(input.End) {
		vErr.add("end", "end must be in Asia/Tokyo (JST)")
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

func needsRecurrenceCleanup(before, after Schedule) bool {
	if !before.Start.Equal(after.Start) || !before.End.Equal(after.End) {
		return true
	}

	beforeParticipants := sortStrings(before.ParticipantIDs)
	afterParticipants := sortStrings(after.ParticipantIDs)
	if !slices.Equal(beforeParticipants, afterParticipants) {
		return true
	}

	return false
}

func sortStrings(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	sort.Strings(out)
	return out
}

func (s *ScheduleService) buildListFilter(params ListSchedulesParams) ScheduleRepositoryFilter {
	participants := make([]string, 0, len(params.ParticipantIDs)+1)
	participants = append(participants, params.ParticipantIDs...)
	if params.Principal.UserID != "" {
		participants = append(participants, params.Principal.UserID)
	}
	participants = sortStrings(uniqueStrings(participants))
	if len(participants) == 0 {
		participants = nil
	}

	startsAfter := params.StartsAfter
	endsBefore := params.EndsBefore

	if params.Period != ListPeriodNone {
		start, end := computePeriodRange(params.Period, params.PeriodReference)
		if startsAfter == nil {
			startsAfter = &start
		}
		if endsBefore == nil {
			endsBefore = &end
		}
	}

	return ScheduleRepositoryFilter{
		ParticipantIDs: participants,
		StartsAfter:    startsAfter,
		EndsBefore:     endsBefore,
	}
}

func computePeriodRange(period ListPeriod, reference time.Time) (time.Time, time.Time) {
	switch period {
	case ListPeriodDay:
		start := startOfDay(reference)
		return start, start.AddDate(0, 0, 1)
	case ListPeriodWeek:
		start := startOfWeek(reference)
		return start, start.AddDate(0, 0, 7)
	case ListPeriodMonth:
		start := startOfMonth(reference)
		return start, start.AddDate(0, 1, 0)
	default:
		return time.Time{}, time.Time{}
	}
}

func startOfDay(t time.Time) time.Time {
	loc := jstLocation()
	inJST := t.In(loc)
	return time.Date(inJST.Year(), inJST.Month(), inJST.Day(), 0, 0, 0, 0, loc)
}

func startOfWeek(t time.Time) time.Time {
	start := startOfDay(t)
	weekday := int(start.Weekday())
	// Adjust so Monday is start of week. In Go, Monday == 1, Sunday == 0.
	offset := (weekday + 6) % 7
	return start.AddDate(0, 0, -offset)
}

func startOfMonth(t time.Time) time.Time {
	start := startOfDay(t)
	return time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
}

func jstLocation() *time.Location {
	return time.FixedZone("JST", 9*60*60)
}

func isJapanStandardTime(t time.Time) bool {
	if t.IsZero() {
		return false
	}
	name, offset := t.Zone()
	if offset != 9*60*60 {
		return false
	}
	if name == "Asia/Tokyo" || name == "JST" {
		return true
	}
	if loc := t.Location(); loc != nil {
		if loc.String() == "Asia/Tokyo" || loc.String() == "JST" {
			return true
		}
	}
	return false
}

func detectListConflicts(schedules []Schedule) []ConflictWarning {
	if len(schedules) <= 1 {
		return nil
	}

	warnings := make([]ConflictWarning, 0)
	converted := make([]scheduler.Schedule, len(schedules))
	for i, sched := range schedules {
		converted[i] = toSchedulerSchedule(sched)
	}

	for i, candidate := range schedules {
		if i+1 >= len(schedules) {
			break
		}
		existing := converted[i+1:]
		conflicts := scheduler.DetectConflicts(existing, toSchedulerSchedule(candidate))
		warnings = append(warnings, toConflictWarnings(conflicts)...)
	}

	if len(warnings) == 0 {
		return nil
	}

	return warnings
}

func mapScheduleRepoError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, persistence.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, persistence.ErrDuplicate) {
		return ErrAlreadyExists
	}
	if errors.Is(err, persistence.ErrConstraintViolation) {
		vErr := &ValidationError{}
		vErr.add("time", "start must be before end")
		return vErr
	}
	if errors.Is(err, persistence.ErrForeignKeyViolation) {
		vErr := &ValidationError{}
		vErr.add("participants", "related records are missing")
		return vErr
	}
	return err
}

func isNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, persistence.ErrNotFound)
}
