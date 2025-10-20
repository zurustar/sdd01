package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/example/enterprise-scheduler/internal/scheduler"
)

type scheduleRepoStub struct {
	schedule   Schedule
	created    Schedule
	updated    Schedule
	err        error
	deleteErr  error
	list       []Schedule
	listErr    error
	listFilter ScheduleRepositoryFilter
}

func (s *scheduleRepoStub) CreateSchedule(ctx context.Context, schedule Schedule) (Schedule, error) {
	if s.err != nil {
		return Schedule{}, s.err
	}
	s.created = schedule
	return schedule, nil
}

func (s *scheduleRepoStub) GetSchedule(ctx context.Context, id string) (Schedule, error) {
	if s.err != nil {
		return Schedule{}, s.err
	}
	if s.schedule.ID == "" {
		return Schedule{}, ErrNotFound
	}
	return s.schedule, nil
}

func (s *scheduleRepoStub) UpdateSchedule(ctx context.Context, schedule Schedule) (Schedule, error) {
	if s.err != nil {
		return Schedule{}, s.err
	}
	s.updated = schedule
	return schedule, nil
}

func (s *scheduleRepoStub) DeleteSchedule(ctx context.Context, id string) error {
	return s.deleteErr
}

func (s *scheduleRepoStub) ListSchedules(ctx context.Context, filter ScheduleRepositoryFilter) ([]Schedule, error) {
	s.listFilter = filter
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.err != nil {
		return nil, s.err
	}
	if len(s.list) == 0 {
		return nil, nil
	}
	out := make([]Schedule, len(s.list))
	copy(out, s.list)
	return out, nil
}

type filteringScheduleRepo struct {
	schedules []Schedule
}

func (f *filteringScheduleRepo) CreateSchedule(ctx context.Context, schedule Schedule) (Schedule, error) {
	f.schedules = append(f.schedules, schedule)
	return schedule, nil
}

func (f *filteringScheduleRepo) GetSchedule(ctx context.Context, id string) (Schedule, error) {
	for _, sched := range f.schedules {
		if sched.ID == id {
			return sched, nil
		}
	}
	return Schedule{}, ErrNotFound
}

func (f *filteringScheduleRepo) UpdateSchedule(ctx context.Context, schedule Schedule) (Schedule, error) {
	for i, existing := range f.schedules {
		if existing.ID == schedule.ID {
			f.schedules[i] = schedule
			return schedule, nil
		}
	}
	return Schedule{}, ErrNotFound
}

func (f *filteringScheduleRepo) DeleteSchedule(ctx context.Context, id string) error {
	for i, existing := range f.schedules {
		if existing.ID == id {
			f.schedules = append(f.schedules[:i], f.schedules[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (f *filteringScheduleRepo) ListSchedules(ctx context.Context, filter ScheduleRepositoryFilter) ([]Schedule, error) {
	filtered := make([]Schedule, 0, len(f.schedules))
	for _, sched := range f.schedules {
		if matchesScheduleFilter(sched, filter) {
			filtered = append(filtered, sched)
		}
	}
	return filtered, nil
}

func matchesScheduleFilter(schedule Schedule, filter ScheduleRepositoryFilter) bool {
	if filter.StartsAfter != nil && !schedule.End.After(filter.StartsAfter.UTC()) {
		return false
	}
	if filter.EndsBefore != nil && !schedule.Start.Before(filter.EndsBefore.UTC()) {
		return false
	}
	if len(filter.ParticipantIDs) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(schedule.ParticipantIDs)+1)
	for _, participant := range schedule.ParticipantIDs {
		set[participant] = struct{}{}
	}
	if schedule.CreatorID != "" {
		set[schedule.CreatorID] = struct{}{}
	}
	for _, id := range filter.ParticipantIDs {
		if _, ok := set[id]; ok {
			return true
		}
	}
	return false
}

type userDirectoryStub struct {
	missing []string
	err     error
}

func (u *userDirectoryStub) MissingUserIDs(ctx context.Context, ids []string) ([]string, error) {
	if u.err != nil {
		return nil, u.err
	}
	return u.missing, nil
}

type roomCatalogStub struct {
	exists bool
	err    error
}

func (r *roomCatalogStub) RoomExists(ctx context.Context, id string) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	return r.exists, nil
}

type recurrenceRepoStub struct {
	savedRecurrence *RecurrenceInput
	savedScheduleID string
	savedStart      time.Time
	deletedIDs      []string
	err             error
}

func (r *recurrenceRepoStub) SaveRecurrence(ctx context.Context, scheduleID string, start time.Time, recurrence RecurrenceInput) error {
	if r.err != nil {
		return r.err
	}
	r.savedScheduleID = scheduleID
	r.savedStart = start
	r.savedRecurrence = &recurrence
	return nil
}

func (r *recurrenceRepoStub) ListRecurrencesForSchedules(ctx context.Context, scheduleIDs []string) (map[string][]RecurrenceRule, error) {
	return nil, nil
}

func (r *recurrenceRepoStub) DeleteRecurrencesForSchedule(ctx context.Context, scheduleID string) error {
	if r.err != nil {
		return r.err
	}
	r.deletedIDs = append(r.deletedIDs, scheduleID)
	return nil
}

func mustJST(t *testing.T, hour int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("failed to load JST location: %v", err)
	}
	return time.Date(2024, 3, 14, hour, 0, 0, 0, loc)
}

func scheduleIDs(schedules []Schedule) []string {
	ids := make([]string, len(schedules))
	for i, sched := range schedules {
		ids[i] = sched.ID
	}
	return ids
}

func compareStringSlices(got, want []string) string {
	if len(got) != len(want) {
		return fmt.Sprintf("length mismatch: got %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			return fmt.Sprintf("element %d mismatch: got %q want %q", i, got[i], want[i])
		}
	}
	return ""
}

func TestScheduleService_CreateSchedule_ValidatesTemporalBounds(t *testing.T) {
	repo := &scheduleRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() string { return "schedule-1" }, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1"},
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Design sync",
			Start:          mustJST(t, 10),
			End:            mustJST(t, 9),
			ParticipantIDs: []string{"user-1"},
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["time"]; !ok {
		t.Fatalf("expected time validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_CreateSchedule_RequiresAdminForDifferentCreator(t *testing.T) {
	repo := &scheduleRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1", IsAdmin: false},
		Input: ScheduleInput{
			CreatorID:      "user-2",
			Title:          "Planning",
			Start:          mustJST(t, 10),
			End:            mustJST(t, 11),
			ParticipantIDs: []string{"user-2"},
		},
	})

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestScheduleService_CreateSchedule_ValidatesParticipantsExist(t *testing.T) {
	repo := &scheduleRepoStub{}
	users := &userDirectoryStub{missing: []string{"user-2"}}
	svc := NewScheduleService(repo, users, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1", IsAdmin: true},
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Planning",
			Start:          mustJST(t, 10),
			End:            mustJST(t, 11),
			ParticipantIDs: []string{"user-1", "user-2"},
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["participants"]; !ok {
		t.Fatalf("expected participants validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_CreateSchedule_ValidatesRoomExistence(t *testing.T) {
	repo := &scheduleRepoStub{}
	roomID := "room-1"
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: false}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1", IsAdmin: true},
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Planning",
			Start:          mustJST(t, 10),
			End:            mustJST(t, 11),
			RoomID:         &roomID,
			ParticipantIDs: []string{"user-1"},
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["room_id"]; !ok {
		t.Fatalf("expected room_id validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_CreateSchedule_ValidatesRequiredFields(t *testing.T) {
	svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1"},
		Input: ScheduleInput{
			CreatorID: "user-1",
			Start:     mustJST(t, 9),
			End:       mustJST(t, 10),
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["title"]; !ok {
		t.Fatalf("expected title validation error, got %v", vErr.FieldErrors)
	}

	if _, ok := vErr.FieldErrors["participants"]; !ok {
		t.Fatalf("expected participants validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_CreateSchedule_ValidatesWebConferenceURL(t *testing.T) {
	svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1"},
		Input: ScheduleInput{
			CreatorID:        "user-1",
			Title:            "Design sync",
			Start:            mustJST(t, 9),
			End:              mustJST(t, 10),
			ParticipantIDs:   []string{"user-1"},
			WebConferenceURL: "not-a-url",
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["web_conference_url"]; !ok {
		t.Fatalf("expected web_conference_url validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_CreateSchedule_PreventsCreatorSpoofingForRegularUsers(t *testing.T) {
	svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1", IsAdmin: false},
		Input: ScheduleInput{
			CreatorID:      "user-2",
			Title:          "Design sync",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			ParticipantIDs: []string{"user-2"},
		},
	})

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestScheduleService_CreateSchedule_AllowsAdministratorOverrides(t *testing.T) {
	repo := &scheduleRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() string { return "schedule-1" }, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "admin", IsAdmin: true},
		Input: ScheduleInput{
			CreatorID:      "user-2",
			Title:          "Design sync",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			ParticipantIDs: []string{"user-2"},
		},
	})

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if repo.created.CreatorID != "user-2" {
		t.Fatalf("expected creator to remain user-2, got %s", repo.created.CreatorID)
	}
}

func TestScheduleService_CreateSchedule_VerifiesParticipantsExist(t *testing.T) {
	svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{missing: []string{"user-2"}}, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "admin", IsAdmin: true},
		Input: ScheduleInput{
			CreatorID:      "user-2",
			Title:          "Design sync",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			ParticipantIDs: []string{"user-2"},
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["participants"]; !ok {
		t.Fatalf("expected participants validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_CreateSchedule_VerifiesRoomExistence(t *testing.T) {
	roomID := "room-1"
	svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{}, &roomCatalogStub{exists: false}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1"},
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Design sync",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			RoomID:         &roomID,
			ParticipantIDs: []string{"user-1"},
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["room_id"]; !ok {
		t.Fatalf("expected room_id validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_CreateSchedule_AllowsHybridMeetings(t *testing.T) {
	roomID := "room-1"
	repo := &scheduleRepoStub{schedule: Schedule{ID: "schedule-1", CreatorID: "user-1"}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() string { return "schedule-1" }, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1"},
		Input: ScheduleInput{
			CreatorID:        "user-1",
			Title:            "Design sync",
			Start:            mustJST(t, 9),
			End:              mustJST(t, 10),
			RoomID:           &roomID,
			WebConferenceURL: "https://example.com/meet",
			ParticipantIDs:   []string{"user-1"},
		},
	})

	if err != nil {
		t.Fatalf("expected success for hybrid meeting, got %v", err)
	}

	if repo.created.RoomID == nil || *repo.created.RoomID != roomID {
		t.Fatalf("expected room to be persisted, got %v", repo.created.RoomID)
	}

	if repo.created.WebConferenceURL != "https://example.com/meet" {
		t.Fatalf("expected web conference URL to persist, got %s", repo.created.WebConferenceURL)
	}
}

func TestScheduleService_CreateSchedule_ReturnsConflictWarnings(t *testing.T) {
	roomID := "room-1"
	repo := &scheduleRepoStub{
		list: []Schedule{{
			ID:             "schedule-existing",
			CreatorID:      "user-1",
			Title:          "Existing",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			RoomID:         &roomID,
			ParticipantIDs: []string{"user-1", "user-2"},
		}},
	}

	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() string { return "schedule-new" }, func() time.Time { return mustJST(t, 8) })

	created, warnings, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1"},
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Design sync",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			RoomID:         &roomID,
			ParticipantIDs: []string{"user-1"},
		},
	})

	if err != nil {
		t.Fatalf("expected success with warnings, got %v", err)
	}

	if len(warnings) != 2 {
		t.Fatalf("expected two conflict warnings, got %v", warnings)
	}

	warningTypes := map[string]ConflictWarning{}
	for _, warning := range warnings {
		warningTypes[warning.Type] = warning
	}

	participantWarning, ok := warningTypes["participant"]
	if !ok {
		t.Fatalf("expected participant conflict warning, got %v", warnings)
	}
	if participantWarning.ParticipantID != "user-1" {
		t.Fatalf("expected participant conflict for user-1, got %v", participantWarning.ParticipantID)
	}
	if participantWarning.ScheduleID != "schedule-existing" {
		t.Fatalf("expected conflict with schedule-existing, got %s", participantWarning.ScheduleID)
	}

	roomWarning, ok := warningTypes["room"]
	if !ok {
		t.Fatalf("expected room conflict warning, got %v", warnings)
	}
	if roomWarning.RoomID == nil || *roomWarning.RoomID != roomID {
		t.Fatalf("expected room conflict for room-1, got %v", roomWarning.RoomID)
	}

	if repo.created.ID == "" {
		t.Fatalf("expected schedule to be persisted despite warnings")
	}

	if created.ID == "" {
		t.Fatalf("expected created schedule to include identifier")
	}
}

func TestScheduleService_CreateSchedule_PersistsWhenConflictListMissing(t *testing.T) {
	repo := &scheduleRepoStub{listErr: ErrNotFound}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() string { return "schedule-new" }, func() time.Time { return mustJST(t, 9) })

	created, warnings, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1"},
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Planning meeting",
			Start:          mustJST(t, 10),
			End:            mustJST(t, 11),
			ParticipantIDs: []string{"user-1"},
		},
	})

	if err != nil {
		t.Fatalf("expected success when conflict list missing, got %v", err)
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings when repository reports no existing schedules, got %v", warnings)
	}

	if repo.created.ID == "" {
		t.Fatalf("expected schedule to be persisted when detector list is missing")
	}

	if created.ID == "" {
		t.Fatalf("expected created schedule to include identifier")
	}
}

func TestScheduleService_UpdateSchedule_ValidatesCreatorImmutability(t *testing.T) {
	repo := &scheduleRepoStub{schedule: Schedule{
		ID:             "schedule-1",
		CreatorID:      "user-1",
		Title:          "Design sync",
		Start:          mustJST(t, 9),
		End:            mustJST(t, 10),
		CreatedAt:      mustJST(t, 8),
		UpdatedAt:      mustJST(t, 8),
		ParticipantIDs: []string{"user-1"},
	}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
		Principal:  Principal{UserID: "user-1"},
		ScheduleID: "schedule-1",
		Input: ScheduleInput{
			CreatorID:      "user-2",
			Title:          "Updated",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			ParticipantIDs: []string{"user-1"},
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["creator_id"]; !ok {
		t.Fatalf("expected creator_id validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_UpdateSchedule_BlocksUnauthorizedUsers(t *testing.T) {
	repo := &scheduleRepoStub{schedule: Schedule{
		ID:             "schedule-1",
		CreatorID:      "user-1",
		Title:          "Design sync",
		Start:          mustJST(t, 9),
		End:            mustJST(t, 10),
		ParticipantIDs: []string{"user-1"},
	}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
		Principal:  Principal{UserID: "user-2"},
		ScheduleID: "schedule-1",
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Design sync",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			ParticipantIDs: []string{"user-1"},
		},
	})

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestScheduleService_UpdateSchedule_ValidatesParticipantsExist(t *testing.T) {
	existing := Schedule{
		ID:             "schedule-1",
		CreatorID:      "user-1",
		Title:          "Design sync",
		Start:          mustJST(t, 9),
		End:            mustJST(t, 10),
		ParticipantIDs: []string{"user-1"},
	}
	repo := &scheduleRepoStub{schedule: existing}
	users := &userDirectoryStub{missing: []string{"user-2"}}
	svc := NewScheduleService(repo, users, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
		Principal:  Principal{UserID: "user-1", IsAdmin: true},
		ScheduleID: "schedule-1",
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Design sync",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			ParticipantIDs: []string{"user-1", "user-2"},
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["participants"]; !ok {
		t.Fatalf("expected participants validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_UpdateSchedule_ValidatesRoomExistence(t *testing.T) {
	existing := Schedule{
		ID:             "schedule-1",
		CreatorID:      "user-1",
		Title:          "Design sync",
		Start:          mustJST(t, 9),
		End:            mustJST(t, 10),
		ParticipantIDs: []string{"user-1"},
	}
	repo := &scheduleRepoStub{schedule: existing}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: false}, nil, nil, func() time.Time { return mustJST(t, 9) })

	roomID := "room-1"
	_, _, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
		Principal:  Principal{UserID: "user-1", IsAdmin: true},
		ScheduleID: "schedule-1",
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Design sync",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			RoomID:         &roomID,
			ParticipantIDs: []string{"user-1"},
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["room_id"]; !ok {
		t.Fatalf("expected room_id validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_UpdateSchedule_AllowsAdministratorOverride(t *testing.T) {
	repo := &scheduleRepoStub{schedule: Schedule{
		ID:             "schedule-1",
		CreatorID:      "user-1",
		Title:          "Design sync",
		Start:          mustJST(t, 9),
		End:            mustJST(t, 10),
		ParticipantIDs: []string{"user-1"},
	}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
		Principal:  Principal{UserID: "admin", IsAdmin: true},
		ScheduleID: "schedule-1",
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Updated",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 11),
			ParticipantIDs: []string{"user-1"},
		},
	})

	if err != nil {
		t.Fatalf("expected update success for admin override, got %v", err)
	}

	if repo.updated.Title != "Updated" {
		t.Fatalf("expected updated title, got %s", repo.updated.Title)
	}
}

func TestScheduleService_UpdateSchedule_ValidatesTemporalBounds(t *testing.T) {
	repo := &scheduleRepoStub{schedule: Schedule{
		ID:             "schedule-1",
		CreatorID:      "user-1",
		Title:          "Design sync",
		Start:          mustJST(t, 9),
		End:            mustJST(t, 10),
		ParticipantIDs: []string{"user-1"},
	}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
		Principal:  Principal{UserID: "user-1"},
		ScheduleID: "schedule-1",
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Design sync",
			Start:          mustJST(t, 11),
			End:            mustJST(t, 10),
			ParticipantIDs: []string{"user-1"},
		},
	})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	if _, ok := vErr.FieldErrors["time"]; !ok {
		t.Fatalf("expected time validation error, got %v", vErr.FieldErrors)
	}
}

func TestScheduleService_DeleteSchedule_BlocksUnauthorizedUsers(t *testing.T) {
	repo := &scheduleRepoStub{schedule: Schedule{ID: "schedule-1", CreatorID: "user-1"}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

	err := svc.DeleteSchedule(context.Background(), Principal{UserID: "user-2"}, "schedule-1")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestScheduleService_DeleteSchedule_AllowsAdministratorOverride(t *testing.T) {
	repo := &scheduleRepoStub{schedule: Schedule{ID: "schedule-1", CreatorID: "user-1"}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

	if err := svc.DeleteSchedule(context.Background(), Principal{UserID: "admin", IsAdmin: true}, "schedule-1"); err != nil {
		t.Fatalf("expected admin delete to succeed, got %v", err)
	}
}

func TestScheduleService_UpdateSchedule_ReturnsNotFoundWhenMissing(t *testing.T) {
	repo := &scheduleRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 9) })

	_, _, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
		Principal:  Principal{UserID: "user-1"},
		ScheduleID: "missing", // repository will surface ErrNotFound for unknown ID
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Design sync",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			ParticipantIDs: []string{"user-1"},
		},
	})

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound when schedule is missing, got %v", err)
	}
}

func TestScheduleService_DeleteSchedule_ReturnsNotFoundWhenMissing(t *testing.T) {
	repo := &scheduleRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

	err := svc.DeleteSchedule(context.Background(), Principal{UserID: "user-1"}, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound when schedule is missing, got %v", err)
	}
}

func TestScheduleService_ListSchedules_FilteringAndOrdering(t *testing.T) {
	t.Run("defaults to returning only the principal's schedules when no participant filter provided", func(t *testing.T) {
		repo := &scheduleRepoStub{
			list: []Schedule{{
				ID:             "schedule-1",
				CreatorID:      "user-1",
				Title:          "Design sync",
				Start:          mustJST(t, 9),
				End:            mustJST(t, 10),
				ParticipantIDs: []string{"user-1"},
			}},
		}

		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

		schedules, warnings, err := svc.ListSchedules(context.Background(), ListSchedulesParams{
			Principal: Principal{UserID: "user-1"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(repo.listFilter.ParticipantIDs) != 1 || repo.listFilter.ParticipantIDs[0] != "user-1" {
			t.Fatalf("expected filter to include only principal, got %v", repo.listFilter.ParticipantIDs)
		}

		if len(schedules) != 1 || schedules[0].ID != "schedule-1" {
			t.Fatalf("expected schedule list to be returned unchanged, got %v", schedules)
		}

		if len(warnings) != 0 {
			t.Fatalf("expected no warnings for non-conflicting schedules, got %v", warnings)
		}
	})

	t.Run("allows explicit participant filter to surface colleague schedules without leaking others", func(t *testing.T) {
		repo := &scheduleRepoStub{list: []Schedule{}}
		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

		_, _, err := svc.ListSchedules(context.Background(), ListSchedulesParams{
			Principal:      Principal{UserID: "user-1"},
			ParticipantIDs: []string{"user-3", "user-2", "user-3"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expected := []string{"user-1", "user-2", "user-3"}
		if diff := compareStringSlices(repo.listFilter.ParticipantIDs, expected); diff != "" {
			t.Fatalf("participant filter mismatch: %s", diff)
		}
	})

	t.Run("returns only principal schedules by default even when repository stores colleagues", func(t *testing.T) {
		repo := &filteringScheduleRepo{
			schedules: []Schedule{
				{
					ID:             "schedule-principal",
					CreatorID:      "user-1",
					Title:          "Principal meeting",
					Start:          mustJST(t, 9),
					End:            mustJST(t, 10),
					ParticipantIDs: []string{"user-1"},
				},
				{
					ID:             "schedule-colleague",
					CreatorID:      "user-2",
					Title:          "Colleague meeting",
					Start:          mustJST(t, 11),
					End:            mustJST(t, 12),
					ParticipantIDs: []string{"user-2"},
				},
			},
		}
		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

		schedules, _, err := svc.ListSchedules(context.Background(), ListSchedulesParams{
			Principal: Principal{UserID: "user-1"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(schedules) != 1 || schedules[0].ID != "schedule-principal" {
			t.Fatalf("expected only principal schedule, got %#v", schedules)
		}
	})

	t.Run("respects explicit participant selections while keeping the principal's schedules", func(t *testing.T) {
		repo := &filteringScheduleRepo{
			schedules: []Schedule{
				{
					ID:             "schedule-principal",
					CreatorID:      "user-1",
					Title:          "Principal meeting",
					Start:          mustJST(t, 13),
					End:            mustJST(t, 14),
					ParticipantIDs: []string{"user-1"},
				},
				{
					ID:             "schedule-colleague",
					CreatorID:      "user-2",
					Title:          "Colleague meeting",
					Start:          mustJST(t, 9),
					End:            mustJST(t, 10),
					ParticipantIDs: []string{"user-2"},
				},
				{
					ID:             "schedule-unrelated",
					CreatorID:      "user-3",
					Title:          "Unrelated",
					Start:          mustJST(t, 8),
					End:            mustJST(t, 9),
					ParticipantIDs: []string{"user-3"},
				},
			},
		}
		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

		schedules, _, err := svc.ListSchedules(context.Background(), ListSchedulesParams{
			Principal:      Principal{UserID: "user-1"},
			ParticipantIDs: []string{"user-2"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		ids := scheduleIDs(schedules)
		expected := []string{"schedule-colleague", "schedule-principal"}
		if diff := compareStringSlices(ids, expected); diff != "" {
			t.Fatalf("expected schedules %v, got %v (%s)", expected, ids, diff)
		}
	})

	t.Run("orders schedules chronologically and deterministically", func(t *testing.T) {
		repo := &scheduleRepoStub{
			list: []Schedule{
				{
					ID:             "schedule-b",
					CreatorID:      "user-1",
					Title:          "Later",
					Start:          mustJST(t, 11),
					End:            mustJST(t, 12),
					ParticipantIDs: []string{"user-1"},
				},
				{
					ID:             "schedule-a",
					CreatorID:      "user-1",
					Title:          "Same time",
					Start:          mustJST(t, 9),
					End:            mustJST(t, 10),
					ParticipantIDs: []string{"user-1"},
				},
				{
					ID:             "schedule-c",
					CreatorID:      "user-1",
					Title:          "Same time",
					Start:          mustJST(t, 9),
					End:            mustJST(t, 10),
					ParticipantIDs: []string{"user-1"},
				},
			},
		}

		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

		schedules, _, err := svc.ListSchedules(context.Background(), ListSchedulesParams{
			Principal: Principal{UserID: "user-1"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		ids := scheduleIDs(schedules)
		expected := []string{"schedule-a", "schedule-c", "schedule-b"}
		if diff := compareStringSlices(ids, expected); diff != "" {
			t.Fatalf("expected sorted schedules %v, got %v (%s)", expected, ids, diff)
		}
	})

	t.Run("expands recurrence hooks for requested period windows", func(t *testing.T) {
		repo := &scheduleRepoStub{list: []Schedule{}}
		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

		_, _, err := svc.ListSchedules(context.Background(), ListSchedulesParams{
			Principal: Principal{UserID: "user-1"},
		})
		if err != nil {
			t.Fatalf("expected no error when listing empty schedules, got %v", err)
		}
	})

	t.Run("propagates detector warnings alongside successful results", func(t *testing.T) {
		roomID := "room-42"
		repo := &scheduleRepoStub{
			list: []Schedule{
				{
					ID:             "schedule-1",
					CreatorID:      "user-1",
					Title:          "Design sync",
					Start:          mustJST(t, 9),
					End:            mustJST(t, 10),
					ParticipantIDs: []string{"user-1"},
					RoomID:         &roomID,
				},
				{
					ID:             "schedule-2",
					CreatorID:      "user-2",
					Title:          "Team sync",
					Start:          mustJST(t, 9),
					End:            mustJST(t, 10),
					ParticipantIDs: []string{"user-1"},
					RoomID:         &roomID,
				},
			},
		}

		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

		_, warnings, err := svc.ListSchedules(context.Background(), ListSchedulesParams{
			Principal: Principal{UserID: "user-1"},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(warnings) == 0 {
			t.Fatalf("expected warnings for conflicting schedules")
		}

		participantWarning := false
		roomWarning := false
		for _, warning := range warnings {
			switch warning.Type {
			case string(scheduler.ConflictTypeParticipant):
				if warning.ScheduleID == "schedule-2" && warning.ParticipantID == "user-1" {
					participantWarning = true
				}
			case string(scheduler.ConflictTypeRoom):
				if warning.ScheduleID == "schedule-2" && warning.RoomID != nil && *warning.RoomID == roomID {
					roomWarning = true
				}
			}
		}

		if !participantWarning {
			t.Fatalf("expected participant warning referencing schedule-2, got %v", warnings)
		}
		if !roomWarning {
			t.Fatalf("expected room warning referencing schedule-2, got %v", warnings)
		}
	})
}

func TestScheduleService_CreateSchedule_SavesRecurrence(t *testing.T) {
	t.Parallel()
	repo := &scheduleRepoStub{}
	recurrences := &recurrenceRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, recurrences, func() string { return "schedule-1" }, func() time.Time { return mustJST(t, 9) })

	recurrenceInput := &RecurrenceInput{
		Frequency: "weekly",
		Weekdays:  []string{"Monday", "Wednesday"},
	}

	_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
		Principal: Principal{UserID: "user-1"},
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Weekly Sync",
			Start:          mustJST(t, 10),
			End:            mustJST(t, 11),
			ParticipantIDs: []string{"user-1"},
			Recurrence:     recurrenceInput,
		},
	})

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if recurrences.savedScheduleID != "schedule-1" {
		t.Errorf("expected SaveRecurrence to be called with schedule ID 'schedule-1', got '%s'", recurrences.savedScheduleID)
	}
	if recurrences.savedRecurrence == nil {
		t.Fatal("expected SaveRecurrence to be called, but it was not")
	}
	if recurrences.savedRecurrence.Frequency != "weekly" {
		t.Errorf("expected saved recurrence frequency to be 'weekly', got '%s'", recurrences.savedRecurrence.Frequency)
	}
	if !recurrences.savedStart.Equal(mustJST(t, 10)) {
		t.Errorf("expected start time to be saved with recurrence")
	}
}

func TestScheduleService_UpdateSchedule_SavesRecurrence(t *testing.T) {
	t.Parallel()
	repo := &scheduleRepoStub{
		schedule: Schedule{
			ID:             "schedule-1",
			CreatorID:      "user-1",
			Start:          mustJST(t, 10),
			ParticipantIDs: []string{"user-1"},
		},
	}
	recurrences := &recurrenceRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, recurrences, nil, func() time.Time { return mustJST(t, 9) })

	recurrenceInput := &RecurrenceInput{
		Frequency: "weekly",
		Weekdays:  []string{"Friday"},
	}

	_, _, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
		Principal:  Principal{UserID: "user-1"},
		ScheduleID: "schedule-1",
		Input: ScheduleInput{
			Title:          "Updated Weekly Sync",
			Start:          mustJST(t, 10),
			End:            mustJST(t, 11),
			ParticipantIDs: []string{"user-1"},
			Recurrence:     recurrenceInput,
		},
	})

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if recurrences.savedScheduleID != "schedule-1" {
		t.Errorf("expected SaveRecurrence to be called with schedule ID 'schedule-1', got '%s'", recurrences.savedScheduleID)
	}
	if recurrences.savedRecurrence == nil {
		t.Fatal("expected SaveRecurrence to be called, but it was not")
	}
	if recurrences.savedRecurrence.Frequency != "weekly" {
		t.Errorf("expected saved recurrence frequency to be 'weekly', got '%s'", recurrences.savedRecurrence.Frequency)
	}
	if !recurrences.savedStart.Equal(mustJST(t, 10)) {
		t.Errorf("expected start time to be saved with recurrence")
	}
}

func TestScheduleService_ListSchedules_PeriodFilters(t *testing.T) {
	t.Run("maps day/week/month presets into StartsAfter/EndsBefore filters", func(t *testing.T) {
		repo := &scheduleRepoStub{}
		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

		reference := time.Date(2024, 4, 3, 15, 30, 0, 0, time.FixedZone("JST", 9*60*60))

		_, _, err := svc.ListSchedules(context.Background(), ListSchedulesParams{
			Principal:       Principal{UserID: "user-1"},
			Period:          ListPeriodWeek,
			PeriodReference: reference,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if repo.listFilter.StartsAfter == nil || repo.listFilter.EndsBefore == nil {
			t.Fatalf("expected filter bounds to be populated, got %+v", repo.listFilter)
		}

		start := repo.listFilter.StartsAfter.In(time.FixedZone("JST", 9*60*60))
		end := repo.listFilter.EndsBefore.In(time.FixedZone("JST", 9*60*60))

		if start.Weekday() != time.Monday || start.Hour() != 0 || start.Minute() != 0 {
			t.Fatalf("expected week start at Monday 00:00 JST, got %v", start)
		}

		if end.Sub(start) != 7*24*time.Hour {
			t.Fatalf("expected week range of 7 days, got %v", end.Sub(start))
		}
	})

	t.Run("clips recurrence expansion to requested window", func(t *testing.T) {
		t.Skip("TODO: verify recurring occurrences outside the window are excluded")
	})
}

func TestScheduleService_CreateSchedule_EnforcesJapanStandardTime(t *testing.T) {
	t.Run("rejects start times outside Asia/Tokyo", func(t *testing.T) {
		svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

		_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
			Principal: Principal{UserID: "user-1"},
			Input: ScheduleInput{
				CreatorID:      "user-1",
				Title:          "Design sync",
				Start:          time.Date(2024, 3, 14, 9, 0, 0, 0, time.UTC),
				End:            mustJST(t, 10),
				ParticipantIDs: []string{"user-1"},
			},
		})

		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected validation error, got %v", err)
		}

		if _, ok := vErr.FieldErrors["start"]; !ok {
			t.Fatalf("expected start timezone validation error, got %v", vErr.FieldErrors)
		}
	})

	t.Run("rejects end times outside Asia/Tokyo", func(t *testing.T) {
		svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, nil)

		_, _, err := svc.CreateSchedule(context.Background(), CreateScheduleParams{
			Principal: Principal{UserID: "user-1"},
			Input: ScheduleInput{
				CreatorID:      "user-1",
				Title:          "Design sync",
				Start:          mustJST(t, 9),
				End:            time.Date(2024, 3, 14, 10, 0, 0, 0, time.UTC),
				ParticipantIDs: []string{"user-1"},
			},
		})

		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected validation error, got %v", err)
		}

		if _, ok := vErr.FieldErrors["end"]; !ok {
			t.Fatalf("expected end timezone validation error, got %v", vErr.FieldErrors)
		}
	})
}

func TestScheduleService_UpdateSchedule_CleansUpRecurrences(t *testing.T) {
	t.Run("removes obsolete recurrence rules when participants change", func(t *testing.T) {
		repo := &scheduleRepoStub{
			schedule: Schedule{
				ID:             "schedule-1",
				CreatorID:      "user-1",
				Title:          "Design sync",
				Start:          mustJST(t, 9),
				End:            mustJST(t, 10),
				ParticipantIDs: []string{"user-1"},
			},
		}
		recurrences := &recurrenceRepoStub{}

		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, recurrences, nil, func() time.Time { return mustJST(t, 8) })

		updated, warnings, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
			Principal:  Principal{UserID: "user-1"},
			ScheduleID: "schedule-1",
			Input: ScheduleInput{
				Title:          "Design sync",
				Start:          mustJST(t, 9),
				End:            mustJST(t, 10),
				ParticipantIDs: []string{"user-1", "user-2"},
			},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(warnings) != 0 {
			t.Fatalf("expected no warnings, got %v", warnings)
		}

		if diff := compareStringSlices(updated.ParticipantIDs, []string{"user-1", "user-2"}); diff != "" {
			t.Fatalf("unexpected participants: %s", diff)
		}

		if len(recurrences.deletedIDs) != 1 || recurrences.deletedIDs[0] != "schedule-1" {
			t.Fatalf("expected recurrence cleanup for schedule-1, got %#v", recurrences.deletedIDs)
		}
	})

	t.Run("maintains recurrence integrity when dates shift", func(t *testing.T) {
		repo := &scheduleRepoStub{
			schedule: Schedule{
				ID:             "schedule-2",
				CreatorID:      "user-1",
				Title:          "Weekly sync",
				Start:          mustJST(t, 9),
				End:            mustJST(t, 10),
				ParticipantIDs: []string{"user-1", "user-2"},
			},
		}
		recurrences := &recurrenceRepoStub{}

		newStart := mustJST(t, 11)
		newEnd := mustJST(t, 12)

		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, recurrences, nil, func() time.Time { return mustJST(t, 8) })

		updated, _, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
			Principal:  Principal{UserID: "user-1"},
			ScheduleID: "schedule-2",
			Input: ScheduleInput{
				Title:          "Weekly sync",
				Start:          newStart,
				End:            newEnd,
				ParticipantIDs: []string{"user-1", "user-2"},
			},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !repo.updated.Start.Equal(newStart) || !repo.updated.End.Equal(newEnd) {
			t.Fatalf("expected updated times to persist, got start %v end %v", repo.updated.Start, repo.updated.End)
		}

		if !updated.Start.Equal(newStart) || !updated.End.Equal(newEnd) {
			t.Fatalf("expected returned schedule to reflect new bounds, got start %v end %v", updated.Start, updated.End)
		}

		if len(recurrences.deletedIDs) != 1 || recurrences.deletedIDs[0] != "schedule-2" {
			t.Fatalf("expected recurrence cleanup for schedule-2, got %#v", recurrences.deletedIDs)
		}
	})
}

func TestScheduleService_DeleteSchedule_CleansUpRecurrences(t *testing.T) {
	t.Run("removes recurrence definitions alongside the schedule", func(t *testing.T) {
		repo := &scheduleRepoStub{
			schedule: Schedule{
				ID:        "schedule-3",
				CreatorID: "user-1",
			},
		}
		recurrences := &recurrenceRepoStub{}

		svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, recurrences, nil, nil)

		if err := svc.DeleteSchedule(context.Background(), Principal{UserID: "user-1"}, "schedule-3"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(recurrences.deletedIDs) != 1 || recurrences.deletedIDs[0] != "schedule-3" {
			t.Fatalf("expected recurrence cleanup for schedule-3, got %#v", recurrences.deletedIDs)
		}
	})
}

func TestScheduleService_UpdateSchedule_PersistsDespiteWarnings(t *testing.T) {
	roomID := "room-2"
	repo := &scheduleRepoStub{
		schedule: Schedule{
			ID:             "schedule-1",
			CreatorID:      "user-1",
			Title:          "Design sync",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			ParticipantIDs: []string{"user-1"},
		},
		list: []Schedule{
			{
				ID:             "schedule-1",
				CreatorID:      "user-1",
				Title:          "Design sync",
				Start:          mustJST(t, 9),
				End:            mustJST(t, 10),
				ParticipantIDs: []string{"user-1"},
			},
			{
				ID:             "schedule-2",
				CreatorID:      "user-2",
				Title:          "Team sync",
				Start:          mustJST(t, 9),
				End:            mustJST(t, 10),
				ParticipantIDs: []string{"user-1"},
				RoomID:         &roomID,
			},
		},
	}

	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil, func() time.Time { return mustJST(t, 8) })

	updated, warnings, err := svc.UpdateSchedule(context.Background(), UpdateScheduleParams{
		Principal:  Principal{UserID: "user-1"},
		ScheduleID: "schedule-1",
		Input: ScheduleInput{
			CreatorID:      "user-1",
			Title:          "Updated title",
			Start:          mustJST(t, 9),
			End:            mustJST(t, 10),
			RoomID:         &roomID,
			ParticipantIDs: []string{"user-1"},
		},
	})

	if err != nil {
		t.Fatalf("expected update to succeed with warnings, got %v", err)
	}

	if len(warnings) == 0 {
		t.Fatalf("expected warnings to be returned, got none")
	}

	foundParticipantWarning := false
	for _, warning := range warnings {
		if warning.Type == "participant" && warning.ScheduleID == "schedule-2" {
			foundParticipantWarning = true
			break
		}
	}

	if !foundParticipantWarning {
		t.Fatalf("expected participant warning referencing schedule-2, got %v", warnings)
	}

	for _, warning := range warnings {
		if warning.ScheduleID == "schedule-1" {
			t.Fatalf("expected detector to ignore the schedule being updated, got warning %v", warning)
		}
	}

	if repo.updated.Title != "Updated title" {
		t.Fatalf("expected repository to receive updated schedule, got %s", repo.updated.Title)
	}

	if updated.Title != "Updated title" {
		t.Fatalf("expected updated schedule to be returned, got %s", updated.Title)
	}
}
