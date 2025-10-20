package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/enterprise-scheduler/internal/application"
)

func TestSessionMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("rejects requests without valid session tokens", func(t *testing.T) {
		t.Parallel()

		buf := &bytes.Buffer{}
		logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{ReplaceAttr: removeTimeAttr}))
		validator := &fakeSessionValidator{}

		handler := RequireSession(validator, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called when authentication fails")
		}))

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401 Unauthorized, got %d", res.StatusCode)
		}
		if validator.calls != 0 {
			t.Fatalf("expected validator not to be invoked, got %d", validator.calls)
		}

		var payload errorResponse
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode error response: %v", err)
		}
		if payload.ErrorCode != "AUTH_SESSION_EXPIRED" {
			t.Fatalf("expected error code AUTH_SESSION_EXPIRED, got %q", payload.ErrorCode)
		}
		if payload.Message != "認証トークンを指定してください" {
			t.Fatalf("unexpected error message: %q", payload.Message)
		}

		entries := parseLogEntries(t, buf)
		var found bool
		for _, entry := range entries {
			if entry["middleware"] == "RequireSession" && entry["error_kind"] == "unauthorized" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected unauthorized log entry, got %v", entries)
		}
	})

	t.Run("attaches authenticated principal to request context", func(t *testing.T) {
		t.Parallel()

		principal := application.Principal{UserID: "employee-123", IsAdmin: true}

		buf := &bytes.Buffer{}
		logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{ReplaceAttr: removeTimeAttr}))
		validator := &fakeSessionValidator{principal: principal}

		req := httptest.NewRequest(http.MethodGet, "/protected", nil).WithContext(context.Background())
		req.AddCookie(&http.Cookie{Name: "session_token", Value: "valid-token"})

		recorder := httptest.NewRecorder()

		capturedPrincipal := make(chan application.Principal, 1)

		middleware := RequireSession(validator, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFromContext(r.Context())
			if !ok {
				t.Fatal("expected principal in context")
			}
			capturedPrincipal <- p
			w.WriteHeader(http.StatusOK)
		}))

		middleware.ServeHTTP(recorder, req)

		res := recorder.Result()
		t.Cleanup(func() { _ = res.Body.Close() })

		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", res.StatusCode)
		}
		if validator.calls != 1 {
			t.Fatalf("expected validator to be invoked once, got %d", validator.calls)
		}

		select {
		case got := <-capturedPrincipal:
			if got.UserID != principal.UserID || got.IsAdmin != principal.IsAdmin {
				t.Fatalf("unexpected principal: %#v", got)
			}
		default:
			t.Fatal("expected principal to be captured")
		}

		entries := parseLogEntries(t, buf)
		var found bool
		for _, entry := range entries {
			if entry["middleware"] == "RequireSession" && entry["user_id"] == principal.UserID {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected success audit log, got %v", entries)
		}
	})

	t.Run("propagates validation errors from session service", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name           string
			err            error
			expectedStatus int
			expectedCode   string
			expectedBody   string
		}{
			{
				name:           "unauthorized",
				err:            application.ErrUnauthorized,
				expectedStatus: http.StatusUnauthorized,
				expectedCode:   "AUTH_SESSION_EXPIRED",
				expectedBody:   "セッションの有効期限が切れています",
			},
			{
				name:           "not found",
				err:            application.ErrNotFound,
				expectedStatus: http.StatusUnauthorized,
				expectedCode:   "AUTH_SESSION_EXPIRED",
				expectedBody:   "セッションの有効期限が切れています",
			},
			{
				name:           "expired",
				err:            application.ErrSessionExpired,
				expectedStatus: http.StatusUnauthorized,
				expectedCode:   "AUTH_SESSION_EXPIRED",
				expectedBody:   "セッションの有効期限が切れています",
			},
			{
				name:           "revoked",
				err:            application.ErrSessionRevoked,
				expectedStatus: http.StatusUnauthorized,
				expectedCode:   "AUTH_SESSION_EXPIRED",
				expectedBody:   "セッションの有効期限が切れています",
			},
			{
				name:           "unexpected",
				err:            errors.New("boom"),
				expectedStatus: http.StatusInternalServerError,
				expectedCode:   "INTERNAL_ERROR",
				expectedBody:   "セッション検証中にエラーが発生しました",
			},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				buf := &bytes.Buffer{}
				logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{ReplaceAttr: removeTimeAttr}))
				validator := &fakeSessionValidator{err: tc.err}

				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.AddCookie(&http.Cookie{Name: "session_token", Value: "token"})
				recorder := httptest.NewRecorder()

				nextCalled := false
				handler := RequireSession(validator, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					nextCalled = true
				}))

				handler.ServeHTTP(recorder, req)

				res := recorder.Result()
				t.Cleanup(func() { _ = res.Body.Close() })

				if nextCalled {
					t.Fatal("expected next handler not to be invoked on validation failure")
				}

				if res.StatusCode != tc.expectedStatus {
					t.Fatalf("expected status %d, got %d", tc.expectedStatus, res.StatusCode)
				}

				var payload errorResponse
				if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if payload.ErrorCode != tc.expectedCode {
					t.Fatalf("unexpected error code: %q", payload.ErrorCode)
				}
				if payload.Message != tc.expectedBody {
					t.Fatalf("unexpected error message: %q", payload.Message)
				}

				entries := parseLogEntries(t, buf)
				var found bool
				for _, entry := range entries {
					if entry["middleware"] == "RequireSession" && entry["error_kind"] == application.ErrorKind(tc.err) {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected audit log for error, got %v", entries)
				}
			})
		}
	})
}

type fakeSessionValidator struct {
	principal application.Principal
	err       error
	calls     int
}

func (f *fakeSessionValidator) ValidateSession(ctx context.Context, token string) (application.Principal, error) {
	f.calls++
	if f.err != nil {
		return application.Principal{}, f.err
	}
	return f.principal, nil
}

func parseLogEntries(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	raw := strings.TrimSpace(buf.String())
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to decode log entry %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func removeTimeAttr(groups []string, attr slog.Attr) slog.Attr {
	if attr.Key == "time" {
		return slog.Attr{}
	}
	return attr
}
