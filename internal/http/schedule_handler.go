package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
)

// scheduleService defines the subset of application schedule operations required by the HTTP layer.
type scheduleService interface {
	CreateSchedule(ctx context.Context, params application.CreateScheduleParams) (application.Schedule, []application.ConflictWarning, error)
	UpdateSchedule(ctx context.Context, params application.UpdateScheduleParams) (application.Schedule, []application.ConflictWarning, error)
}

// ScheduleHandler exposes HTTP endpoints backed by the schedule service.
type ScheduleHandler struct {
	service scheduleService
}

// NewScheduleHandler wires dependencies for schedule endpoints.
func NewScheduleHandler(service scheduleService) *ScheduleHandler {
	return &ScheduleHandler{service: service}
}

// Create handles POST /schedules style requests.
func (h *ScheduleHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var req scheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())

	schedule, warnings, err := h.service.CreateSchedule(r.Context(), application.CreateScheduleParams{
		Principal: principal,
		Input:     req.toInput(),
	})
	if err != nil {
		h.renderError(w, err)
		return
	}

	h.renderSchedule(w, schedule, warnings, http.StatusCreated)
}

// Update handles PUT /schedules/{id} style requests.
func (h *ScheduleHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	scheduleID, ok := ScheduleIDFromContext(r.Context())
	if !ok || strings.TrimSpace(scheduleID) == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var req scheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())

	schedule, warnings, err := h.service.UpdateSchedule(r.Context(), application.UpdateScheduleParams{
		Principal:  principal,
		ScheduleID: scheduleID,
		Input:      req.toInput(),
	})
	if err != nil {
		h.renderError(w, err)
		return
	}

	h.renderSchedule(w, schedule, warnings, http.StatusOK)
}

func (h *ScheduleHandler) renderError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, application.ErrUnauthorized):
		status = http.StatusForbidden
	case errors.Is(err, application.ErrNotFound):
		status = http.StatusNotFound
	default:
		var vErr *application.ValidationError
		if errors.As(err, &vErr) {
			status = http.StatusUnprocessableEntity
		}
	}

	http.Error(w, http.StatusText(status), status)
}

func (h *ScheduleHandler) renderSchedule(w http.ResponseWriter, schedule application.Schedule, warnings []application.ConflictWarning, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	payload := scheduleResponse{
		Schedule: toScheduleDTO(schedule),
		Warnings: toWarningDTOs(warnings),
	}

	_ = json.NewEncoder(w).Encode(payload)
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
		CreatorID:        r.CreatorID,
		Title:            r.Title,
		Description:      r.Description,
		Start:            parseTime(r.Start),
		End:              parseTime(r.End),
		RoomID:           r.RoomID,
		WebConferenceURL: r.WebConferenceURL,
		ParticipantIDs:   r.ParticipantIDs,
	}
}

func parseTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
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

type scheduleDTO struct {
	ID               string   `json:"id"`
	CreatorID        string   `json:"creator_id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Start            string   `json:"start"`
	End              string   `json:"end"`
	RoomID           *string  `json:"room_id,omitempty"`
	WebConferenceURL string   `json:"web_conference_url,omitempty"`
	ParticipantIDs   []string `json:"participant_ids"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
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
	}
}

type conflictWarningDTO struct {
	ScheduleID    string  `json:"schedule_id"`
	Type          string  `json:"type"`
	ParticipantID string  `json:"participant_id,omitempty"`
	RoomID        *string `json:"room_id,omitempty"`
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
