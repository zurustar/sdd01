package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAuthService_Authenticate(t *testing.T) {
	t.Parallel()

	t.Run("verifies password hashing edge cases", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: cover authenticate with legacy hashes and malformed encodings")
	})

	t.Run("enforces account lockout when configured", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure repeated failures trigger lockout behavior")
	})

	t.Run("issues sessions for valid credentials", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		creds := &credentialStoreStub{
			credentials: UserCredentials{
				User:         User{ID: "user-1", Email: "user@example.com"},
				PasswordHash: "secret",
			},
		}

		repo := newSessionRepositoryStub()
		tokenSeq := []string{"session-id", "session-token"}
		svc := NewAuthService(creds, repo, nil, func() string {
			if len(tokenSeq) == 0 {
				return "fallback"
			}
			token := tokenSeq[0]
			tokenSeq = tokenSeq[1:]
			return token
		}, func() time.Time { return now }, time.Hour)

		result, err := svc.Authenticate(context.Background(), AuthenticateParams{Email: "User@example.com", Password: "secret", Fingerprint: " device "})
		if err != nil {
			t.Fatalf("Authenticate failed: %v", err)
		}

		if result.Session.Token != "session-token" {
			t.Fatalf("expected issued token, got %s", result.Session.Token)
		}
		if result.Session.Fingerprint != "device" {
			t.Fatalf("expected fingerprint to be trimmed, got %q", result.Session.Fingerprint)
		}
		if len(repo.deleteCalls) != 1 || !repo.deleteCalls[0].Equal(now) {
			t.Fatalf("expected DeleteExpiredSessions to be called with now, got %#v", repo.deleteCalls)
		}
	})

	t.Run("rejects disabled accounts", func(t *testing.T) {
		t.Parallel()

		creds := &credentialStoreStub{credentials: UserCredentials{User: User{ID: "user"}, Disabled: true}}
		svc := NewAuthService(creds, nil, nil, nil, time.Now, time.Hour)

		_, err := svc.Authenticate(context.Background(), AuthenticateParams{Email: "user@example.com", Password: "secret"})
		if !errors.Is(err, ErrAccountDisabled) {
			t.Fatalf("expected ErrAccountDisabled, got %v", err)
		}
	})

	t.Run("rejects invalid credentials with sentinel error", func(t *testing.T) {
		t.Parallel()

		creds := &credentialStoreStub{
			credentials: UserCredentials{User: User{ID: "user"}, PasswordHash: "expected"},
		}
		svc := NewAuthService(creds, nil, nil, nil, time.Now, time.Hour)

		_, err := svc.Authenticate(context.Background(), AuthenticateParams{Email: "user@example.com", Password: "wrong"})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("propagates repository failures", func(t *testing.T) {
		t.Parallel()

		expected := errors.New("boom")
		creds := &credentialStoreStub{
			credentials: UserCredentials{User: User{ID: "user"}, PasswordHash: "secret"},
		}
		repo := newSessionRepositoryStub()
		repo.createErr = expected

		svc := NewAuthService(creds, repo, nil, func() string { return "token" }, time.Now, time.Hour)

		_, err := svc.Authenticate(context.Background(), AuthenticateParams{Email: "user@example.com", Password: "secret"})
		if !errors.Is(err, expected) {
			t.Fatalf("expected error %v, got %v", expected, err)
		}
	})

	t.Run("propagates cleanup failures", func(t *testing.T) {
		t.Parallel()

		expected := errors.New("cleanup-failed")
		creds := &credentialStoreStub{
			credentials: UserCredentials{User: User{ID: "user"}, PasswordHash: "secret"},
		}
		repo := newSessionRepositoryStub()
		repo.deleteErr = expected

		svc := NewAuthService(creds, repo, nil, func() string { return "token" }, time.Now, time.Hour)

		_, err := svc.Authenticate(context.Background(), AuthenticateParams{Email: "user@example.com", Password: "secret"})
		if !errors.Is(err, expected) {
			t.Fatalf("expected cleanup error %v, got %v", expected, err)
		}
	})

	t.Run("records audit events for successful sign-ins", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert Authenticate triggers audit logging hook")
	})
}

func TestAuthService_RefreshSession(t *testing.T) {
	t.Parallel()

	t.Run("issues new tokens for valid sessions", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		repo := newSessionRepositoryStub()
		repo.seed(Session{ID: "session-1", UserID: "user", Token: "existing", ExpiresAt: now.Add(time.Minute), UpdatedAt: now, CreatedAt: now})

		tokens := []string{"new-token"}
		svc := NewAuthService(nil, repo, nil, func() string {
			token := tokens[0]
			tokens = tokens[1:]
			return token
		}, func() time.Time { return now }, 2*time.Hour)

		result, err := svc.RefreshSession(context.Background(), RefreshSessionParams{Token: "existing", Fingerprint: "updated"})
		if err != nil {
			t.Fatalf("RefreshSession failed: %v", err)
		}

		if result.Session.Token != "new-token" {
			t.Fatalf("expected rotated token, got %s", result.Session.Token)
		}
		if result.Session.Fingerprint != "updated" {
			t.Fatalf("expected fingerprint update, got %q", result.Session.Fingerprint)
		}
		stored := repo.sessionsByID["session-1"]
		if stored.Token != "new-token" || stored.Fingerprint != "updated" {
			t.Fatalf("expected persisted update, got %#v", stored)
		}
	})

	t.Run("rejects expired sessions", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		repo := newSessionRepositoryStub()
		repo.seed(Session{ID: "session-1", UserID: "user", Token: "expired", ExpiresAt: now.Add(-time.Minute), UpdatedAt: now, CreatedAt: now})

		svc := NewAuthService(nil, repo, nil, nil, func() time.Time { return now }, time.Hour)

		_, err := svc.RefreshSession(context.Background(), RefreshSessionParams{Token: "expired"})
		if !errors.Is(err, ErrSessionExpired) {
			t.Fatalf("expected ErrSessionExpired, got %v", err)
		}
	})

	t.Run("rejects revoked sessions", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		revokedAt := now.Add(-time.Minute)
		repo := newSessionRepositoryStub()
		repo.seed(Session{ID: "session-1", UserID: "user", Token: "revoked", ExpiresAt: now.Add(time.Minute), RevokedAt: &revokedAt, UpdatedAt: now, CreatedAt: now})

		svc := NewAuthService(nil, repo, nil, nil, func() time.Time { return now }, time.Hour)

		_, err := svc.RefreshSession(context.Background(), RefreshSessionParams{Token: "revoked"})
		if !errors.Is(err, ErrSessionRevoked) {
			t.Fatalf("expected ErrSessionRevoked, got %v", err)
		}
	})

	t.Run("persists rotated session metadata", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		repo := newSessionRepositoryStub()
		repo.seed(Session{ID: "session-1", UserID: "user", Token: "existing", ExpiresAt: now.Add(time.Minute), Fingerprint: "old", UpdatedAt: now, CreatedAt: now})

		svc := NewAuthService(nil, repo, nil, func() string { return "new-token" }, func() time.Time { return now }, time.Hour)

		_, err := svc.RefreshSession(context.Background(), RefreshSessionParams{Token: "existing", Fingerprint: "new"})
		if err != nil {
			t.Fatalf("RefreshSession failed: %v", err)
		}

		stored := repo.sessionsByID["session-1"]
		if stored.Token != "new-token" {
			t.Fatalf("expected token rotation, got %#v", stored)
		}
		if stored.ExpiresAt.Before(now.Add(time.Hour)) {
			t.Fatalf("expected expiry to be extended, got %v", stored.ExpiresAt)
		}
	})
}

func TestAuthService_RevokeSession(t *testing.T) {
	t.Parallel()

	t.Run("revokes active sessions", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		repo := newSessionRepositoryStub()
		repo.seed(Session{ID: "session-1", UserID: "user", Token: "token", ExpiresAt: now.Add(time.Hour), UpdatedAt: now, CreatedAt: now})

		svc := NewAuthService(nil, repo, nil, nil, func() time.Time { return now }, time.Hour)

		if err := svc.RevokeSession(context.Background(), "token"); err != nil {
			t.Fatalf("RevokeSession failed: %v", err)
		}

		stored := repo.sessionsByID["session-1"]
		if stored.RevokedAt == nil || stored.RevokedAt.IsZero() {
			t.Fatalf("expected RevokedAt to be set, got %#v", stored.RevokedAt)
		}
		if len(repo.deleteCalls) == 0 {
			t.Fatalf("expected DeleteExpiredSessions to be invoked")
		}
	})

	t.Run("requires non-empty token", func(t *testing.T) {
		t.Parallel()

		repo := newSessionRepositoryStub()
		svc := NewAuthService(nil, repo, nil, nil, time.Now, time.Hour)

		if err := svc.RevokeSession(context.Background(), "  "); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("maps missing tokens to invalid credentials", func(t *testing.T) {
		t.Parallel()

		repo := newSessionRepositoryStub()
		repo.revokeErr = ErrNotFound
		svc := NewAuthService(nil, repo, nil, nil, time.Now, time.Hour)

		if err := svc.RevokeSession(context.Background(), "missing"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("propagates repository errors", func(t *testing.T) {
		t.Parallel()

		expected := errors.New("boom")
		repo := newSessionRepositoryStub()
		repo.revokeErr = expected
		svc := NewAuthService(nil, repo, nil, nil, time.Now, time.Hour)

		if err := svc.RevokeSession(context.Background(), "token"); !errors.Is(err, expected) {
			t.Fatalf("expected %v, got %v", expected, err)
		}
	})
}

func TestAuthService_ValidateSession(t *testing.T) {
	t.Parallel()

	t.Run("returns principal for active session", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		creds := &credentialStoreStub{credentials: UserCredentials{User: User{ID: "user-1", IsAdmin: true}}}
		repo := newSessionRepositoryStub()
		repo.seed(Session{ID: "session-1", UserID: "user-1", Token: "token", ExpiresAt: now.Add(time.Hour), UpdatedAt: now, CreatedAt: now})
		svc := NewAuthService(creds, repo, nil, nil, func() time.Time { return now }, time.Hour)

		principal, err := svc.ValidateSession(context.Background(), " token ")
		if err != nil {
			t.Fatalf("ValidateSession failed: %v", err)
		}

		if principal.UserID != "user-1" || !principal.IsAdmin {
			t.Fatalf("unexpected principal: %#v", principal)
		}
	})

	t.Run("rejects expired sessions", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		creds := &credentialStoreStub{credentials: UserCredentials{User: User{ID: "user-1"}}}
		repo := newSessionRepositoryStub()
		repo.seed(Session{ID: "session-1", UserID: "user-1", Token: "token", ExpiresAt: now.Add(-time.Minute), UpdatedAt: now, CreatedAt: now})
		svc := NewAuthService(creds, repo, nil, nil, func() time.Time { return now }, time.Hour)

		_, err := svc.ValidateSession(context.Background(), "token")
		if !errors.Is(err, ErrSessionExpired) {
			t.Fatalf("expected ErrSessionExpired, got %v", err)
		}
	})

	t.Run("rejects revoked sessions", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		revoked := now.Add(-time.Minute)
		creds := &credentialStoreStub{credentials: UserCredentials{User: User{ID: "user-1"}}}
		repo := newSessionRepositoryStub()
		repo.seed(Session{ID: "session-1", UserID: "user-1", Token: "token", ExpiresAt: now.Add(time.Hour), RevokedAt: &revoked, UpdatedAt: now, CreatedAt: now})
		svc := NewAuthService(creds, repo, nil, nil, func() time.Time { return now }, time.Hour)

		_, err := svc.ValidateSession(context.Background(), "token")
		if !errors.Is(err, ErrSessionRevoked) {
			t.Fatalf("expected ErrSessionRevoked, got %v", err)
		}
	})

	t.Run("rejects empty tokens", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		creds := &credentialStoreStub{credentials: UserCredentials{User: User{ID: "user-1"}}}
		repo := newSessionRepositoryStub()
		repo.seed(Session{ID: "session-1", UserID: "user-1", Token: "token", ExpiresAt: now.Add(time.Hour), UpdatedAt: now, CreatedAt: now})
		svc := NewAuthService(creds, repo, nil, nil, func() time.Time { return now }, time.Hour)

		_, err := svc.ValidateSession(context.Background(), "  ")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("returns unauthorized when user record is missing", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		creds := &credentialStoreStub{credentials: UserCredentials{User: User{ID: "other"}}}
		repo := newSessionRepositoryStub()
		repo.seed(Session{ID: "session-1", UserID: "user-1", Token: "token", ExpiresAt: now.Add(time.Hour), UpdatedAt: now, CreatedAt: now})
		svc := NewAuthService(creds, repo, nil, nil, func() time.Time { return now }, time.Hour)

		_, err := svc.ValidateSession(context.Background(), "token")
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("propagates repository failures", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		expected := errors.New("boom")
		creds := &credentialStoreStub{credentials: UserCredentials{User: User{ID: "user-1"}}}
		repo := newSessionRepositoryStub()
		repo.getErr = expected
		svc := NewAuthService(creds, repo, nil, nil, func() time.Time { return now }, time.Hour)

		_, err := svc.ValidateSession(context.Background(), "token")
		if !errors.Is(err, expected) {
			t.Fatalf("expected %v, got %v", expected, err)
		}
	})
}

// credentialStoreStub implements CredentialStore for tests.
type credentialStoreStub struct {
	credentials UserCredentials
	err         error
}

func (c *credentialStoreStub) GetUserCredentialsByEmail(ctx context.Context, email string) (UserCredentials, error) {
	if c.err != nil {
		return UserCredentials{}, c.err
	}
	if c.credentials.User.ID == "" {
		return UserCredentials{}, ErrNotFound
	}
	return c.credentials, nil
}

func (c *credentialStoreStub) GetUser(ctx context.Context, id string) (User, error) {
	if c.err != nil {
		return User{}, c.err
	}
	if c.credentials.User.ID == id {
		return c.credentials.User, nil
	}
	return User{}, ErrNotFound
}

// sessionRepositoryStub provides an in-memory implementation of SessionRepository for tests.
type sessionRepositoryStub struct {
	sessionsByID map[string]Session
	tokenToID    map[string]string

	createErr error
	getErr    error
	updateErr error
	revokeErr error
	deleteErr error

	deleteCalls []time.Time
}

func newSessionRepositoryStub() *sessionRepositoryStub {
	return &sessionRepositoryStub{
		sessionsByID: make(map[string]Session),
		tokenToID:    make(map[string]string),
	}
}

func (s *sessionRepositoryStub) seed(session Session) {
	s.sessionsByID[session.ID] = cloneSession(session)
	s.tokenToID[session.Token] = session.ID
}

func (s *sessionRepositoryStub) CreateSession(ctx context.Context, session Session) (Session, error) {
	if s.createErr != nil {
		return Session{}, s.createErr
	}
	s.seed(session)
	return cloneSession(session), nil
}

func (s *sessionRepositoryStub) GetSession(ctx context.Context, token string) (Session, error) {
	if s.getErr != nil {
		return Session{}, s.getErr
	}
	id, ok := s.tokenToID[token]
	if !ok {
		return Session{}, ErrNotFound
	}
	return cloneSession(s.sessionsByID[id]), nil
}

func (s *sessionRepositoryStub) UpdateSession(ctx context.Context, session Session) (Session, error) {
	if s.updateErr != nil {
		return Session{}, s.updateErr
	}
	current, ok := s.sessionsByID[session.ID]
	if !ok {
		return Session{}, ErrNotFound
	}
	if current.Token != session.Token {
		delete(s.tokenToID, current.Token)
	}
	s.sessionsByID[session.ID] = cloneSession(session)
	s.tokenToID[session.Token] = session.ID
	return cloneSession(session), nil
}

func (s *sessionRepositoryStub) RevokeSession(ctx context.Context, token string, revokedAt time.Time) (Session, error) {
	if s.revokeErr != nil {
		return Session{}, s.revokeErr
	}
	id, ok := s.tokenToID[token]
	if !ok {
		return Session{}, ErrNotFound
	}
	session := s.sessionsByID[id]
	revoked := revokedAt.UTC()
	session.RevokedAt = &revoked
	session.UpdatedAt = revoked
	s.sessionsByID[id] = session
	return cloneSession(session), nil
}

func (s *sessionRepositoryStub) DeleteExpiredSessions(ctx context.Context, reference time.Time) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	cutoff := reference.UTC()
	s.deleteCalls = append(s.deleteCalls, cutoff)
	for id, session := range s.sessionsByID {
		if session.ExpiresAt.IsZero() {
			continue
		}
		if !session.ExpiresAt.After(cutoff) {
			delete(s.sessionsByID, id)
			delete(s.tokenToID, session.Token)
		}
	}
	return nil
}

func cloneSession(session Session) Session {
	clone := session
	if session.RevokedAt != nil {
		revoked := session.RevokedAt.UTC()
		clone.RevokedAt = &revoked
	}
	return clone
}
