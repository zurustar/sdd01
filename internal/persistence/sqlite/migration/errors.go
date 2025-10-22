package migration

import (
	"errors"
	"fmt"
)

// Migration-specific error types for different failure scenarios
var (
	// ErrMigrationFailed indicates that a migration execution failed
	ErrMigrationFailed = errors.New("migration execution failed")
	
	// ErrInvalidMigrationFile indicates that a migration file is malformed or invalid
	ErrInvalidMigrationFile = errors.New("invalid migration file format")
	
	// ErrMigrationNotFound indicates that a required migration file was not found
	ErrMigrationNotFound = errors.New("migration file not found")
	
	// ErrVersionConflict indicates that there's a conflict with migration versions
	ErrVersionConflict = errors.New("migration version conflict")
	
	// ErrDatabaseLocked indicates that the database is locked and cannot be migrated
	ErrDatabaseLocked = errors.New("database is locked")
	
	// ErrInvalidVersion indicates that a migration version is invalid or malformed
	ErrInvalidVersion = errors.New("invalid migration version")
	
	// ErrDuplicateVersion indicates that multiple migrations have the same version
	ErrDuplicateVersion = errors.New("duplicate migration version")
	
	// ErrMigrationTimeout indicates that a migration exceeded the allowed execution time
	ErrMigrationTimeout = errors.New("migration execution timeout")
	
	// ErrVersionTableCorrupt indicates that the schema_migrations table is corrupted
	ErrVersionTableCorrupt = errors.New("schema_migrations table is corrupted")
)

// MigrationError wraps migration-specific errors with additional context
type MigrationError struct {
	Version   string // Migration version that caused the error
	FilePath  string // Path to the migration file
	Operation string // Operation being performed (scan, execute, etc.)
	Err       error  // Underlying error
}

// Error implements the error interface
func (e *MigrationError) Error() string {
	if e.Version != "" {
		return fmt.Sprintf("migration %s (%s): %s: %v", e.Version, e.FilePath, e.Operation, e.Err)
	}
	return fmt.Sprintf("migration error (%s): %s: %v", e.FilePath, e.Operation, e.Err)
}

// Unwrap returns the underlying error for error unwrapping
func (e *MigrationError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches a target error
func (e *MigrationError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewMigrationError creates a new MigrationError with context
func NewMigrationError(version, filePath, operation string, err error) *MigrationError {
	return &MigrationError{
		Version:   version,
		FilePath:  filePath,
		Operation: operation,
		Err:       err,
	}
}

// FileSystemError wraps file system related errors during migration operations
type FileSystemError struct {
	Path      string // File or directory path
	Operation string // File operation (read, scan, etc.)
	Err       error  // Underlying error
}

// Error implements the error interface
func (e *FileSystemError) Error() string {
	return fmt.Sprintf("filesystem error during %s of %s: %v", e.Operation, e.Path, e.Err)
}

// Unwrap returns the underlying error
func (e *FileSystemError) Unwrap() error {
	return e.Err
}

// NewFileSystemError creates a new FileSystemError
func NewFileSystemError(path, operation string, err error) *FileSystemError {
	return &FileSystemError{
		Path:      path,
		Operation: operation,
		Err:       err,
	}
}

// DatabaseError wraps database-related errors during migration operations
type DatabaseError struct {
	Version   string // Migration version (if applicable)
	Query     string // SQL query that failed (if applicable)
	Operation string // Database operation (execute, query, etc.)
	Err       error  // Underlying error
}

// Error implements the error interface
func (e *DatabaseError) Error() string {
	if e.Version != "" {
		return fmt.Sprintf("database error in migration %s during %s: %v", e.Version, e.Operation, e.Err)
	}
	return fmt.Sprintf("database error during %s: %v", e.Operation, e.Err)
}

// Unwrap returns the underlying error
func (e *DatabaseError) Unwrap() error {
	return e.Err
}

// NewDatabaseError creates a new DatabaseError
func NewDatabaseError(version, query, operation string, err error) *DatabaseError {
	return &DatabaseError{
		Version:   version,
		Query:     query,
		Operation: operation,
		Err:       err,
	}
}