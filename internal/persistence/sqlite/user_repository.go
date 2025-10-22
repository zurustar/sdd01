package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

// UserRepository implements persistence.UserRepository using SQLite
type UserRepository struct {
	pool   *ConnectionPool
	helper *QueryHelper
	mapper *ErrorMapper
}

// NewUserRepository creates a new SQLite user repository
func NewUserRepository(pool *ConnectionPool) *UserRepository {
	return &UserRepository{
		pool:   pool,
		helper: NewQueryHelper(pool),
		mapper: NewErrorMapper(),
	}
}

// CreateUser inserts a new user into the database
func (r *UserRepository) CreateUser(ctx context.Context, user persistence.User) error {
	if user.ID == "" {
		return persistence.ErrConstraintViolation
	}
	if user.PasswordHash == "" {
		return persistence.ErrConstraintViolation
	}
	
	// Normalize email for uniqueness check
	normalizedEmail := normalizeEmail(user.Email)
	
	// Set timestamps
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now
	
	query := `
		INSERT INTO users (id, email, display_name, password_hash, is_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	
	_, err := r.helper.Exec(ctx, query,
		user.ID,
		normalizedEmail,
		user.DisplayName,
		user.PasswordHash,
		user.IsAdmin,
		user.CreatedAt.Format(time.RFC3339),
		user.UpdatedAt.Format(time.RFC3339),
	)
	
	if err != nil {
		return r.mapUserError(err)
	}
	
	return nil
}

// UpdateUser updates an existing user in the database
func (r *UserRepository) UpdateUser(ctx context.Context, user persistence.User) error {
	if user.ID == "" {
		return persistence.ErrConstraintViolation
	}
	if user.PasswordHash == "" {
		return persistence.ErrConstraintViolation
	}
	
	// Normalize email for uniqueness check
	normalizedEmail := normalizeEmail(user.Email)
	
	// Set updated timestamp
	user.UpdatedAt = time.Now().UTC()
	
	query := `
		UPDATE users 
		SET email = ?, display_name = ?, password_hash = ?, is_admin = ?, updated_at = ?
		WHERE id = ?
	`
	
	result, err := r.helper.Exec(ctx, query,
		normalizedEmail,
		user.DisplayName,
		user.PasswordHash,
		user.IsAdmin,
		user.UpdatedAt.Format(time.RFC3339),
		user.ID,
	)
	
	if err != nil {
		return r.mapUserError(err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return persistence.ErrNotFound
	}
	
	return nil
}

// GetUser retrieves a user by ID from the database
func (r *UserRepository) GetUser(ctx context.Context, id string) (persistence.User, error) {
	if id == "" {
		return persistence.User{}, persistence.ErrNotFound
	}
	
	query := `
		SELECT id, email, display_name, password_hash, is_admin, created_at, updated_at
		FROM users
		WHERE id = ?
	`
	
	var user persistence.User
	var createdAtStr, updatedAtStr string
	
	err := r.helper.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.PasswordHash,
		&user.IsAdmin,
		&createdAtStr,
		&updatedAtStr,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return persistence.User{}, persistence.ErrNotFound
		}
		return persistence.User{}, r.mapper.MapError(err)
	}
	
	// Parse timestamps
	if user.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
		return persistence.User{}, fmt.Errorf("failed to parse created_at: %w", err)
	}
	if user.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
		return persistence.User{}, fmt.Errorf("failed to parse updated_at: %w", err)
	}
	
	return user, nil
}

// GetUserByEmail retrieves a user by email address from the database
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (persistence.User, error) {
	if email == "" {
		return persistence.User{}, persistence.ErrNotFound
	}
	
	normalizedEmail := normalizeEmail(email)
	
	query := `
		SELECT id, email, display_name, password_hash, is_admin, created_at, updated_at
		FROM users
		WHERE email = ?
	`
	
	var user persistence.User
	var createdAtStr, updatedAtStr string
	
	err := r.helper.QueryRow(ctx, query, normalizedEmail).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.PasswordHash,
		&user.IsAdmin,
		&createdAtStr,
		&updatedAtStr,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return persistence.User{}, persistence.ErrNotFound
		}
		return persistence.User{}, r.mapper.MapError(err)
	}
	
	// Parse timestamps
	if user.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
		return persistence.User{}, fmt.Errorf("failed to parse created_at: %w", err)
	}
	if user.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
		return persistence.User{}, fmt.Errorf("failed to parse updated_at: %w", err)
	}
	
	return user, nil
}

// ListUsers returns all users ordered by creation timestamp then ID
func (r *UserRepository) ListUsers(ctx context.Context) ([]persistence.User, error) {
	query := `
		SELECT id, email, display_name, password_hash, is_admin, created_at, updated_at
		FROM users
		ORDER BY created_at ASC, id ASC
	`
	
	rows, err := r.helper.Query(ctx, query)
	if err != nil {
		return nil, r.mapper.MapError(err)
	}
	defer rows.Close()
	
	var users []persistence.User
	
	for rows.Next() {
		var user persistence.User
		var createdAtStr, updatedAtStr string
		
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.DisplayName,
			&user.PasswordHash,
			&user.IsAdmin,
			&createdAtStr,
			&updatedAtStr,
		)
		
		if err != nil {
			return nil, r.mapper.MapError(err)
		}
		
		// Parse timestamps
		if user.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		if user.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse updated_at: %w", err)
		}
		
		users = append(users, user)
	}
	
	if err := rows.Err(); err != nil {
		return nil, r.mapper.MapError(err)
	}
	
	return users, nil
}

// DeleteUser removes a user by ID from the database
func (r *UserRepository) DeleteUser(ctx context.Context, id string) error {
	if id == "" {
		return persistence.ErrNotFound
	}
	
	return r.pool.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Check if user exists and if they have created any schedules
		var scheduleCount int
		err := r.helper.QueryRowTx(tx, "SELECT COUNT(*) FROM schedules WHERE creator_id = ?", id).Scan(&scheduleCount)
		if err != nil {
			return r.mapper.MapError(err)
		}
		
		if scheduleCount > 0 {
			return persistence.ErrForeignKeyViolation
		}
		
		// Remove user from schedule participants
		_, err = r.helper.ExecTx(tx, "DELETE FROM schedule_participants WHERE user_id = ?", id)
		if err != nil {
			return r.mapper.MapError(err)
		}
		
		// Delete the user
		result, err := r.helper.ExecTx(tx, "DELETE FROM users WHERE id = ?", id)
		if err != nil {
			return r.mapper.MapError(err)
		}
		
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}
		
		if rowsAffected == 0 {
			return persistence.ErrNotFound
		}
		
		return nil
	})
}

// mapUserError maps SQLite errors to appropriate persistence errors for user operations
func (r *UserRepository) mapUserError(err error) error {
	if err == nil {
		return nil
	}
	
	errStr := err.Error()
	
	// Handle unique constraint violations
	if containsAny(errStr, []string{"UNIQUE constraint failed"}) {
		if containsAny(errStr, []string{"users.email"}) {
			return persistence.ErrDuplicate
		}
		if containsAny(errStr, []string{"users.id", "PRIMARY KEY"}) {
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

// normalizeEmail normalizes email addresses for consistent storage and lookup
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}