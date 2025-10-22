package migration

import (
	"context"
	"time"
)

// Migration represents a database migration with its metadata and SQL content
type Migration struct {
	Version     string    // Version identifier (e.g., "001", "002")
	Description string    // Human-readable description of the migration
	SQL         string    // SQL statements to execute
	FilePath    string    // Path to the migration file
	Checksum    string    // Optional checksum for verification
	AppliedAt   time.Time // Timestamp when migration was applied (for tracking)
}

// MigrationManager orchestrates the migration process
type MigrationManager interface {
	// RunMigrations executes all pending migrations in sequential order
	RunMigrations(ctx context.Context) error
	
	// GetAppliedVersions returns list of migration versions that have been applied
	GetAppliedVersions(ctx context.Context) ([]string, error)
	
	// GetPendingMigrations returns list of migrations that need to be applied
	GetPendingMigrations(ctx context.Context) ([]Migration, error)
	
	// GetMigrationStatus returns status information about migrations
	GetMigrationStatus(ctx context.Context) (*MigrationStatus, error)
	
	// ListAppliedMigrations returns all applied migrations with timestamps and execution details
	ListAppliedMigrations(ctx context.Context) ([]AppliedMigration, error)
	
	// LogCurrentSchemaVersion logs the current database schema version
	LogCurrentSchemaVersion(ctx context.Context) error
	
	// LogPendingMigrations logs information about pending migrations before execution
	LogPendingMigrations(ctx context.Context) error
}

// FileScanner handles scanning and parsing migration files from the filesystem
type FileScanner interface {
	// ScanMigrations scans the migration directory for migration files
	ScanMigrations(migrationDir string) ([]Migration, error)
	
	// ValidateFileName checks if migration file follows naming convention
	ValidateFileName(filename string) error
	
	// ParseMigrationFile reads and parses a single migration file
	ParseMigrationFile(filePath string) (*Migration, error)
}

// Executor handles the actual execution of migrations against the database
type Executor interface {
	// ExecuteMigration runs a single migration within a transaction
	ExecuteMigration(ctx context.Context, migration Migration) error
	
	// InitializeVersionTable creates the schema_migrations table if it doesn't exist
	InitializeVersionTable(ctx context.Context) error
	
	// RecordMigration records a successful migration in the version tracking table
	RecordMigration(ctx context.Context, version string, executionTime time.Duration) error
	
	// IsVersionApplied checks if a specific migration version has been applied
	IsVersionApplied(ctx context.Context, version string) (bool, error)
	
	// GetAppliedVersions returns all applied migration versions with timestamps
	GetAppliedVersions(ctx context.Context) ([]AppliedMigration, error)
}

// MigrationStatus provides information about the current migration state
type MigrationStatus struct {
	CurrentVersion    string              // Latest applied migration version
	PendingCount      int                 // Number of pending migrations
	AppliedMigrations []AppliedMigration  // List of applied migrations
	PendingMigrations []Migration         // List of pending migrations
}

// AppliedMigration represents a migration that has been successfully applied
type AppliedMigration struct {
	Version         string        // Migration version
	AppliedAt       time.Time     // When the migration was applied
	ExecutionTime   time.Duration // How long the migration took to execute
	Checksum        string        // Checksum of the migration file when applied
}