package application

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CredentialStore exposes user credential lookup operations required by the auth service.
type CredentialStore interface {
	GetUserCredentialsByEmail(ctx context.Context, email string) (UserCredentials, error)
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
}

// NewAuthService constructs an AuthService with the provided dependencies.
func NewAuthService(credentials CredentialStore, sessions SessionRepository, verify PasswordVerifier, tokenGenerator func() string, now func() time.Time, sessionTTL time.Duration) *AuthService {
	if verify == nil {
		verify = func(hashedPassword, password string) error {
			if hashedPassword == "" || password == "" {
				return ErrInvalidCredentials
			}
			if subtle.ConstantTimeCompare([]byte(hashedPassword), []byte(password)) != 1 {
				return ErrInvalidCredentials
			}
			return nil
		}
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
	}
}

// Authenticate validates credentials and issues a new session token.
func (s *AuthService) Authenticate(ctx context.Context, params AuthenticateParams) (AuthenticateResult, error) {
	if s == nil {
		return AuthenticateResult{}, fmt.Errorf("AuthService is nil")
	}
	if s.credentials == nil {
		return AuthenticateResult{}, fmt.Errorf("credential store not configured")
	}

	email := strings.TrimSpace(strings.ToLower(params.Email))
	password := params.Password

	if email == "" || password == "" {
		return AuthenticateResult{}, ErrInvalidCredentials
	}

	creds, err := s.credentials.GetUserCredentialsByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AuthenticateResult{}, ErrInvalidCredentials
		}
		return AuthenticateResult{}, err
	}

	if creds.Disabled {
		return AuthenticateResult{}, ErrAccountDisabled
	}

	if err := s.verifyPassword(creds.PasswordHash, password); err != nil {
		return AuthenticateResult{}, ErrInvalidCredentials
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
		if err := s.sessions.DeleteExpiredSessions(ctx, now); err != nil {
			return AuthenticateResult{}, err
		}

		persisted, err := s.sessions.CreateSession(ctx, session)
		if err != nil {
			return AuthenticateResult{}, err
		}
		session = persisted
	}

	return AuthenticateResult{User: creds.User, Session: session}, nil
}

// RefreshSession rotates an existing session token, extending its validity window.
func (s *AuthService) RefreshSession(ctx context.Context, params RefreshSessionParams) (RefreshSessionResult, error) {
	if s == nil {
		return RefreshSessionResult{}, fmt.Errorf("AuthService is nil")
	}
	if s.sessions == nil {
		return RefreshSessionResult{}, fmt.Errorf("session repository not configured")
	}

	token := strings.TrimSpace(params.Token)
	if token == "" {
		return RefreshSessionResult{}, ErrInvalidCredentials
	}

	session, err := s.sessions.GetSession(ctx, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return RefreshSessionResult{}, ErrInvalidCredentials
		}
		return RefreshSessionResult{}, err
	}

	now := s.now()
	if session.RevokedAt != nil && !session.RevokedAt.IsZero() {
		return RefreshSessionResult{}, ErrSessionRevoked
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(now) {
		return RefreshSessionResult{}, ErrSessionExpired
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

	persisted, err := s.sessions.UpdateSession(ctx, session)
	if err != nil {
		return RefreshSessionResult{}, err
	}

	return RefreshSessionResult{Session: persisted}, nil
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

	if _, err := s.sessions.RevokeSession(ctx, trimmed, s.now()); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrInvalidCredentials
		}
		return err
	}

	if err := s.sessions.DeleteExpiredSessions(ctx, s.now()); err != nil {
		return err
	}
	return nil
}
