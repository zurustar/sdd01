package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// CredentialStore exposes user credential lookup operations required by the auth service.
type CredentialStore interface {
	GetUserCredentialsByEmail(ctx context.Context, email string) (UserCredentials, error)
	GetUser(ctx context.Context, id string) (User, error)
}

// SessionRepository captures the persistence interactions for issued sessions.
type SessionRepository interface {
	CreateSession(ctx context.Context, session Session) (Session, error)
	GetSession(ctx context.Context, token string) (Session, error)
	UpdateSession(ctx context.Context, session Session) (Session, error)
	RevokeSession(ctx context.Context, token string, revokedAt time.Time) (Session, error)
	DeleteExpiredSessions(ctx context.Context, reference time.Time) error
}

// PasswordVerifier compares a stored hash with a candidate password.
type PasswordVerifier func(hashedPassword, password string) error

// AuthService coordinates authentication flows such as login and session refresh.
type AuthService struct {
	credentials    CredentialStore
	sessions       SessionRepository
	verifyPassword PasswordVerifier
	tokenGenerator func() string
	now            func() time.Time
	sessionTTL     time.Duration
	logger         *slog.Logger
}

// NewAuthService constructs an AuthService with the provided dependencies.
func NewAuthService(credentials CredentialStore, sessions SessionRepository, verify PasswordVerifier, tokenGenerator func() string, now func() time.Time, sessionTTL time.Duration) *AuthService {
	return NewAuthServiceWithLogger(credentials, sessions, verify, tokenGenerator, now, sessionTTL, nil)
}

// NewAuthServiceWithLogger constructs an AuthService with a specified logger.
func NewAuthServiceWithLogger(credentials CredentialStore, sessions SessionRepository, verify PasswordVerifier, tokenGenerator func() string, now func() time.Time, sessionTTL time.Duration, logger *slog.Logger) *AuthService {
	if verify == nil {
		verify = VerifyPassword
	}
	if tokenGenerator == nil {
		tokenGenerator = func() string { return "" }
	}
	if now == nil {
		now = time.Now
	}
	if sessionTTL <= 0 {
		sessionTTL = 24 * time.Hour
	}
	return &AuthService{
		credentials:    credentials,
		sessions:       sessions,
		verifyPassword: verify,
		tokenGenerator: tokenGenerator,
		now:            now,
		sessionTTL:     sessionTTL,
		logger:         defaultLogger(logger),
	}
}

func (s *AuthService) loggerWith(ctx context.Context, operation string, attrs ...any) *slog.Logger {
	return serviceLogger(ctx, s.logger, "AuthService", operation, attrs...)
}

// Authenticate validates credentials and issues a new session token.
func (s *AuthService) Authenticate(ctx context.Context, params AuthenticateParams) (result AuthenticateResult, err error) {
	if s == nil {
		err = fmt.Errorf("AuthService is nil")
		return
	}
	if s.credentials == nil {
		err = fmt.Errorf("credential store not configured")
		return
	}

	email := strings.TrimSpace(strings.ToLower(params.Email))
	password := params.Password

	logger := s.loggerWith(ctx, "Authenticate",
		"email", email,
	)
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "authentication failed", "error", err, "error_kind", ErrorKind(err))
			return
		}
		logger.With(
			"user_id", result.User.ID,
			"session_id", result.Session.ID,
		).InfoContext(ctx, "authentication succeeded")
	}()

	if email == "" || password == "" {
		err = ErrInvalidCredentials
		return
	}

	var creds UserCredentials
	creds, err = s.credentials.GetUserCredentialsByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			err = ErrInvalidCredentials
			return
		}
		return
	}

	if creds.Disabled {
		err = ErrAccountDisabled
		return
	}

	if err = s.verifyPassword(creds.PasswordHash, password); err != nil {
		err = ErrInvalidCredentials
		return
	}

	now := s.now()
	id := s.tokenGenerator()
	token := s.tokenGenerator()
	if token == "" {
		token = id
	}

	session := Session{
		ID:          id,
		UserID:      creds.User.ID,
		Token:       token,
		Fingerprint: strings.TrimSpace(params.Fingerprint),
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(s.sessionTTL),
	}

	if s.sessions != nil {
		if err = s.sessions.DeleteExpiredSessions(ctx, now); err != nil {
			return
		}

		var persisted Session
		persisted, err = s.sessions.CreateSession(ctx, session)
		if err != nil {
			return
		}
		session = persisted
	}

	result = AuthenticateResult{User: creds.User, Session: session}
	return
}

// RefreshSession rotates an existing session token, extending its validity window.
func (s *AuthService) RefreshSession(ctx context.Context, params RefreshSessionParams) (result RefreshSessionResult, err error) {
	if s == nil {
		err = fmt.Errorf("AuthService is nil")
		return
	}
	if s.sessions == nil {
		err = fmt.Errorf("session repository not configured")
		return
	}

	token := strings.TrimSpace(params.Token)
	logger := s.loggerWith(ctx, "RefreshSession",
		"token_provided", token != "",
	)
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "session refresh failed", "error", err, "error_kind", ErrorKind(err))
			return
		}
		logger.With(
			"session_id", result.Session.ID,
			"user_id", result.Session.UserID,
		).InfoContext(ctx, "session refreshed")
	}()

	if token == "" {
		err = ErrInvalidCredentials
		return
	}

	var session Session
	session, err = s.sessions.GetSession(ctx, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			err = ErrInvalidCredentials
			return
		}
		return
	}

	now := s.now()
	if session.RevokedAt != nil && !session.RevokedAt.IsZero() {
		err = ErrSessionRevoked
		return
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(now) {
		err = ErrSessionExpired
		return
	}

	newToken := s.tokenGenerator()
	if newToken == "" {
		newToken = session.Token
	}

	session.Token = newToken
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.sessionTTL)
	if fp := strings.TrimSpace(params.Fingerprint); fp != "" {
		session.Fingerprint = fp
	}

	session, err = s.sessions.UpdateSession(ctx, session)
	if err != nil {
		return
	}

	result = RefreshSessionResult{Session: session}
	return
}

// RevokeSession invalidates an existing session token.
func (s *AuthService) RevokeSession(ctx context.Context, token string) error {
	if s == nil {
		return fmt.Errorf("AuthService is nil")
	}
	if s.sessions == nil {
		return fmt.Errorf("session repository not configured")
	}

	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ErrInvalidCredentials
	}

	logger := s.loggerWith(ctx, "RevokeSession", "token_provided", trimmed != "")

	if _, err := s.sessions.RevokeSession(ctx, trimmed, s.now()); err != nil {
		if errors.Is(err, ErrNotFound) {
			logger.ErrorContext(ctx, "failed to revoke session", "error", ErrInvalidCredentials, "error_kind", ErrorKind(ErrInvalidCredentials))
			return ErrInvalidCredentials
		}
		logger.ErrorContext(ctx, "failed to revoke session", "error", err, "error_kind", ErrorKind(err))
		return err
	}

	if err := s.sessions.DeleteExpiredSessions(ctx, s.now()); err != nil {
		logger.ErrorContext(ctx, "failed to prune expired sessions", "error", err, "error_kind", ErrorKind(err))
		return err
	}
	logger.InfoContext(ctx, "session revoked")
	return nil
}

// ValidateSession verifies that the provided token corresponds to an active session and returns its principal.
func (s *AuthService) ValidateSession(ctx context.Context, token string) (principal Principal, err error) {
	if s == nil {
		err = fmt.Errorf("AuthService is nil")
		return
	}
	if s.sessions == nil {
		err = fmt.Errorf("session repository not configured")
		return
	}
	if s.credentials == nil {
		err = fmt.Errorf("credential store not configured")
		return
	}

	trimmed := strings.TrimSpace(token)
	logger := s.loggerWith(ctx, "ValidateSession", "token_provided", trimmed != "")
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "session validation failed", "error", err, "error_kind", ErrorKind(err))
			return
		}
		logger.With("principal_id", principal.UserID).InfoContext(ctx, "session validated")
	}()

	if trimmed == "" {
		err = ErrInvalidCredentials
		return
	}

	var session Session
	session, err = s.sessions.GetSession(ctx, trimmed)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			err = ErrUnauthorized
		}
		return
	}

	now := s.now()
	if session.RevokedAt != nil && !session.RevokedAt.IsZero() {
		err = ErrSessionRevoked
		return
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(now) {
		err = ErrSessionExpired
		return
	}

	var user User
	user, err = s.credentials.GetUser(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			err = ErrUnauthorized
		}
		return
	}

	principal = Principal{UserID: user.ID, IsAdmin: user.IsAdmin}
	return
}
