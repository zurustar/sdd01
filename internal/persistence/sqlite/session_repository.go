package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

// SessionRepository implements persistence.SessionRepository using SQLite
type SessionRepository struct {
	pool   *ConnectionPool
	helper *QueryHelper
	mapper *ErrorMapper
}

// NewSessionRepository creates a new SQLite session repository
func NewSessionRepository(pool *ConnectionPool) *SessionRepository {
	return &SessionRepository{
		pool:   pool,
		helper: NewQueryHelper(pool),
		mapper: NewErrorMapper(),
	}
}

// CreateSession stores a new session token for a user
func (r *SessionRepository) CreateSession(ctx context.Context, session persistence.Session) (persistence.Session, error) {
	if session.ID == "" {
		return persistence.Session{}, persistence.ErrConstraintViolation
	}
	if session.UserID == "" {
		return persistence.Session{}, persistence.ErrConstraintViolation
	}
	if strings.TrimSpace(session.Token) == "" {
		return persistence.Session{}, persistence.ErrConstraintViolation
	}
	
	// Normalize session data
	normalized, err := r.normalizeSession(session)
	if err != nil {
		return persistence.Session{}, err
	}
	
	// Set timestamps
	now := time.Now().UTC()
	normalized.CreatedAt = now
	normalized.UpdatedAt = now
	
	query := `
		INSERT INTO sessions (id, user_id, token, fingerprint, expires_at, revoked_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	var revokedAt sql.NullString
	if normalized.RevokedAt != nil {
		revokedAt.String = normalized.RevokedAt.Format(time.RFC3339)
		revokedAt.Valid = true
	}
	
	_, err = r.helper.Exec(ctx, query,
		normalized.ID,
		normalized.UserID,
		normalized.Token,
		normalized.Fingerprint,
		normalized.ExpiresAt.Format(time.RFC3339),
		revokedAt,
		normalized.CreatedAt.Format(time.RFC3339),
		normalized.UpdatedAt.Format(time.RFC3339),
	)
	
	if err != nil {
		return persistence.Session{}, r.mapSessionError(err)
	}
	
	return r.cloneSession(normalized), nil
}

// GetSession retrieves a session by its token value
func (r *SessionRepository) GetSession(ctx context.Context, token string) (persistence.Session, error) {
	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" {
		return persistence.Session{}, persistence.ErrNotFound
	}
	
	query := `
		SELECT id, user_id, token, fingerprint, expires_at, revoked_at, created_at, updated_at
		FROM sessions
		WHERE token = ?
	`
	
	var session persistence.Session
	var expiresAtStr, createdAtStr, updatedAtStr string
	var revokedAt sql.NullString
	
	err := r.helper.QueryRow(ctx, query, normalizedToken).Scan(
		&session.ID,
		&session.UserID,
		&session.Token,
		&session.Fingerprint,
		&expiresAtStr,
		&revokedAt,
		&createdAtStr,
		&updatedAtStr,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return persistence.Session{}, persistence.ErrNotFound
		}
		return persistence.Session{}, r.mapper.MapError(err)
	}
	
	// Handle nullable revoked_at field
	if revokedAt.Valid {
		if session.RevokedAt, err = parseTimePtr(revokedAt.String); err != nil {
			return persistence.Session{}, fmt.Errorf("failed to parse revoked_at: %w", err)
		}
	}
	
	// Parse timestamps
	if session.ExpiresAt, err = time.Parse(time.RFC3339, expiresAtStr); err != nil {
		return persistence.Session{}, fmt.Errorf("failed to parse expires_at: %w", err)
	}
	if session.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
		return persistence.Session{}, fmt.Errorf("failed to parse created_at: %w", err)
	}
	if session.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
		return persistence.Session{}, fmt.Errorf("failed to parse updated_at: %w", err)
	}
	
	return r.cloneSession(session), nil
}

// UpdateSession updates mutable fields of an existing session
func (r *SessionRepository) UpdateSession(ctx context.Context, session persistence.Session) (persistence.Session, error) {
	if session.ID == "" {
		return persistence.Session{}, persistence.ErrConstraintViolation
	}
	
	// Get current session to preserve immutable fields
	current, err := r.getSessionByID(ctx, session.ID)
	if err != nil {
		return persistence.Session{}, err
	}
	
	// Preserve immutable fields
	session.ID = current.ID
	session.UserID = current.UserID
	session.CreatedAt = current.CreatedAt
	
	// Normalize session data
	normalized, err := r.normalizeSession(session)
	if err != nil {
		return persistence.Session{}, err
	}
	
	// Set updated timestamp
	normalized.UpdatedAt = time.Now().UTC()
	
	query := `
		UPDATE sessions 
		SET token = ?, fingerprint = ?, expires_at = ?, revoked_at = ?, updated_at = ?
		WHERE id = ?
	`
	
	var revokedAt sql.NullString
	if normalized.RevokedAt != nil {
		revokedAt.String = normalized.RevokedAt.Format(time.RFC3339)
		revokedAt.Valid = true
	}
	
	result, err := r.helper.Exec(ctx, query,
		normalized.Token,
		normalized.Fingerprint,
		normalized.ExpiresAt.Format(time.RFC3339),
		revokedAt,
		normalized.UpdatedAt.Format(time.RFC3339),
		normalized.ID,
	)
	
	if err != nil {
		return persistence.Session{}, r.mapSessionError(err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return persistence.Session{}, fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return persistence.Session{}, persistence.ErrNotFound
	}
	
	return r.cloneSession(normalized), nil
}

// RevokeSession marks a session as revoked based on its token value
func (r *SessionRepository) RevokeSession(ctx context.Context, token string, revokedAt time.Time) (persistence.Session, error) {
	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" {
		return persistence.Session{}, persistence.ErrNotFound
	}
	
	revokedAtUTC := revokedAt.UTC()
	updatedAt := revokedAtUTC
	
	err := r.pool.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Get the current session
		var session persistence.Session
		var expiresAtStr, createdAtStr, updatedAtStr string
		var currentRevokedAt sql.NullString
		
		err := r.helper.QueryRowTx(tx, `
			SELECT id, user_id, token, fingerprint, expires_at, revoked_at, created_at, updated_at
			FROM sessions
			WHERE token = ?
		`, normalizedToken).Scan(
			&session.ID,
			&session.UserID,
			&session.Token,
			&session.Fingerprint,
			&expiresAtStr,
			&currentRevokedAt,
			&createdAtStr,
			&updatedAtStr,
		)
		
		if err != nil {
			if err == sql.ErrNoRows {
				return persistence.ErrNotFound
			}
			return r.mapper.MapError(err)
		}
		
		// Parse timestamps
		if session.ExpiresAt, err = time.Parse(time.RFC3339, expiresAtStr); err != nil {
			return fmt.Errorf("failed to parse expires_at: %w", err)
		}
		if session.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
			return fmt.Errorf("failed to parse created_at: %w", err)
		}
		if session.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
			return fmt.Errorf("failed to parse updated_at: %w", err)
		}
		
		// Handle existing revoked_at
		if currentRevokedAt.Valid {
			if session.RevokedAt, err = parseTimePtr(currentRevokedAt.String); err != nil {
				return fmt.Errorf("failed to parse existing revoked_at: %w", err)
			}
		}
		
		// Update revocation timestamp
		session.RevokedAt = &revokedAtUTC
		session.UpdatedAt = updatedAt
		
		// Update the session in database
		updateQuery := `
			UPDATE sessions 
			SET revoked_at = ?, updated_at = ?
			WHERE token = ?
		`
		
		result, err := r.helper.ExecTx(tx, updateQuery,
			revokedAtUTC.Format(time.RFC3339),
			updatedAt.Format(time.RFC3339),
			normalizedToken,
		)
		
		if err != nil {
			return r.mapSessionError(err)
		}
		
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}
		
		if rowsAffected == 0 {
			return persistence.ErrNotFound
		}
		
		// Return the updated session (this will be captured by the transaction wrapper)
		// We need to store it in a way that can be returned from the transaction
		return nil
	})
	
	if err != nil {
		return persistence.Session{}, err
	}
	
	// If transaction succeeded, fetch and return the updated session
	return r.GetSession(ctx, normalizedToken)
}

// DeleteExpiredSessions removes sessions that expired on or before the provided timestamp
func (r *SessionRepository) DeleteExpiredSessions(ctx context.Context, reference time.Time) error {
	cutoff := reference.UTC()
	
	query := `
		DELETE FROM sessions 
		WHERE expires_at <= ? AND expires_at != '0001-01-01T00:00:00Z'
	`
	
	_, err := r.helper.Exec(ctx, query, cutoff.Format(time.RFC3339))
	if err != nil {
		return r.mapper.MapError(err)
	}
	
	return nil
}

// getSessionByID retrieves a session by ID (internal helper)
func (r *SessionRepository) getSessionByID(ctx context.Context, id string) (persistence.Session, error) {
	query := `
		SELECT id, user_id, token, fingerprint, expires_at, revoked_at, created_at, updated_at
		FROM sessions
		WHERE id = ?
	`
	
	var session persistence.Session
	var expiresAtStr, createdAtStr, updatedAtStr string
	var revokedAt sql.NullString
	
	err := r.helper.QueryRow(ctx, query, id).Scan(
		&session.ID,
		&session.UserID,
		&session.Token,
		&session.Fingerprint,
		&expiresAtStr,
		&revokedAt,
		&createdAtStr,
		&updatedAtStr,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return persistence.Session{}, persistence.ErrNotFound
		}
		return persistence.Session{}, r.mapper.MapError(err)
	}
	
	// Handle nullable revoked_at field
	if revokedAt.Valid {
		if session.RevokedAt, err = parseTimePtr(revokedAt.String); err != nil {
			return persistence.Session{}, fmt.Errorf("failed to parse revoked_at: %w", err)
		}
	}
	
	// Parse timestamps
	if session.ExpiresAt, err = time.Parse(time.RFC3339, expiresAtStr); err != nil {
		return persistence.Session{}, fmt.Errorf("failed to parse expires_at: %w", err)
	}
	if session.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
		return persistence.Session{}, fmt.Errorf("failed to parse created_at: %w", err)
	}
	if session.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
		return persistence.Session{}, fmt.Errorf("failed to parse updated_at: %w", err)
	}
	
	return session, nil
}

// normalizeSession normalizes session data for consistent storage
func (r *SessionRepository) normalizeSession(session persistence.Session) (persistence.Session, error) {
	if session.ID == "" {
		return persistence.Session{}, persistence.ErrConstraintViolation
	}
	
	session.Token = strings.TrimSpace(session.Token)
	if session.Token == "" {
		return persistence.Session{}, persistence.ErrConstraintViolation
	}
	
	session.Fingerprint = strings.TrimSpace(session.Fingerprint)
	session.CreatedAt = session.CreatedAt.UTC()
	session.UpdatedAt = session.UpdatedAt.UTC()
	session.ExpiresAt = session.ExpiresAt.UTC()
	
	if session.RevokedAt != nil {
		revoked := session.RevokedAt.UTC()
		session.RevokedAt = &revoked
	}
	
	return session, nil
}

// cloneSession creates a deep copy of a session
func (r *SessionRepository) cloneSession(session persistence.Session) persistence.Session {
	clone := session
	if session.RevokedAt != nil {
		revoked := session.RevokedAt.UTC()
		clone.RevokedAt = &revoked
	}
	return clone
}

// mapSessionError maps SQLite errors to appropriate persistence errors for session operations
func (r *SessionRepository) mapSessionError(err error) error {
	if err == nil {
		return nil
	}
	
	errStr := err.Error()
	
	// Handle unique constraint violations
	if containsAny(errStr, []string{"UNIQUE constraint failed"}) {
		if containsAny(errStr, []string{"sessions.token"}) {
			return persistence.ErrDuplicate
		}
		if containsAny(errStr, []string{"sessions.id", "PRIMARY KEY"}) {
			return persistence.ErrDuplicate
		}
		return persistence.ErrDuplicate
	}
	
	// Handle foreign key violations
	if containsAny(errStr, []string{"FOREIGN KEY constraint failed"}) {
		return persistence.ErrForeignKeyViolation
	}
	
	// Handle check constraint violations
	if containsAny(errStr, []string{"CHECK constraint failed"}) {
		return persistence.ErrConstraintViolation
	}
	
	// Use the general error mapper for other cases
	return r.mapper.MapError(err)
}