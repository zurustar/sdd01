package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

type scheduleRepoStub struct {
	schedule  Schedule
	created   Schedule
	updated   Schedule
	err       error
	deleteErr error
	list      []Schedule
	listErr   error
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

func (s *scheduleRepoStub) ListSchedules(ctx context.Context) ([]Schedule, error) {
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

func mustJST(t *testing.T, hour int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("failed to load JST location: %v", err)
	}
	return time.Date(2024, 3, 14, hour, 0, 0, 0, loc)
}

func TestScheduleService_CreateSchedule_ValidatesTemporalBounds(t *testing.T) {
	t.Parallel()

	repo := &scheduleRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, func() string { return "schedule-1" }, func() time.Time { return mustJST(t, 9) })

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

func TestScheduleService_CreateSchedule_ValidatesRequiredFields(t *testing.T) {
	t.Parallel()

	svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

	svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

	svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil)

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
	t.Parallel()

	repo := &scheduleRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, func() string { return "schedule-1" }, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

	svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{missing: []string{"user-2"}}, &roomCatalogStub{exists: true}, nil, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

	roomID := "room-1"
	svc := NewScheduleService(&scheduleRepoStub{}, &userDirectoryStub{}, &roomCatalogStub{exists: false}, nil, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

	roomID := "room-1"
	repo := &scheduleRepoStub{schedule: Schedule{ID: "schedule-1", CreatorID: "user-1"}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, func() string { return "schedule-1" }, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

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

	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, func() string { return "schedule-new" }, func() time.Time { return mustJST(t, 8) })

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
	t.Parallel()

	repo := &scheduleRepoStub{listErr: ErrNotFound}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, func() string { return "schedule-new" }, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

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
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

	repo := &scheduleRepoStub{schedule: Schedule{
		ID:             "schedule-1",
		CreatorID:      "user-1",
		Title:          "Design sync",
		Start:          mustJST(t, 9),
		End:            mustJST(t, 10),
		ParticipantIDs: []string{"user-1"},
	}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() time.Time { return mustJST(t, 9) })

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

func TestScheduleService_UpdateSchedule_AllowsAdministratorOverride(t *testing.T) {
	t.Parallel()

	repo := &scheduleRepoStub{schedule: Schedule{
		ID:             "schedule-1",
		CreatorID:      "user-1",
		Title:          "Design sync",
		Start:          mustJST(t, 9),
		End:            mustJST(t, 10),
		ParticipantIDs: []string{"user-1"},
	}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

	repo := &scheduleRepoStub{schedule: Schedule{
		ID:             "schedule-1",
		CreatorID:      "user-1",
		Title:          "Design sync",
		Start:          mustJST(t, 9),
		End:            mustJST(t, 10),
		ParticipantIDs: []string{"user-1"},
	}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

	repo := &scheduleRepoStub{schedule: Schedule{ID: "schedule-1", CreatorID: "user-1"}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil)

	err := svc.DeleteSchedule(context.Background(), Principal{UserID: "user-2"}, "schedule-1")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestScheduleService_DeleteSchedule_AllowsAdministratorOverride(t *testing.T) {
	t.Parallel()

	repo := &scheduleRepoStub{schedule: Schedule{ID: "schedule-1", CreatorID: "user-1"}}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil)

	if err := svc.DeleteSchedule(context.Background(), Principal{UserID: "admin", IsAdmin: true}, "schedule-1"); err != nil {
		t.Fatalf("expected admin delete to succeed, got %v", err)
	}
}

func TestScheduleService_UpdateSchedule_ReturnsNotFoundWhenMissing(t *testing.T) {
	t.Parallel()

	repo := &scheduleRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() time.Time { return mustJST(t, 9) })

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
	t.Parallel()

	repo := &scheduleRepoStub{}
	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, nil)

	err := svc.DeleteSchedule(context.Background(), Principal{UserID: "user-1"}, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound when schedule is missing, got %v", err)
	}
}

func TestScheduleService_ListSchedules_FilteringAndOrdering(t *testing.T) {
	t.Parallel()

	t.Run("defaults to returning only the principal's schedules when no participant filter provided", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListSchedules restricts results to the authenticated principal when filter is empty")
	})

	t.Run("allows explicit participant filter to surface colleague schedules without leaking others", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert ListSchedules returns only schedules for requested participant IDs")
	})

	t.Run("orders schedules chronologically and deterministically", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: verify ListSchedules sorts by start time then stable identifier")
	})

	t.Run("expands recurrence hooks for requested period windows", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListSchedules leverages recurrence expansion for requested ranges")
	})

	t.Run("propagates detector warnings alongside successful results", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert conflict warnings are returned even when schedules persist")
	})
}

func TestScheduleService_ListSchedules_PeriodFilters(t *testing.T) {
	t.Parallel()

	t.Run("maps day/week/month presets into StartsAfter/EndsBefore filters", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure predefined period options translate to accurate interval filters")
	})

	t.Run("clips recurrence expansion to requested window", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: verify recurring occurrences outside the window are excluded")
	})
}

func TestScheduleService_CreateSchedule_EnforcesJapanStandardTime(t *testing.T) {
	t.Parallel()

	t.Run("rejects start times outside Asia/Tokyo", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure CreateSchedule validates times are provided in JST")
	})

	t.Run("rejects end times outside Asia/Tokyo", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure CreateSchedule enforces JST end times")
	})
}

func TestScheduleService_UpdateSchedule_CleansUpRecurrences(t *testing.T) {
	t.Parallel()

	t.Run("removes obsolete recurrence rules when participants change", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure UpdateSchedule triggers recurrence cleanup when cadence shifts")
	})

	t.Run("maintains recurrence integrity when dates shift", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert UpdateSchedule recalculates recurrence window bounds")
	})
}

func TestScheduleService_DeleteSchedule_CleansUpRecurrences(t *testing.T) {
	t.Parallel()

	t.Run("removes recurrence definitions alongside the schedule", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure DeleteSchedule cascades recurrence removal")
	})
}

func TestScheduleService_UpdateSchedule_PersistsDespiteWarnings(t *testing.T) {
	t.Parallel()

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

	svc := NewScheduleService(repo, &userDirectoryStub{}, &roomCatalogStub{exists: true}, nil, func() time.Time { return mustJST(t, 8) })

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
