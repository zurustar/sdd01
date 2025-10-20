package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
)

func TestAuthHandlers(t *testing.T) {
	t.Run("login issues session token via cookie and header", func(t *testing.T) {
		issuedAt := time.Date(2024, 4, 1, 15, 0, 0, 0, time.UTC)
		service := &fakeAuthService{
			authenticateFunc: func(ctx context.Context, params application.AuthenticateParams) (application.AuthenticateResult, error) {
				if params.Email != "alice@example.com" {
					t.Fatalf("expected trimmed email, got %q", params.Email)
				}
				if params.Password != "correcthorsebatterystaple" {
					t.Fatalf("unexpected password: %q", params.Password)
				}
				return application.AuthenticateResult{
					User: application.User{ID: "user-1", IsAdmin: true},
					Session: application.Session{
						Token:     "session-token",
						ExpiresAt: issuedAt,
					},
				}, nil
			},
		}

		handler := NewAuthHandler(service, nil)

		credentials := map[string]string{"email": "  alice@example.com  ", "password": "correcthorsebatterystaple"}
		body, err := json.Marshal(credentials)
		if err != nil {
			t.Fatalf("failed to marshal credentials: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		recorder := httptest.NewRecorder()

		handler.Login(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", res.StatusCode)
		}

		if token := res.Header.Get("X-Session-Token"); token != "session-token" {
			t.Fatalf("expected header token %q, got %q", "session-token", token)
		}

		var sessionCookie *http.Cookie
		for _, cookie := range res.Cookies() {
			if cookie.Name == "session_token" {
				sessionCookie = cookie
				break
			}
		}
		if sessionCookie == nil {
			t.Fatalf("expected session_token cookie to be set")
		}
		if sessionCookie.Value != "session-token" {
			t.Fatalf("expected cookie value session-token, got %q", sessionCookie.Value)
		}
		if !sessionCookie.HttpOnly {
			t.Fatalf("expected HttpOnly cookie")
		}
		if !sessionCookie.Secure {
			t.Fatalf("expected Secure cookie")
		}
		if sessionCookie.Path != "/" {
			t.Fatalf("expected cookie path '/', got %q", sessionCookie.Path)
		}
		if !sessionCookie.Expires.Equal(issuedAt) {
			t.Fatalf("expected cookie expiry %v, got %v", issuedAt, sessionCookie.Expires)
		}

		var payload struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expires_at"`
			Principal struct {
				UserID  string `json:"user_id"`
				IsAdmin bool   `json:"is_admin"`
			} `json:"principal"`
		}
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if payload.Token != "session-token" {
			t.Fatalf("expected payload token session-token, got %q", payload.Token)
		}
		if payload.Principal.UserID != "user-1" {
			t.Fatalf("expected principal user_id user-1, got %q", payload.Principal.UserID)
		}
		if !payload.Principal.IsAdmin {
			t.Fatalf("expected principal is_admin true")
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, payload.ExpiresAt)
		if err != nil {
			t.Fatalf("failed to parse expires_at: %v", err)
		}
		if !expiresAt.Equal(issuedAt) {
			t.Fatalf("expected expires_at %v, got %v", issuedAt, expiresAt)
		}
	})

	t.Run("logout revokes the session", func(t *testing.T) {
		var revokedToken string
		service := &fakeAuthService{
			revokeFunc: func(ctx context.Context, token string) error {
				revokedToken = token
				return nil
			},
		}

		handler := NewAuthHandler(service, nil)

		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		req.Header.Set("Authorization", "Bearer header-token")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: "cookie-token"})
		recorder := httptest.NewRecorder()

		handler.Logout(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("expected status 204 No Content, got %d", res.StatusCode)
		}
		if revokedToken != "header-token" {
			t.Fatalf("expected token header-token to be revoked, got %q", revokedToken)
		}

		var clearCookie *http.Cookie
		for _, cookie := range res.Cookies() {
			if cookie.Name == "session_token" {
				clearCookie = cookie
				break
			}
		}
		if clearCookie == nil {
			t.Fatalf("expected clearing cookie to be set")
		}
		if clearCookie.Value != "" {
			t.Fatalf("expected cleared cookie value to be empty, got %q", clearCookie.Value)
		}
		if clearCookie.MaxAge != -1 {
			t.Fatalf("expected cleared cookie MaxAge -1, got %d", clearCookie.MaxAge)
		}
		if clearCookie.Expires.IsZero() || clearCookie.Expires.After(time.Now().Add(-time.Minute)) {
			t.Fatalf("expected cleared cookie to be expired, got %v", clearCookie.Expires)
		}
	})
}

func TestUserHandlers(t *testing.T) {
	t.Run("require administrator authorization", func(t *testing.T) {
		var capturedPrincipal application.Principal
		service := &fakeUserService{
			createUserFunc: func(ctx context.Context, params application.CreateUserParams) (application.User, error) {
				capturedPrincipal = params.Principal
				return application.User{}, application.ErrUnauthorized
			},
		}

		handler := NewUserHandler(service, nil)

		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(`{"email":"bob@example.com","display_name":"Bob"}`))
		req = req.WithContext(ContextWithPrincipal(req.Context(), application.Principal{UserID: "employee", IsAdmin: false}))
		recorder := httptest.NewRecorder()

		handler.Create(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("expected status 403 Forbidden, got %d", res.StatusCode)
		}

		if capturedPrincipal.UserID != "employee" || capturedPrincipal.IsAdmin {
			t.Fatalf("unexpected principal captured: %#v", capturedPrincipal)
		}

		var payload struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if payload.Message != "この操作を実行する権限がありません。" {
			t.Fatalf("unexpected error message: %q", payload.Message)
		}
	})

	t.Run("return localized validation errors", func(t *testing.T) {
		service := &fakeUserService{
			createUserFunc: func(ctx context.Context, params application.CreateUserParams) (application.User, error) {
				return application.User{}, &application.ValidationError{FieldErrors: map[string]string{
					"email":        "email is invalid",
					"display_name": "display name is required",
				}}
			},
		}

		handler := NewUserHandler(service, nil)

		invalidPayload := map[string]any{"email": "bad", "display_name": ""}
		body, err := json.Marshal(invalidPayload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req = req.WithContext(ContextWithPrincipal(req.Context(), application.Principal{UserID: "admin", IsAdmin: true}))
		recorder := httptest.NewRecorder()

		handler.Create(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422 Unprocessable Entity, got %d", res.StatusCode)
		}

		var payload struct {
			Message string            `json:"message"`
			Errors  map[string]string `json:"errors"`
		}
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if payload.Message != "入力内容に誤りがあります。" {
			t.Fatalf("unexpected message: %q", payload.Message)
		}
		if payload.Errors["email"] != "メールアドレスの形式が不正です。" {
			t.Fatalf("unexpected email error: %q", payload.Errors["email"])
		}
		if payload.Errors["display_name"] != "表示名は必須です。" {
			t.Fatalf("unexpected display_name error: %q", payload.Errors["display_name"])
		}
	})
}

func TestScheduleHandlers(t *testing.T) {
	t.Run("enforce creator authorization rules", func(t *testing.T) {
		service := &fakeScheduleService{
			deleteScheduleFunc: func(ctx context.Context, principal application.Principal, scheduleID string) error {
				if principal.UserID != "user-2" {
					t.Fatalf("unexpected principal: %#v", principal)
				}
				if scheduleID != "sched-123" {
					t.Fatalf("unexpected schedule id: %s", scheduleID)
				}
				return application.ErrUnauthorized
			},
		}

		handler := NewScheduleHandler(service, nil)

		req := httptest.NewRequest(http.MethodDelete, "/schedules/sched-123", nil)
		ctx := ContextWithPrincipal(req.Context(), application.Principal{UserID: "user-2", IsAdmin: false})
		ctx = ContextWithScheduleID(ctx, "sched-123")
		req = req.WithContext(ctx)
		recorder := httptest.NewRecorder()

		handler.Delete(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("expected status 403 Forbidden, got %d", res.StatusCode)
		}

		var payload struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if payload.Message != "この操作を実行する権限がありません。" {
			t.Fatalf("unexpected message: %q", payload.Message)
		}
	})

	t.Run("serialize conflict warnings in responses", func(t *testing.T) {
		roomID := "room-1"
		warnings := []application.ConflictWarning{
			{ScheduleID: "existing-1", Type: "participant", ParticipantID: "user-2"},
			{ScheduleID: "existing-2", Type: "room", RoomID: &roomID},
		}

		service := &fakeScheduleService{
			createScheduleFunc: func(ctx context.Context, params application.CreateScheduleParams) (application.Schedule, []application.ConflictWarning, error) {
				return application.Schedule{
					ID:               "schedule-new",
					CreatorID:        params.Principal.UserID,
					Title:            params.Input.Title,
					Description:      params.Input.Description,
					Start:            params.Input.Start,
					End:              params.Input.End,
					RoomID:           params.Input.RoomID,
					WebConferenceURL: params.Input.WebConferenceURL,
					ParticipantIDs:   params.Input.ParticipantIDs,
					CreatedAt:        mustParse(t, "2024-04-01T00:00:00Z"),
					UpdatedAt:        mustParse(t, "2024-04-01T00:00:00Z"),
				}, warnings, nil
			},
		}

		handler := NewScheduleHandler(service, nil)

		payload := map[string]any{
			"title":              "Design sync",
			"description":        "Discuss roadmap",
			"start":              "2024-04-01T01:00:00Z",
			"end":                "2024-04-01T02:00:00Z",
			"participant_ids":    []string{"user-1", "user-2"},
			"web_conference_url": "https://meet.example.com/rooms/123",
		}

		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/schedules", bytes.NewReader(body))
		req = req.WithContext(ContextWithPrincipal(req.Context(), application.Principal{UserID: "user-1"}))
		recorder := httptest.NewRecorder()

		handler.Create(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201 Created, got %d", res.StatusCode)
		}

		var decoded scheduleResponse
		if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(decoded.Warnings) != len(warnings) {
			t.Fatalf("expected %d warnings, got %d", len(warnings), len(decoded.Warnings))
		}

		warningByType := map[string]conflictWarningDTO{}
		for _, warning := range decoded.Warnings {
			warningByType[warning.Type] = warning
		}

		if participant, ok := warningByType["participant"]; !ok || participant.ParticipantID != "user-2" {
			t.Fatalf("expected participant warning for user-2, got %v", participant)
		}

		if room, ok := warningByType["room"]; !ok || room.RoomID == nil || *room.RoomID != roomID {
			t.Fatalf("expected room warning for %s, got %v", roomID, room.RoomID)
		}
	})

	t.Run("serialize conflict warnings in update responses", func(t *testing.T) {
		roomID := "room-99"
		warnings := []application.ConflictWarning{
			{ScheduleID: "existing-3", Type: "participant", ParticipantID: "user-5"},
			{ScheduleID: "existing-4", Type: "room", RoomID: &roomID},
		}

		service := &fakeScheduleService{
			updateScheduleFunc: func(ctx context.Context, params application.UpdateScheduleParams) (application.Schedule, []application.ConflictWarning, error) {
				return application.Schedule{
					ID:               params.ScheduleID,
					CreatorID:        params.Principal.UserID,
					Title:            params.Input.Title,
					Description:      params.Input.Description,
					Start:            params.Input.Start,
					End:              params.Input.End,
					RoomID:           params.Input.RoomID,
					WebConferenceURL: params.Input.WebConferenceURL,
					ParticipantIDs:   params.Input.ParticipantIDs,
					CreatedAt:        mustParse(t, "2024-04-01T00:00:00Z"),
					UpdatedAt:        mustParse(t, "2024-04-02T00:00:00Z"),
				}, warnings, nil
			},
		}

		handler := NewScheduleHandler(service, nil)

		payload := map[string]any{
			"title":              "Weekly sync",
			"description":        "Project update",
			"start":              "2024-04-02T01:00:00Z",
			"end":                "2024-04-02T02:00:00Z",
			"participant_ids":    []string{"user-1", "user-5"},
			"web_conference_url": "https://meet.example.com/rooms/456",
		}

		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}

		req := httptest.NewRequest(http.MethodPut, "/schedules/schedule-1", bytes.NewReader(body))
		ctx := ContextWithPrincipal(req.Context(), application.Principal{UserID: "user-1"})
		ctx = ContextWithScheduleID(ctx, "schedule-1")
		req = req.WithContext(ctx)
		recorder := httptest.NewRecorder()

		handler.Update(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", res.StatusCode)
		}

		var decoded scheduleResponse
		if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(decoded.Warnings) != len(warnings) {
			t.Fatalf("expected %d warnings, got %d", len(warnings), len(decoded.Warnings))
		}

		warningByType := map[string]conflictWarningDTO{}
		for _, warning := range decoded.Warnings {
			warningByType[warning.Type] = warning
		}

		if participant, ok := warningByType["participant"]; !ok || participant.ParticipantID != "user-5" {
			t.Fatalf("expected participant warning for user-5, got %v", participant)
		}

		if room, ok := warningByType["room"]; !ok || room.RoomID == nil || *room.RoomID != roomID {
			t.Fatalf("expected room warning for %s, got %v", roomID, room.RoomID)
		}
	})

	t.Run("expand recurrences in list responses", func(t *testing.T) {
		occurrenceStart := mustParse(t, "2024-05-01T09:00:00+09:00")
		occurrenceEnd := mustParse(t, "2024-05-01T10:00:00+09:00")
		service := &fakeScheduleService{
			listSchedulesFunc: func(ctx context.Context, params application.ListSchedulesParams) ([]application.Schedule, []application.ConflictWarning, error) {
				return []application.Schedule{
					{
						ID:          "sched-1",
						CreatorID:   "user-1",
						Title:       "Weekly Sync",
						Description: "Discuss updates",
						Start:       mustParse(t, "2024-05-01T09:00:00+09:00"),
						End:         mustParse(t, "2024-05-01T10:00:00+09:00"),
						ParticipantIDs: []string{
							"user-1", "user-2",
						},
						CreatedAt: mustParse(t, "2024-04-01T00:00:00Z"),
						UpdatedAt: mustParse(t, "2024-04-02T00:00:00Z"),
						Occurrences: []application.ScheduleOccurrence{{
							ScheduleID: "sched-1",
							RuleID:     "rule-1",
							Start:      occurrenceStart,
							End:        occurrenceEnd,
						}},
					},
				}, nil, nil
			},
		}

		handler := NewScheduleHandler(service, nil)

		req := httptest.NewRequest(http.MethodGet, "/schedules", nil)
		req = req.WithContext(ContextWithPrincipal(req.Context(), application.Principal{UserID: "user-1"}))
		recorder := httptest.NewRecorder()

		handler.List(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", res.StatusCode)
		}

		var payload listSchedulesResponse
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(payload.Schedules) != 1 {
			t.Fatalf("expected 1 schedule, got %d", len(payload.Schedules))
		}
		schedule := payload.Schedules[0]
		if len(schedule.Occurrences) != 1 {
			t.Fatalf("expected 1 occurrence, got %d", len(schedule.Occurrences))
		}
		occurrence := schedule.Occurrences[0]
		if occurrence.ScheduleID != "sched-1" {
			t.Fatalf("unexpected schedule_id: %q", occurrence.ScheduleID)
		}
		if occurrence.RuleID != "rule-1" {
			t.Fatalf("unexpected rule_id: %q", occurrence.RuleID)
		}
		start, err := time.Parse(time.RFC3339Nano, occurrence.Start)
		if err != nil {
			t.Fatalf("failed to parse occurrence start: %v", err)
		}
		if !start.Equal(occurrenceStart.UTC()) {
			t.Fatalf("unexpected occurrence start: %v", start)
		}
		end, err := time.Parse(time.RFC3339Nano, occurrence.End)
		if err != nil {
			t.Fatalf("failed to parse occurrence end: %v", err)
		}
		if !end.Equal(occurrenceEnd.UTC()) {
			t.Fatalf("unexpected occurrence end: %v", end)
		}
	})

	t.Run("map service sentinel errors to HTTP status codes", func(t *testing.T) {
		cases := []struct {
			name     string
			err      error
			expected int
			message  string
		}{
			{name: "unauthorized", err: application.ErrUnauthorized, expected: http.StatusForbidden, message: "この操作を実行する権限がありません。"},
			{name: "not found", err: application.ErrNotFound, expected: http.StatusNotFound, message: "指定されたリソースが見つかりません。"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				service := &fakeScheduleService{
					updateScheduleFunc: func(ctx context.Context, params application.UpdateScheduleParams) (application.Schedule, []application.ConflictWarning, error) {
						return application.Schedule{}, nil, tc.err
					},
				}

				handler := NewScheduleHandler(service, nil)

				req := httptest.NewRequest(http.MethodPut, "/schedules/sched-999", bytes.NewReader([]byte(`{"title":"Update"}`)))
				ctx := ContextWithPrincipal(req.Context(), application.Principal{UserID: "user-1"})
				ctx = ContextWithScheduleID(ctx, "sched-999")
				req = req.WithContext(ctx)
				recorder := httptest.NewRecorder()

				handler.Update(recorder, req)

				res := recorder.Result()
				t.Cleanup(func() { _ = res.Body.Close() })

				if res.StatusCode != tc.expected {
					t.Fatalf("expected status %d, got %d", tc.expected, res.StatusCode)
				}

				var payload struct {
					Message string `json:"message"`
				}
				if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if payload.Message != tc.message {
					t.Fatalf("unexpected message: %q", payload.Message)
				}
			})
		}
	})

	t.Run("map day, week, and month query parameters to filter options", func(t *testing.T) {
		var captured application.ListSchedulesParams
		service := &fakeScheduleService{
			listSchedulesFunc: func(ctx context.Context, params application.ListSchedulesParams) ([]application.Schedule, []application.ConflictWarning, error) {
				captured = params
				return nil, nil, nil
			},
		}

		handler := NewScheduleHandler(service, nil)

		values := url.Values{}
		values.Set("day", "2024-04-01")
		values.Set("week", "2024-04-01")
		values.Set("month", "2024-04")
		req := httptest.NewRequest(http.MethodGet, "/schedules?"+values.Encode(), nil)
		req = req.WithContext(ContextWithPrincipal(req.Context(), application.Principal{UserID: "user-1"}))
		recorder := httptest.NewRecorder()

		handler.List(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if captured.Principal.UserID != "user-1" {
			t.Fatalf("expected principal user-1, got %#v", captured.Principal)
		}
		if captured.Period != application.ListPeriodDay {
			t.Fatalf("expected period day, got %q", captured.Period)
		}
		expectedRef, err := time.Parse("2006-01-02", "2024-04-01")
		if err != nil {
			t.Fatalf("failed to parse expected reference: %v", err)
		}
		if !captured.PeriodReference.Equal(expectedRef) {
			t.Fatalf("unexpected period reference: %v", captured.PeriodReference)
		}
	})

	t.Run("default list view returns only caller's schedules", func(t *testing.T) {
		var captured application.ListSchedulesParams
		service := &fakeScheduleService{
			listSchedulesFunc: func(ctx context.Context, params application.ListSchedulesParams) ([]application.Schedule, []application.ConflictWarning, error) {
				captured = params
				return nil, nil, nil
			},
		}

		handler := NewScheduleHandler(service, nil)

		principal := application.Principal{UserID: "user-1"}
		req := httptest.NewRequest(http.MethodGet, "/schedules", nil)
		req = req.WithContext(ContextWithPrincipal(req.Context(), principal))
		recorder := httptest.NewRecorder()

		handler.List(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if !slicesEqual(captured.ParticipantIDs, []string{"user-1"}) {
			t.Fatalf("expected participant filter [user-1], got %v", captured.ParticipantIDs)
		}
	})

	t.Run("explicit colleague filter exposes shared calendars", func(t *testing.T) {
		var captured application.ListSchedulesParams
		service := &fakeScheduleService{
			listSchedulesFunc: func(ctx context.Context, params application.ListSchedulesParams) ([]application.Schedule, []application.ConflictWarning, error) {
				captured = params
				return nil, nil, nil
			},
		}

		handler := NewScheduleHandler(service, nil)

		req := httptest.NewRequest(http.MethodGet, "/schedules?participants=user-2,user-3", nil)
		req = req.WithContext(ContextWithPrincipal(req.Context(), application.Principal{UserID: "user-1"}))
		recorder := httptest.NewRecorder()

		handler.List(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if !slicesEqual(captured.ParticipantIDs, []string{"user-2", "user-3"}) {
			t.Fatalf("expected participants [user-2 user-3], got %v", captured.ParticipantIDs)
		}
	})

	t.Run("missing or forbidden schedules map to 404 or 403", func(t *testing.T) {
		cases := []struct {
			name     string
			err      error
			expected int
			message  string
		}{
			{name: "forbidden", err: application.ErrUnauthorized, expected: http.StatusForbidden, message: "この操作を実行する権限がありません。"},
			{name: "missing", err: application.ErrNotFound, expected: http.StatusNotFound, message: "指定されたリソースが見つかりません。"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				service := &fakeScheduleService{
					deleteScheduleFunc: func(ctx context.Context, principal application.Principal, scheduleID string) error {
						return tc.err
					},
				}

				handler := NewScheduleHandler(service, nil)

				req := httptest.NewRequest(http.MethodDelete, "/schedules/any", nil)
				ctx := ContextWithPrincipal(req.Context(), application.Principal{UserID: "user-1"})
				ctx = ContextWithScheduleID(ctx, "any")
				req = req.WithContext(ctx)
				recorder := httptest.NewRecorder()

				handler.Delete(recorder, req)

				res := recorder.Result()
				t.Cleanup(func() { _ = res.Body.Close() })

				if res.StatusCode != tc.expected {
					t.Fatalf("expected status %d, got %d", tc.expected, res.StatusCode)
				}

				var payload struct {
					Message string `json:"message"`
				}
				if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if payload.Message != tc.message {
					t.Fatalf("unexpected message: %q", payload.Message)
				}
			})
		}
	})
}

func TestRoomHandlers(t *testing.T) {
	t.Run("rejects unauthenticated requests for list", func(t *testing.T) {
		service := &fakeRoomService{
			listRoomsFunc: func(ctx context.Context, principal application.Principal) ([]application.Room, error) {
				t.Fatal("ListRooms should not be called when principal is missing")
				return nil, nil
			},
		}

		handler := NewRoomHandler(service, nil)

		req := httptest.NewRequest(http.MethodGet, "/rooms", nil)
		recorder := httptest.NewRecorder()

		handler.List(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401 Unauthorized, got %d", res.StatusCode)
		}

		var payload errorResponse
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode error response: %v", err)
		}
		if payload.Message != "セッショントークンが指定されていません。" {
			t.Fatalf("unexpected error message: %q", payload.Message)
		}
	})

	t.Run("allow non-admins to list rooms", func(t *testing.T) {
		var capturedPrincipal application.Principal
		service := &fakeRoomService{
			listRoomsFunc: func(ctx context.Context, principal application.Principal) ([]application.Room, error) {
				capturedPrincipal = principal
				return []application.Room{{
					ID:        "room-1",
					Name:      "会議室A",
					Location:  "東京オフィス",
					Capacity:  10,
					CreatedAt: mustParse(t, "2024-04-01T00:00:00Z"),
					UpdatedAt: mustParse(t, "2024-04-01T00:00:00Z"),
				}}, nil
			},
		}

		handler := NewRoomHandler(service, nil)

		principal := application.Principal{UserID: "employee", IsAdmin: false}
		req := httptest.NewRequest(http.MethodGet, "/rooms", nil)
		req = req.WithContext(ContextWithPrincipal(req.Context(), principal))
		recorder := httptest.NewRecorder()

		handler.List(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if capturedPrincipal.UserID != "employee" || capturedPrincipal.IsAdmin {
			t.Fatalf("unexpected principal captured: %#v", capturedPrincipal)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", res.StatusCode)
		}

		var payload listRoomsResponse
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(payload.Rooms) != 1 || payload.Rooms[0].ID != "room-1" {
			t.Fatalf("unexpected rooms payload: %#v", payload.Rooms)
		}
	})

	t.Run("require admin role for mutations", func(t *testing.T) {
		t.Run("non-admin receives forbidden", func(t *testing.T) {
			service := &fakeRoomService{
				createRoomFunc: func(ctx context.Context, params application.CreateRoomParams) (application.Room, error) {
					return application.Room{}, application.ErrUnauthorized
				},
			}

			handler := NewRoomHandler(service, nil)

			req := httptest.NewRequest(http.MethodPost, "/rooms", bytes.NewReader([]byte(`{"name":"会議室"}`)))
			req = req.WithContext(ContextWithPrincipal(req.Context(), application.Principal{UserID: "employee", IsAdmin: false}))
			recorder := httptest.NewRecorder()

			handler.Create(recorder, req)

			res := recorder.Result()
			t.Cleanup(func() { _ = res.Body.Close() })

			if res.StatusCode != http.StatusForbidden {
				t.Fatalf("expected status 403 Forbidden, got %d", res.StatusCode)
			}
		})

		t.Run("admin can create room", func(t *testing.T) {
			service := &fakeRoomService{
				createRoomFunc: func(ctx context.Context, params application.CreateRoomParams) (application.Room, error) {
					if !params.Principal.IsAdmin {
						t.Fatalf("expected admin principal, got %#v", params.Principal)
					}
					return application.Room{
						ID:        "room-2",
						Name:      "大会議室",
						Location:  "大阪オフィス",
						Capacity:  20,
						CreatedAt: mustParse(t, "2024-04-01T00:00:00Z"),
						UpdatedAt: mustParse(t, "2024-04-01T00:00:00Z"),
					}, nil
				},
			}

			handler := NewRoomHandler(service, nil)

			body := []byte(`{"name":"大会議室","location":"大阪オフィス","capacity":20}`)
			req := httptest.NewRequest(http.MethodPost, "/rooms", bytes.NewReader(body))
			req = req.WithContext(ContextWithPrincipal(req.Context(), application.Principal{UserID: "admin", IsAdmin: true}))
			recorder := httptest.NewRecorder()

			handler.Create(recorder, req)

			res := recorder.Result()
			t.Cleanup(func() { _ = res.Body.Close() })

			if res.StatusCode != http.StatusCreated {
				t.Fatalf("expected status 201 Created, got %d", res.StatusCode)
			}

			var payload roomResponse
			if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if payload.Room.ID != "room-2" {
				t.Fatalf("unexpected room payload: %#v", payload.Room)
			}
		})
	})
}

type fakeAuthService struct {
	authenticateFunc func(context.Context, application.AuthenticateParams) (application.AuthenticateResult, error)
	revokeFunc       func(context.Context, string) error
}

func (f *fakeAuthService) Authenticate(ctx context.Context, params application.AuthenticateParams) (application.AuthenticateResult, error) {
	if f.authenticateFunc != nil {
		return f.authenticateFunc(ctx, params)
	}
	return application.AuthenticateResult{}, nil
}

func (f *fakeAuthService) RevokeSession(ctx context.Context, token string) error {
	if f.revokeFunc != nil {
		return f.revokeFunc(ctx, token)
	}
	return nil
}

type fakeUserService struct {
	createUserFunc func(context.Context, application.CreateUserParams) (application.User, error)
	updateUserFunc func(context.Context, application.UpdateUserParams) (application.User, error)
	deleteUserFunc func(context.Context, application.Principal, string) error
	listUsersFunc  func(context.Context, application.Principal) ([]application.User, error)
}

func (f *fakeUserService) CreateUser(ctx context.Context, params application.CreateUserParams) (application.User, error) {
	if f.createUserFunc != nil {
		return f.createUserFunc(ctx, params)
	}
	return application.User{}, nil
}

func (f *fakeUserService) UpdateUser(ctx context.Context, params application.UpdateUserParams) (application.User, error) {
	if f.updateUserFunc != nil {
		return f.updateUserFunc(ctx, params)
	}
	return application.User{}, nil
}

func (f *fakeUserService) DeleteUser(ctx context.Context, principal application.Principal, userID string) error {
	if f.deleteUserFunc != nil {
		return f.deleteUserFunc(ctx, principal, userID)
	}
	return nil
}

func (f *fakeUserService) ListUsers(ctx context.Context, principal application.Principal) ([]application.User, error) {
	if f.listUsersFunc != nil {
		return f.listUsersFunc(ctx, principal)
	}
	return nil, nil
}

type fakeRoomService struct {
	createRoomFunc func(context.Context, application.CreateRoomParams) (application.Room, error)
	updateRoomFunc func(context.Context, application.UpdateRoomParams) (application.Room, error)
	deleteRoomFunc func(context.Context, application.Principal, string) error
	listRoomsFunc  func(context.Context, application.Principal) ([]application.Room, error)
}

func (f *fakeRoomService) CreateRoom(ctx context.Context, params application.CreateRoomParams) (application.Room, error) {
	if f.createRoomFunc != nil {
		return f.createRoomFunc(ctx, params)
	}
	return application.Room{}, nil
}

func (f *fakeRoomService) UpdateRoom(ctx context.Context, params application.UpdateRoomParams) (application.Room, error) {
	if f.updateRoomFunc != nil {
		return f.updateRoomFunc(ctx, params)
	}
	return application.Room{}, nil
}

func (f *fakeRoomService) DeleteRoom(ctx context.Context, principal application.Principal, roomID string) error {
	if f.deleteRoomFunc != nil {
		return f.deleteRoomFunc(ctx, principal, roomID)
	}
	return nil
}

func (f *fakeRoomService) ListRooms(ctx context.Context, principal application.Principal) ([]application.Room, error) {
	if f.listRoomsFunc != nil {
		return f.listRoomsFunc(ctx, principal)
	}
	return nil, nil
}

type fakeScheduleService struct {
	createScheduleFunc func(context.Context, application.CreateScheduleParams) (application.Schedule, []application.ConflictWarning, error)
	updateScheduleFunc func(context.Context, application.UpdateScheduleParams) (application.Schedule, []application.ConflictWarning, error)
	deleteScheduleFunc func(context.Context, application.Principal, string) error
	listSchedulesFunc  func(context.Context, application.ListSchedulesParams) ([]application.Schedule, []application.ConflictWarning, error)
}

func (f *fakeScheduleService) CreateSchedule(ctx context.Context, params application.CreateScheduleParams) (application.Schedule, []application.ConflictWarning, error) {
	if f.createScheduleFunc != nil {
		return f.createScheduleFunc(ctx, params)
	}
	return application.Schedule{}, nil, nil
}

func (f *fakeScheduleService) UpdateSchedule(ctx context.Context, params application.UpdateScheduleParams) (application.Schedule, []application.ConflictWarning, error) {
	if f.updateScheduleFunc != nil {
		return f.updateScheduleFunc(ctx, params)
	}
	return application.Schedule{}, nil, nil
}

func (f *fakeScheduleService) DeleteSchedule(ctx context.Context, principal application.Principal, scheduleID string) error {
	if f.deleteScheduleFunc != nil {
		return f.deleteScheduleFunc(ctx, principal, scheduleID)
	}
	return nil
}

func (f *fakeScheduleService) ListSchedules(ctx context.Context, params application.ListSchedulesParams) ([]application.Schedule, []application.ConflictWarning, error) {
	if f.listSchedulesFunc != nil {
		return f.listSchedulesFunc(ctx, params)
	}
	return nil, nil, nil
}

func mustParse(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, value)
	}
	if err != nil {
		t.Fatalf("failed to parse time %s: %v", value, err)
	}
	return parsed
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
