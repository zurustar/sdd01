package migration

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // SQLite driver
)

// SQLiteConfig holds SQLite-specific database configuration
type SQLiteConfig struct {
	// DSN is the database file path or connection string
	DSN string
	
	// BusyTimeout sets how long to wait for database locks
	BusyTimeout time.Duration
	
	// EnableForeignKeys enables foreign key constraint checking
	EnableForeignKeys bool
	
	// JournalMode sets the SQLite journal mode (WAL, DELETE, TRUNCATE, etc.)
	JournalMode string
	
	// Synchronous sets the synchronous mode (FULL, NORMAL, OFF)
	Synchronous string
	
	// CacheSize sets the page cache size in KB (negative for pages)
	CacheSize int
	
	// MaxOpenConns sets the maximum number of open connections
	MaxOpenConns int
	
	// MaxIdleConns sets the maximum number of idle connections
	MaxIdleConns int
	
	// ConnMaxLifetime sets the maximum lifetime of connections
	ConnMaxLifetime time.Duration
}

// MigrationConfig holds migration-specific configuration
type MigrationConfig struct {
	// MigrationDir is the directory containing migration files
	MigrationDir string
	
	// Enabled controls whether migrations should run
	Enabled bool
	
	// TimeoutPerFile sets the timeout for each migration file execution
	TimeoutPerFile time.Duration
	
	// MaxRetries sets the number of retry attempts for failed migrations
	MaxRetries int
	
	// VerifyChecksum enables checksum verification of migration files
	VerifyChecksum bool
	
	// CreateDirIfNotExists creates the migration directory if it doesn't exist
	CreateDirIfNotExists bool
}

// ConnectionManager manages SQLite database connections with proper configuration
type ConnectionManager interface {
	// GetConnection returns a configured SQLite database connection
	GetConnection() (*sql.DB, error)
	
	// ConfigureDatabase applies SQLite-specific settings to an existing connection
	ConfigureDatabase(db *sql.DB) error
	
	// CreateDatabaseFile creates the database file if it doesn't exist
	CreateDatabaseFile() error
	
	// ValidateConfig validates the SQLite configuration
	ValidateConfig() error
}

// sqliteConnectionManager implements ConnectionManager for SQLite
type sqliteConnectionManager struct {
	config SQLiteConfig
}

// NewConnectionManager creates a new SQLite connection manager
func NewConnectionManager(config SQLiteConfig) ConnectionManager {
	return &sqliteConnectionManager{
		config: config,
	}
}

// GetConnection returns a configured SQLite database connection
func (cm *sqliteConnectionManager) GetConnection() (*sql.DB, error) {
	// Validate configuration first
	if err := cm.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid SQLite configuration: %w", err)
	}
	
	// Create database file if it doesn't exist
	if err := cm.CreateDatabaseFile(); err != nil {
		return nil, fmt.Errorf("failed to create database file: %w", err)
	}
	
	// Open database connection
	db, err := sql.Open("sqlite", cm.config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}
	
	// Configure connection pool settings
	if cm.config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cm.config.MaxOpenConns)
	}
	
	if cm.config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cm.config.MaxIdleConns)
	}
	
	if cm.config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cm.config.ConnMaxLifetime)
	}
	
	// Apply SQLite-specific configuration
	if err := cm.ConfigureDatabase(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure SQLite database: %w", err)
	}
	
	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}
	
	return db, nil
}

// ConfigureDatabase applies SQLite-specific settings to an existing connection
func (cm *sqliteConnectionManager) ConfigureDatabase(db *sql.DB) error {
	// Apply PRAGMA settings for optimal performance and behavior
	pragmas := []struct {
		name  string
		value interface{}
	}{
		{"busy_timeout", int(cm.config.BusyTimeout.Milliseconds())},
		{"journal_mode", cm.config.JournalMode},
		{"synchronous", cm.config.Synchronous},
	}
	
	// Add foreign keys pragma if enabled
	if cm.config.EnableForeignKeys {
		pragmas = append(pragmas, struct {
			name  string
			value interface{}
		}{"foreign_keys", "ON"})
	}
	
	// Add cache size if specified
	if cm.config.CacheSize != 0 {
		pragmas = append(pragmas, struct {
			name  string
			value interface{}
		}{"cache_size", cm.config.CacheSize})
	}
	
	// Execute PRAGMA statements
	for _, pragma := range pragmas {
		var sql string
		switch v := pragma.value.(type) {
		case string:
			sql = fmt.Sprintf("PRAGMA %s = %s", pragma.name, v)
		case int:
			sql = fmt.Sprintf("PRAGMA %s = %d", pragma.name, v)
		default:
			sql = fmt.Sprintf("PRAGMA %s = %v", pragma.name, v)
		}
		
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("failed to set PRAGMA %s: %w", pragma.name, err)
		}
	}
	
	return nil
}

// CreateDatabaseFile creates the database file if it doesn't exist
func (cm *sqliteConnectionManager) CreateDatabaseFile() error {
	// Skip if using in-memory database
	if cm.config.DSN == ":memory:" {
		return nil
	}
	
	// Get the directory path
	dbDir := filepath.Dir(cm.config.DSN)
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
	}
	
	// Check if database file already exists
	if _, err := os.Stat(cm.config.DSN); err == nil {
		return nil // File already exists
	}
	
	// Create empty database file with proper permissions
	file, err := os.OpenFile(cm.config.DSN, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create database file %s: %w", cm.config.DSN, err)
	}
	
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close database file %s: %w", cm.config.DSN, err)
	}
	
	return nil
}

// ValidateConfig validates the SQLite configuration
func (cm *sqliteConnectionManager) ValidateConfig() error {
	// Validate DSN
	if cm.config.DSN == "" {
		return fmt.Errorf("DSN cannot be empty")
	}
	
	// Validate timeout
	if cm.config.BusyTimeout < 0 {
		return fmt.Errorf("BusyTimeout cannot be negative")
	}
	
	// Validate journal mode
	validJournalModes := map[string]bool{
		"DELETE":   true,
		"TRUNCATE": true,
		"PERSIST":  true,
		"MEMORY":   true,
		"WAL":      true,
		"OFF":      true,
	}
	
	if cm.config.JournalMode != "" && !validJournalModes[cm.config.JournalMode] {
		return fmt.Errorf("invalid journal mode: %s", cm.config.JournalMode)
	}
	
	// Validate synchronous mode
	validSyncModes := map[string]bool{
		"OFF":    true,
		"NORMAL": true,
		"FULL":   true,
		"EXTRA":  true,
	}
	
	if cm.config.Synchronous != "" && !validSyncModes[cm.config.Synchronous] {
		return fmt.Errorf("invalid synchronous mode: %s", cm.config.Synchronous)
	}
	
	// Validate connection pool settings
	if cm.config.MaxOpenConns < 0 {
		return fmt.Errorf("MaxOpenConns cannot be negative")
	}
	
	if cm.config.MaxIdleConns < 0 {
		return fmt.Errorf("MaxIdleConns cannot be negative")
	}
	
	if cm.config.ConnMaxLifetime < 0 {
		return fmt.Errorf("ConnMaxLifetime cannot be negative")
	}
	
	return nil
}

// DefaultSQLiteConfig returns a SQLite configuration with sensible defaults
func DefaultSQLiteConfig(databasePath string) SQLiteConfig {
	return SQLiteConfig{
		DSN:               databasePath,
		BusyTimeout:       30 * time.Second,
		EnableForeignKeys: true,
		JournalMode:       "WAL",
		Synchronous:       "NORMAL",
		CacheSize:         -2000, // 2000 pages
		MaxOpenConns:      25,
		MaxIdleConns:      5,
		ConnMaxLifetime:   5 * time.Minute,
	}
}

// DefaultMigrationConfig returns a migration configuration with sensible defaults
func DefaultMigrationConfig(migrationDir string) MigrationConfig {
	return MigrationConfig{
		MigrationDir:         migrationDir,
		Enabled:              true,
		TimeoutPerFile:       5 * time.Minute,
		MaxRetries:           3,
		VerifyChecksum:       false,
		CreateDirIfNotExists: true,
	}
}

// TestMigrationConfig returns a migration configuration optimized for testing scenarios
func TestMigrationConfig(migrationDir string) MigrationConfig {
	return MigrationConfig{
		MigrationDir:         migrationDir,
		Enabled:              true,
		TimeoutPerFile:       30 * time.Second, // Shorter timeout for tests
		MaxRetries:           1,                 // Fewer retries for faster test execution
		VerifyChecksum:       false,             // Skip checksum verification in tests
		CreateDirIfNotExists: true,
	}
}

// InMemoryTestSQLiteConfig returns a SQLite configuration optimized for in-memory testing
func InMemoryTestSQLiteConfig() SQLiteConfig {
	return SQLiteConfig{
		DSN:               ":memory:",
		BusyTimeout:       5 * time.Second,  // Shorter timeout for tests
		EnableForeignKeys: true,             // Enable for testing FK constraints
		JournalMode:       "MEMORY",         // Use memory journal for speed
		Synchronous:       "OFF",            // Fastest mode for testing
		CacheSize:         -1000,            // Smaller cache for tests
		MaxOpenConns:      1,                // Single connection for in-memory
		MaxIdleConns:      1,                // Single idle connection
		ConnMaxLifetime:   1 * time.Minute,  // Shorter lifetime for tests
	}
}

// TempFileTestSQLiteConfig returns a SQLite configuration for temporary file-based testing
func TempFileTestSQLiteConfig(tempFilePath string) SQLiteConfig {
	return SQLiteConfig{
		DSN:               tempFilePath,
		BusyTimeout:       5 * time.Second,  // Shorter timeout for tests
		EnableForeignKeys: true,             // Enable for testing FK constraints
		JournalMode:       "MEMORY",         // Use memory journal for speed
		Synchronous:       "OFF",            // Fastest mode for testing
		CacheSize:         -1000,            // Smaller cache for tests
		MaxOpenConns:      5,                // Limited connections for tests
		MaxIdleConns:      2,                // Limited idle connections
		ConnMaxLifetime:   1 * time.Minute,  // Shorter lifetime for tests
	}
}

// ValidateMigrationConfig validates the migration configuration
func ValidateMigrationConfig(config MigrationConfig) error {
	// Validate migration directory
	if config.MigrationDir == "" {
		return fmt.Errorf("MigrationDir cannot be empty")
	}
	
	// Validate timeout
	if config.TimeoutPerFile <= 0 {
		return fmt.Errorf("TimeoutPerFile must be positive")
	}
	
	// Validate retry count
	if config.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries cannot be negative")
	}
	
	// Check if migration directory exists (if not creating it)
	if !config.CreateDirIfNotExists {
		if _, err := os.Stat(config.MigrationDir); os.IsNotExist(err) {
			return fmt.Errorf("migration directory does not exist: %s", config.MigrationDir)
		}
	}
	
	return nil
}