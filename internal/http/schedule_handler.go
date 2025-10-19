package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
)

type scheduleService interface {
	CreateSchedule(ctx context.Context, params application.CreateScheduleParams) (application.Schedule, []application.ConflictWarning, error)
	UpdateSchedule(ctx context.Context, params application.UpdateScheduleParams) (application.Schedule, []application.ConflictWarning, error)
	DeleteSchedule(ctx context.Context, principal application.Principal, scheduleID string) error
	ListSchedules(ctx context.Context, params application.ListSchedulesParams) ([]application.Schedule, []application.ConflictWarning, error)
}

type ScheduleHandler struct {
	service   scheduleService
	responder responder
}

func NewScheduleHandler(service scheduleService, logger *slog.Logger) *ScheduleHandler {
	return &ScheduleHandler{service: service, responder: newResponder(logger)}
}

func (h *ScheduleHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var req scheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errBadRequestBody)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())

	schedule, warnings, err := h.service.CreateSchedule(r.Context(), application.CreateScheduleParams{
		Principal: principal,
		Input:     req.toInput(),
	})
	if err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	h.renderSchedule(r.Context(), w, schedule, warnings, http.StatusCreated)
}

func (h *ScheduleHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	scheduleID, ok := ScheduleIDFromContext(r.Context())
	if !ok || strings.TrimSpace(scheduleID) == "" {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errInvalidScheduleID)
		return
	}

	var req scheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errBadRequestBody)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())

	schedule, warnings, err := h.service.UpdateSchedule(r.Context(), application.UpdateScheduleParams{
		Principal:  principal,
		ScheduleID: scheduleID,
		Input:      req.toInput(),
	})
	if err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	h.renderSchedule(r.Context(), w, schedule, warnings, http.StatusOK)
}

func (h *ScheduleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	scheduleID, ok := ScheduleIDFromContext(r.Context())
	if !ok || strings.TrimSpace(scheduleID) == "" {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errInvalidScheduleID)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())
	if err := h.service.DeleteSchedule(r.Context(), principal, scheduleID); err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	h.responder.writeJSON(r.Context(), w, http.StatusNoContent, nil)
}

func (h *ScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())
	params := buildListParams(r.URL.Query(), principal)

	schedules, warnings, err := h.service.ListSchedules(r.Context(), params)
	if err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	response := listSchedulesResponse{
		Schedules: toScheduleDTOs(schedules),
		Warnings:  toWarningDTOs(warnings),
	}

	h.responder.writeJSON(r.Context(), w, http.StatusOK, response)
}

func (h *ScheduleHandler) renderSchedule(ctx context.Context, w http.ResponseWriter, schedule application.Schedule, warnings []application.ConflictWarning, status int) {
	payload := scheduleResponse{
		Schedule: toScheduleDTO(schedule),
		Warnings: toWarningDTOs(warnings),
	}
	h.responder.writeJSON(ctx, w, status, payload)
}

type scheduleRequest struct {
	CreatorID        string   `json:"creator_id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Start            string   `json:"start"`
	End              string   `json:"end"`
	RoomID           *string  `json:"room_id"`
	WebConferenceURL string   `json:"web_conference_url"`
	ParticipantIDs   []string `json:"participant_ids"`
}

func (r scheduleRequest) toInput() application.ScheduleInput {
	return application.ScheduleInput{
		CreatorID:        strings.TrimSpace(r.CreatorID),
		Title:            strings.TrimSpace(r.Title),
		Description:      r.Description,
		Start:            parseTime(r.Start),
		End:              parseTime(r.End),
		RoomID:           r.RoomID,
		WebConferenceURL: strings.TrimSpace(r.WebConferenceURL),
		ParticipantIDs:   append([]string(nil), r.ParticipantIDs...),
	}
}

func parseTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts
	}
	return time.Time{}
}

type scheduleResponse struct {
	Schedule scheduleDTO          `json:"schedule"`
	Warnings []conflictWarningDTO `json:"warnings,omitempty"`
}

type listSchedulesResponse struct {
	Schedules []scheduleDTO        `json:"schedules"`
	Warnings  []conflictWarningDTO `json:"warnings,omitempty"`
}

type scheduleDTO struct {
	ID               string          `json:"id"`
	CreatorID        string          `json:"creator_id"`
	Title            string          `json:"title"`
	Description      string          `json:"description"`
	Start            string          `json:"start"`
	End              string          `json:"end"`
	RoomID           *string         `json:"room_id,omitempty"`
	WebConferenceURL string          `json:"web_conference_url,omitempty"`
	ParticipantIDs   []string        `json:"participant_ids"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
	Occurrences      []occurrenceDTO `json:"occurrences,omitempty"`
}

func toScheduleDTO(schedule application.Schedule) scheduleDTO {
	return scheduleDTO{
		ID:               schedule.ID,
		CreatorID:        schedule.CreatorID,
		Title:            schedule.Title,
		Description:      schedule.Description,
		Start:            schedule.Start.UTC().Format(time.RFC3339Nano),
		End:              schedule.End.UTC().Format(time.RFC3339Nano),
		RoomID:           schedule.RoomID,
		WebConferenceURL: schedule.WebConferenceURL,
		ParticipantIDs:   append([]string(nil), schedule.ParticipantIDs...),
		CreatedAt:        schedule.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:        schedule.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Occurrences:      toOccurrenceDTOs(schedule.Occurrences),
	}
}

func toScheduleDTOs(schedules []application.Schedule) []scheduleDTO {
	if len(schedules) == 0 {
		return nil
	}
	out := make([]scheduleDTO, 0, len(schedules))
	for _, schedule := range schedules {
		out = append(out, toScheduleDTO(schedule))
	}
	return out
}

type conflictWarningDTO struct {
	ScheduleID    string  `json:"schedule_id"`
	Type          string  `json:"type"`
	ParticipantID string  `json:"participant_id,omitempty"`
	RoomID        *string `json:"room_id,omitempty"`
}

type occurrenceDTO struct {
	ScheduleID string `json:"schedule_id"`
	RuleID     string `json:"rule_id,omitempty"`
	Start      string `json:"start"`
	End        string `json:"end"`
}

func toOccurrenceDTOs(occurrences []application.ScheduleOccurrence) []occurrenceDTO {
	if len(occurrences) == 0 {
		return nil
	}

	out := make([]occurrenceDTO, 0, len(occurrences))
	for _, occurrence := range occurrences {
		dto := occurrenceDTO{
			ScheduleID: occurrence.ScheduleID,
			RuleID:     occurrence.RuleID,
			Start:      occurrence.Start.UTC().Format(time.RFC3339Nano),
			End:        occurrence.End.UTC().Format(time.RFC3339Nano),
		}
		out = append(out, dto)
	}
	return out
}

func toWarningDTOs(warnings []application.ConflictWarning) []conflictWarningDTO {
	if len(warnings) == 0 {
		return nil
	}

	out := make([]conflictWarningDTO, 0, len(warnings))
	for _, warning := range warnings {
		dto := conflictWarningDTO{
			ScheduleID:    warning.ScheduleID,
			Type:          warning.Type,
			ParticipantID: warning.ParticipantID,
			RoomID:        warning.RoomID,
		}
		out = append(out, dto)
	}
	return out
}

func buildListParams(values url.Values, principal application.Principal) application.ListSchedulesParams {
	params := application.ListSchedulesParams{Principal: principal}

	if participants := strings.TrimSpace(values.Get("participants")); participants != "" {
		params.ParticipantIDs = parseCSV(participants)
	}

	if after := strings.TrimSpace(values.Get("starts_after")); after != "" {
		if ts, err := time.Parse(time.RFC3339Nano, after); err == nil {
			params.StartsAfter = &ts
		} else if ts, err := time.Parse(time.RFC3339, after); err == nil {
			params.StartsAfter = &ts
		}
	}

	if before := strings.TrimSpace(values.Get("ends_before")); before != "" {
		if ts, err := time.Parse(time.RFC3339Nano, before); err == nil {
			params.EndsBefore = &ts
		} else if ts, err := time.Parse(time.RFC3339, before); err == nil {
			params.EndsBefore = &ts
		}
	}

	if day := strings.TrimSpace(values.Get("day")); day != "" {
		if ts, err := time.Parse("2006-01-02", day); err == nil {
			params.Period = application.ListPeriodDay
			params.PeriodReference = ts
		}
	} else if week := strings.TrimSpace(values.Get("week")); week != "" {
		if ts, err := time.Parse("2006-01-02", week); err == nil {
			params.Period = application.ListPeriodWeek
			params.PeriodReference = ts
		}
	} else if month := strings.TrimSpace(values.Get("month")); month != "" {
		if ts, err := time.Parse("2006-01", month); err == nil {
			params.Period = application.ListPeriodMonth
			params.PeriodReference = ts
		}
	}

	if len(params.ParticipantIDs) == 0 {
		if principal.UserID != "" {
			params.ParticipantIDs = []string{principal.UserID}
		} else {
			params.ParticipantIDs = nil
		}
	}

	return params
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
