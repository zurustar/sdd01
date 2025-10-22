package migration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SQLiteExecutor implements the Executor interface for SQLite databases
type SQLiteExecutor struct {
	db *sql.DB
}

// NewSQLiteExecutor creates a new SQLite migration executor
func NewSQLiteExecutor(db *sql.DB) *SQLiteExecutor {
	return &SQLiteExecutor{
		db: db,
	}
}

// ExecuteMigration runs a single migration within a transaction
func (e *SQLiteExecutor) ExecuteMigration(ctx context.Context, migration Migration) error {
	// Start a transaction for the migration
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return NewDatabaseError(migration.Version, "", "begin transaction", err)
	}
	
	// Ensure transaction is rolled back on error
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				// Log rollback error but don't override the original error
				fmt.Printf("Failed to rollback transaction for migration %s: %v\n", migration.Version, rollbackErr)
			}
		}
	}()
	
	// Parse and execute SQL statements
	statements := e.parseSQL(migration.SQL)
	if len(statements) == 0 {
		err = NewMigrationError(migration.Version, migration.FilePath, "parse SQL", 
			fmt.Errorf("no SQL statements found in migration"))
		return err
	}
	
	// Execute each SQL statement
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		
		_, execErr := tx.ExecContext(ctx, stmt)
		if execErr != nil {
			err = NewDatabaseError(migration.Version, stmt, fmt.Sprintf("execute statement %d", i+1), execErr)
			return err
		}
	}
	
	// Commit the transaction
	if err = tx.Commit(); err != nil {
		err = NewDatabaseError(migration.Version, "", "commit transaction", err)
		return err
	}
	
	return nil
}

// InitializeVersionTable creates the schema_migrations table if it doesn't exist
func (e *SQLiteExecutor) InitializeVersionTable(ctx context.Context) error {
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			checksum TEXT,
			execution_time_ms INTEGER
		);
	`
	
	_, err := e.db.ExecContext(ctx, createTableSQL)
	if err != nil {
		return NewDatabaseError("", createTableSQL, "create schema_migrations table", err)
	}
	
	return nil
}

// RecordMigration records a successful migration in the version tracking table
func (e *SQLiteExecutor) RecordMigration(ctx context.Context, version string, executionTime time.Duration) error {
	insertSQL := `
		INSERT INTO schema_migrations (version, applied_at, execution_time_ms)
		VALUES (?, ?, ?)
	`
	
	executionTimeMs := executionTime.Milliseconds()
	appliedAt := time.Now().UTC().Format(time.RFC3339)
	
	_, err := e.db.ExecContext(ctx, insertSQL, version, appliedAt, executionTimeMs)
	if err != nil {
		return NewDatabaseError(version, insertSQL, "record migration", err)
	}
	
	return nil
}

// IsVersionApplied checks if a specific migration version has been applied
func (e *SQLiteExecutor) IsVersionApplied(ctx context.Context, version string) (bool, error) {
	querySQL := `SELECT 1 FROM schema_migrations WHERE version = ? LIMIT 1`
	
	var exists int
	err := e.db.QueryRowContext(ctx, querySQL, version).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows || strings.Contains(err.Error(), "no rows") {
			return false, nil
		}
		return false, NewDatabaseError(version, querySQL, "check version applied", err)
	}
	
	return true, nil
}

// GetAppliedVersions returns all applied migration versions with timestamps
func (e *SQLiteExecutor) GetAppliedVersions(ctx context.Context) ([]AppliedMigration, error) {
	querySQL := `
		SELECT version, applied_at, COALESCE(execution_time_ms, 0), COALESCE(checksum, '')
		FROM schema_migrations
		ORDER BY version ASC
	`
	
	rows, err := e.db.QueryContext(ctx, querySQL)
	if err != nil {
		return nil, NewDatabaseError("", querySQL, "get applied versions", err)
	}
	defer rows.Close()
	
	var appliedMigrations []AppliedMigration
	
	for rows.Next() {
		var version, appliedAtStr, checksum string
		var executionTimeMs int64
		
		if err := rows.Scan(&version, &appliedAtStr, &executionTimeMs, &checksum); err != nil {
			// Handle stub driver "no rows" error
			if strings.Contains(err.Error(), "no rows") {
				break
			}
			return nil, NewDatabaseError("", querySQL, "scan applied migration", err)
		}
		
		appliedAt, parseErr := time.Parse(time.RFC3339, appliedAtStr)
		if parseErr != nil {
			// Fallback to current time if parsing fails
			appliedAt = time.Now().UTC()
		}
		
		appliedMigrations = append(appliedMigrations, AppliedMigration{
			Version:       version,
			AppliedAt:     appliedAt,
			ExecutionTime: time.Duration(executionTimeMs) * time.Millisecond,
			Checksum:      checksum,
		})
	}
	
	if err := rows.Err(); err != nil {
		// Handle stub driver "no rows" error
		if strings.Contains(err.Error(), "no rows") {
			return appliedMigrations, nil
		}
		return nil, NewDatabaseError("", querySQL, "iterate applied migrations", err)
	}
	
	return appliedMigrations, nil
}

// parseSQL splits SQL content into individual statements
// This handles multiple SQL statements separated by semicolons and filters out comments
func (e *SQLiteExecutor) parseSQL(sql string) []string {
	// Split by semicolon and filter out empty statements and comments
	statements := strings.Split(sql, ";")
	var validStatements []string
	
	for _, stmt := range statements {
		// Remove leading/trailing whitespace
		stmt = strings.TrimSpace(stmt)
		
		// Skip empty statements and comment-only lines
		if stmt == "" {
			continue
		}
		
		// Filter out lines that are only comments
		lines := strings.Split(stmt, "\n")
		var nonCommentLines []string
		
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "--") {
				nonCommentLines = append(nonCommentLines, line)
			}
		}
		
		if len(nonCommentLines) > 0 {
			cleanStmt := strings.Join(nonCommentLines, "\n")
			cleanStmt = strings.TrimSpace(cleanStmt)
			if cleanStmt != "" {
				validStatements = append(validStatements, cleanStmt)
			}
		}
	}
	
	return validStatements
}