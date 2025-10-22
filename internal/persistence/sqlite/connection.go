package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence/sqlite/migration"
	_ "modernc.org/sqlite"
)

// ConnectionPool manages SQLite database connections with transaction support
type ConnectionPool struct {
	db     *sql.DB
	config migration.SQLiteConfig
}

// NewConnectionPool creates a new SQLite connection pool
func NewConnectionPool(config migration.SQLiteConfig) (*ConnectionPool, error) {
	connectionManager := migration.NewConnectionManager(config)
	
	db, err := connectionManager.GetConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}
	
	return &ConnectionPool{
		db:     db,
		config: config,
	}, nil
}

// DB returns the underlying database connection
func (cp *ConnectionPool) DB() *sql.DB {
	return cp.db
}

// Close closes the connection pool
func (cp *ConnectionPool) Close() error {
	if cp.db != nil {
		return cp.db.Close()
	}
	return nil
}

// Ping tests the database connection
func (cp *ConnectionPool) Ping(ctx context.Context) error {
	return cp.db.PingContext(ctx)
}

// TransactionFunc represents a function that executes within a transaction
type TransactionFunc func(tx *sql.Tx) error

// WithTransaction executes a function within a database transaction
// If the function returns an error, the transaction is rolled back
// Otherwise, the transaction is committed
func (cp *ConnectionPool) WithTransaction(ctx context.Context, fn TransactionFunc) error {
	tx, err := cp.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	defer func() {
		if p := recover(); p != nil {
			// Rollback on panic
			if rbErr := tx.Rollback(); rbErr != nil {
				// Log rollback error but don't mask the original panic
			}
			panic(p)
		}
	}()
	
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction failed (rollback error: %v): %w", rbErr, err)
		}
		return err
	}
	
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// WithReadOnlyTransaction executes a function within a read-only transaction
func (cp *ConnectionPool) WithReadOnlyTransaction(ctx context.Context, fn TransactionFunc) error {
	tx, err := cp.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to begin read-only transaction: %w", err)
	}
	
	defer func() {
		if p := recover(); p != nil {
			// Rollback on panic
			if rbErr := tx.Rollback(); rbErr != nil {
				// Log rollback error but don't mask the original panic
			}
			panic(p)
		}
	}()
	
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("read-only transaction failed (rollback error: %v): %w", rbErr, err)
		}
		return err
	}
	
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit read-only transaction: %w", err)
	}
	
	return nil
}

// QueryHelper provides helper methods for common query patterns
type QueryHelper struct {
	pool *ConnectionPool
}

// NewQueryHelper creates a new query helper
func NewQueryHelper(pool *ConnectionPool) *QueryHelper {
	return &QueryHelper{pool: pool}
}

// QueryRow executes a query that returns a single row
func (qh *QueryHelper) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return qh.pool.db.QueryRowContext(ctx, query, args...)
}

// Query executes a query that returns multiple rows
func (qh *QueryHelper) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return qh.pool.db.QueryContext(ctx, query, args...)
}

// Exec executes a query that doesn't return rows
func (qh *QueryHelper) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return qh.pool.db.ExecContext(ctx, query, args...)
}

// QueryRowTx executes a query that returns a single row within a transaction
func (qh *QueryHelper) QueryRowTx(tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	return tx.QueryRow(query, args...)
}

// QueryTx executes a query that returns multiple rows within a transaction
func (qh *QueryHelper) QueryTx(tx *sql.Tx, query string, args ...interface{}) (*sql.Rows, error) {
	return tx.Query(query, args...)
}

// ExecTx executes a query that doesn't return rows within a transaction
func (qh *QueryHelper) ExecTx(tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	return tx.Exec(query, args...)
}

// ErrorMapper maps SQLite errors to persistence layer errors
type ErrorMapper struct{}

// NewErrorMapper creates a new error mapper
func NewErrorMapper() *ErrorMapper {
	return &ErrorMapper{}
}

// MapError maps SQLite-specific errors to persistence layer errors
func (em *ErrorMapper) MapError(err error) error {
	if err == nil {
		return nil
	}
	
	// Handle sql.ErrNoRows
	if err == sql.ErrNoRows {
		return fmt.Errorf("record not found: %w", err)
	}
	
	// Check for SQLite-specific error codes
	errStr := err.Error()
	
	// UNIQUE constraint violations
	if containsAny(errStr, []string{"UNIQUE constraint failed", "constraint failed"}) {
		return fmt.Errorf("duplicate record: %w", err)
	}
	
	// FOREIGN KEY constraint violations
	if containsAny(errStr, []string{"FOREIGN KEY constraint failed", "foreign key constraint"}) {
		return fmt.Errorf("foreign key violation: %w", err)
	}
	
	// CHECK constraint violations
	if containsAny(errStr, []string{"CHECK constraint failed"}) {
		return fmt.Errorf("constraint violation: %w", err)
	}
	
	// Database locked errors
	if containsAny(errStr, []string{"database is locked", "database locked"}) {
		return fmt.Errorf("database locked: %w", err)
	}
	
	// Return original error if no mapping found
	return err
}

// containsAny checks if the string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// RetryConfig configures retry behavior for database operations
type RetryConfig struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// DefaultRetryConfig returns a retry configuration with sensible defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	}
}

// RetryHelper provides retry functionality for database operations
type RetryHelper struct {
	config RetryConfig
	mapper *ErrorMapper
}

// NewRetryHelper creates a new retry helper
func NewRetryHelper(config RetryConfig) *RetryHelper {
	return &RetryHelper{
		config: config,
		mapper: NewErrorMapper(),
	}
}

// RetryableFunc represents a function that can be retried
type RetryableFunc func() error

// WithRetry executes a function with retry logic for transient errors
func (rh *RetryHelper) WithRetry(ctx context.Context, fn RetryableFunc) error {
	var lastErr error
	delay := rh.config.InitialDelay
	
	for attempt := 0; attempt <= rh.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Calculate next delay with exponential backoff
				delay = time.Duration(float64(delay) * rh.config.BackoffFactor)
				if delay > rh.config.MaxDelay {
					delay = rh.config.MaxDelay
				}
			}
		}
		
		err := fn()
		if err == nil {
			return nil
		}
		
		lastErr = rh.mapper.MapError(err)
		
		// Don't retry certain types of errors
		if !isRetryableError(lastErr) {
			return lastErr
		}
	}
	
	return fmt.Errorf("operation failed after %d retries: %w", rh.config.MaxRetries, lastErr)
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	
	// Retry database locked errors
	if containsAny(errStr, []string{"database locked", "database is locked"}) {
		return true
	}
	
	// Retry busy errors
	if containsAny(errStr, []string{"database is busy", "busy"}) {
		return true
	}
	
	// Don't retry constraint violations, not found errors, etc.
	if containsAny(errStr, []string{
		"duplicate record",
		"foreign key violation", 
		"constraint violation",
		"record not found",
	}) {
		return false
	}
	
	// Default to not retrying unknown errors
	return false
}