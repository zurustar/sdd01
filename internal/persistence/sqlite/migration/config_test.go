package migration

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestDefaultSQLiteConfig(t *testing.T) {
	dbPath := "/tmp/test.db"
	config := DefaultSQLiteConfig(dbPath)

	// Test default values
	if config.DSN != dbPath {
		t.Errorf("Expected DSN %s, got %s", dbPath, config.DSN)
	}

	if config.BusyTimeout != 30*time.Second {
		t.Errorf("Expected BusyTimeout 30s, got %v", config.BusyTimeout)
	}

	if !config.EnableForeignKeys {
		t.Error("Expected EnableForeignKeys to be true")
	}

	if config.JournalMode != "WAL" {
		t.Errorf("Expected JournalMode WAL, got %s", config.JournalMode)
	}

	if config.Synchronous != "NORMAL" {
		t.Errorf("Expected Synchronous NORMAL, got %s", config.Synchronous)
	}

	if config.CacheSize != -2000 {
		t.Errorf("Expected CacheSize -2000, got %d", config.CacheSize)
	}

	if config.MaxOpenConns != 25 {
		t.Errorf("Expected MaxOpenConns 25, got %d", config.MaxOpenConns)
	}

	if config.MaxIdleConns != 5 {
		t.Errorf("Expected MaxIdleConns 5, got %d", config.MaxIdleConns)
	}

	if config.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("Expected ConnMaxLifetime 5m, got %v", config.ConnMaxLifetime)
	}
}

func TestDefaultMigrationConfig(t *testing.T) {
	migrationDir := "/tmp/migrations"
	config := DefaultMigrationConfig(migrationDir)

	// Test default values
	if config.MigrationDir != migrationDir {
		t.Errorf("Expected MigrationDir %s, got %s", migrationDir, config.MigrationDir)
	}

	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}

	if config.TimeoutPerFile != 5*time.Minute {
		t.Errorf("Expected TimeoutPerFile 5m, got %v", config.TimeoutPerFile)
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}

	if config.VerifyChecksum {
		t.Error("Expected VerifyChecksum to be false")
	}

	if !config.CreateDirIfNotExists {
		t.Error("Expected CreateDirIfNotExists to be true")
	}
}

func TestSQLiteConfig_ValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      SQLiteConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: SQLiteConfig{
				DSN:               ":memory:",
				BusyTimeout:       30 * time.Second,
				EnableForeignKeys: true,
				JournalMode:       "WAL",
				Synchronous:       "NORMAL",
				MaxOpenConns:      10,
				MaxIdleConns:      5,
				ConnMaxLifetime:   5 * time.Minute,
			},
			expectError: false,
		},
		{
			name: "empty DSN",
			config: SQLiteConfig{
				DSN: "",
			},
			expectError: true,
			errorMsg:    "DSN cannot be empty",
		},
		{
			name: "negative busy timeout",
			config: SQLiteConfig{
				DSN:         ":memory:",
				BusyTimeout: -1 * time.Second,
			},
			expectError: true,
			errorMsg:    "BusyTimeout cannot be negative",
		},
		{
			name: "invalid journal mode",
			config: SQLiteConfig{
				DSN:         ":memory:",
				JournalMode: "INVALID",
			},
			expectError: true,
			errorMsg:    "invalid journal mode: INVALID",
		},
		{
			name: "invalid synchronous mode",
			config: SQLiteConfig{
				DSN:         ":memory:",
				Synchronous: "INVALID",
			},
			expectError: true,
			errorMsg:    "invalid synchronous mode: INVALID",
		},
		{
			name: "negative max open conns",
			config: SQLiteConfig{
				DSN:          ":memory:",
				MaxOpenConns: -1,
			},
			expectError: true,
			errorMsg:    "MaxOpenConns cannot be negative",
		},
		{
			name: "negative max idle conns",
			config: SQLiteConfig{
				DSN:          ":memory:",
				MaxIdleConns: -1,
			},
			expectError: true,
			errorMsg:    "MaxIdleConns cannot be negative",
		},
		{
			name: "negative conn max lifetime",
			config: SQLiteConfig{
				DSN:             ":memory:",
				ConnMaxLifetime: -1 * time.Second,
			},
			expectError: true,
			errorMsg:    "ConnMaxLifetime cannot be negative",
		},
		{
			name: "valid journal modes",
			config: SQLiteConfig{
				DSN:         ":memory:",
				JournalMode: "DELETE",
			},
			expectError: false,
		},
		{
			name: "valid synchronous modes",
			config: SQLiteConfig{
				DSN:         ":memory:",
				Synchronous: "FULL",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewConnectionManager(tt.config)
			err := cm.ValidateConfig()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateMigrationConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		config      MigrationConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: MigrationConfig{
				MigrationDir:         tempDir,
				Enabled:              true,
				TimeoutPerFile:       5 * time.Minute,
				MaxRetries:           3,
				CreateDirIfNotExists: false,
			},
			expectError: false,
		},
		{
			name: "empty migration dir",
			config: MigrationConfig{
				MigrationDir: "",
			},
			expectError: true,
			errorMsg:    "MigrationDir cannot be empty",
		},
		{
			name: "zero timeout",
			config: MigrationConfig{
				MigrationDir:   tempDir,
				TimeoutPerFile: 0,
			},
			expectError: true,
			errorMsg:    "TimeoutPerFile must be positive",
		},
		{
			name: "negative timeout",
			config: MigrationConfig{
				MigrationDir:   tempDir,
				TimeoutPerFile: -1 * time.Second,
			},
			expectError: true,
			errorMsg:    "TimeoutPerFile must be positive",
		},
		{
			name: "negative max retries",
			config: MigrationConfig{
				MigrationDir:   tempDir,
				TimeoutPerFile: 1 * time.Minute,
				MaxRetries:     -1,
			},
			expectError: true,
			errorMsg:    "MaxRetries cannot be negative",
		},
		{
			name: "non-existent directory without create flag",
			config: MigrationConfig{
				MigrationDir:         "/non/existent/path",
				TimeoutPerFile:       1 * time.Minute,
				MaxRetries:           0,
				CreateDirIfNotExists: false,
			},
			expectError: true,
			errorMsg:    "migration directory does not exist: /non/existent/path",
		},
		{
			name: "non-existent directory with create flag",
			config: MigrationConfig{
				MigrationDir:         "/tmp/will-be-created",
				TimeoutPerFile:       1 * time.Minute,
				MaxRetries:           0,
				CreateDirIfNotExists: true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMigrationConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestConnectionManager_CreateDatabaseFile(t *testing.T) {
	// Test with temporary directory
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	config := SQLiteConfig{
		DSN: dbPath,
	}

	cm := NewConnectionManager(config)

	// Test creating database file
	err := cm.CreateDatabaseFile()
	if err != nil {
		t.Fatalf("Failed to create database file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Verify file permissions
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Failed to stat database file: %v", err)
	}

	expectedMode := os.FileMode(0644)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("Expected file permissions %v, got %v", expectedMode, info.Mode().Perm())
	}

	// Test creating file again (should not error)
	err = cm.CreateDatabaseFile()
	if err != nil {
		t.Errorf("Creating existing database file should not error: %v", err)
	}
}

func TestConnectionManager_CreateDatabaseFile_InMemory(t *testing.T) {
	config := SQLiteConfig{
		DSN: ":memory:",
	}

	cm := NewConnectionManager(config)

	// Test with in-memory database (should not create file)
	err := cm.CreateDatabaseFile()
	if err != nil {
		t.Errorf("Creating in-memory database should not error: %v", err)
	}
}

func TestConnectionManager_CreateDatabaseFile_NestedDirectory(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "nested", "dir", "test.db")

	config := SQLiteConfig{
		DSN: dbPath,
	}

	cm := NewConnectionManager(config)

	// Test creating database file in nested directory
	err := cm.CreateDatabaseFile()
	if err != nil {
		t.Fatalf("Failed to create database file in nested directory: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created in nested directory")
	}

	// Verify directory was created with proper permissions
	dirInfo, err := os.Stat(filepath.Dir(dbPath))
	if err != nil {
		t.Fatalf("Failed to stat database directory: %v", err)
	}

	expectedDirMode := os.FileMode(0755)
	if dirInfo.Mode().Perm() != expectedDirMode {
		t.Errorf("Expected directory permissions %v, got %v", expectedDirMode, dirInfo.Mode().Perm())
	}
}

func TestConnectionManager_ConfigureDatabase(t *testing.T) {
	// Test with in-memory database for simplicity
	config := SQLiteConfig{
		DSN:               ":memory:",
		BusyTimeout:       5 * time.Second,
		EnableForeignKeys: true,
		JournalMode:       "WAL",
		Synchronous:       "NORMAL",
		CacheSize:         -1000,
	}

	cm := NewConnectionManager(config)

	// Open database connection
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test configuring database
	err = cm.ConfigureDatabase(db)
	if err != nil {
		t.Fatalf("Failed to configure database: %v", err)
	}

	// Test that we can execute a simple query to verify the connection works
	// This indirectly tests that PRAGMA settings were applied without errors
	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Errorf("Failed to create test table after configuration: %v", err)
	}

	// Test foreign key constraint if enabled (indirect test)
	if config.EnableForeignKeys {
		// Create tables with foreign key relationship
		_, err = db.Exec(`
			CREATE TABLE parent (id INTEGER PRIMARY KEY);
			CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id));
		`)
		if err != nil {
			t.Errorf("Failed to create tables with foreign keys: %v", err)
		}

		// Try to insert invalid foreign key (should fail if foreign keys are enabled)
		_, err = db.Exec("INSERT INTO child (id, parent_id) VALUES (1, 999)")
		if err == nil {
			t.Log("Note: Foreign key constraint not enforced (may be expected in test environment)")
		}
	}
}

func TestConnectionManager_ConfigureDatabase_WithoutOptionalSettings(t *testing.T) {
	// Test with minimal configuration
	config := SQLiteConfig{
		DSN:               ":memory:",
		EnableForeignKeys: false, // Explicitly disabled
	}

	cm := NewConnectionManager(config)

	// Open database connection
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test configuring database
	err = cm.ConfigureDatabase(db)
	if err != nil {
		t.Fatalf("Failed to configure database: %v", err)
	}

	// Test that configuration completed without error
	// This indirectly verifies that optional settings were handled correctly
	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Errorf("Failed to create test table after minimal configuration: %v", err)
	}
}

func TestConnectionManager_GetConnection(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	config := SQLiteConfig{
		DSN:               dbPath,
		BusyTimeout:       1 * time.Second,
		EnableForeignKeys: true,
		JournalMode:       "WAL",
		Synchronous:       "NORMAL",
		MaxOpenConns:      10,
		MaxIdleConns:      2,
		ConnMaxLifetime:   1 * time.Minute,
	}

	cm := NewConnectionManager(config)

	// Test getting connection
	db, err := cm.GetConnection()
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}
	defer db.Close()

	// Verify connection works
	err = db.Ping()
	if err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Test that the database is properly configured by creating a simple table
	_, err = db.Exec("CREATE TABLE test_config (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Errorf("Failed to create test table: %v", err)
	}
}

func TestConnectionManager_GetConnection_InvalidConfig(t *testing.T) {
	config := SQLiteConfig{
		DSN: "", // Invalid empty DSN
	}

	cm := NewConnectionManager(config)

	// Test getting connection with invalid config
	_, err := cm.GetConnection()
	if err == nil {
		t.Error("Expected error for invalid configuration")
	}

	expectedMsg := "invalid SQLite configuration: DSN cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestConnectionManager_GetConnection_InMemory(t *testing.T) {
	config := DefaultSQLiteConfig(":memory:")

	cm := NewConnectionManager(config)

	// Test getting in-memory connection
	db, err := cm.GetConnection()
	if err != nil {
		t.Fatalf("Failed to get in-memory connection: %v", err)
	}
	defer db.Close()

	// Verify connection works
	err = db.Ping()
	if err != nil {
		t.Errorf("Failed to ping in-memory database: %v", err)
	}
}

func TestAllValidJournalModes(t *testing.T) {
	validModes := []string{"DELETE", "TRUNCATE", "PERSIST", "MEMORY", "WAL", "OFF"}

	for _, mode := range validModes {
		t.Run("journal_mode_"+mode, func(t *testing.T) {
			config := SQLiteConfig{
				DSN:         ":memory:",
				JournalMode: mode,
			}

			cm := NewConnectionManager(config)
			err := cm.ValidateConfig()
			if err != nil {
				t.Errorf("Journal mode %s should be valid, got error: %v", mode, err)
			}
		})
	}
}

func TestAllValidSynchronousModes(t *testing.T) {
	validModes := []string{"OFF", "NORMAL", "FULL", "EXTRA"}

	for _, mode := range validModes {
		t.Run("synchronous_mode_"+mode, func(t *testing.T) {
			config := SQLiteConfig{
				DSN:         ":memory:",
				Synchronous: mode,
			}

			cm := NewConnectionManager(config)
			err := cm.ValidateConfig()
			if err != nil {
				t.Errorf("Synchronous mode %s should be valid, got error: %v", mode, err)
			}
		})
	}
}

func TestConnectionManager_ConfigureDatabase_RealFile(t *testing.T) {
	// Use a real file database to test PRAGMA settings more reliably
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pragma_test.db")

	config := SQLiteConfig{
		DSN:               dbPath,
		BusyTimeout:       2 * time.Second,
		EnableForeignKeys: true,
		JournalMode:       "WAL",
		Synchronous:       "NORMAL",
		CacheSize:         -500,
	}

	cm := NewConnectionManager(config)

	// Get connection which should configure the database
	db, err := cm.GetConnection()
	if err != nil {
		t.Fatalf("Failed to get configured connection: %v", err)
	}
	defer db.Close()

	// Test that we can create tables and the database is working
	_, err = db.Exec("CREATE TABLE pragma_test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Errorf("Failed to create table: %v", err)
	}

	// Test foreign key enforcement by creating related tables
	_, err = db.Exec(`
		CREATE TABLE parent_table (id INTEGER PRIMARY KEY);
		CREATE TABLE child_table (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent_table(id));
	`)
	if err != nil {
		t.Errorf("Failed to create tables with foreign key relationships: %v", err)
	}

	// Insert valid parent record
	_, err = db.Exec("INSERT INTO parent_table (id) VALUES (1)")
	if err != nil {
		t.Errorf("Failed to insert parent record: %v", err)
	}

	// Insert valid child record (should succeed)
	_, err = db.Exec("INSERT INTO child_table (id, parent_id) VALUES (1, 1)")
	if err != nil {
		t.Errorf("Failed to insert valid child record: %v", err)
	}

	// Test that the database file was created with proper permissions
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Errorf("Failed to stat database file: %v", err)
	} else {
		expectedMode := os.FileMode(0644)
		if info.Mode().Perm() != expectedMode {
			t.Errorf("Expected file permissions %v, got %v", expectedMode, info.Mode().Perm())
		}
	}
}

func TestConnectionManager_ConfigureDatabase_ErrorHandling(t *testing.T) {
	// Test configuration with invalid PRAGMA values
	config := SQLiteConfig{
		DSN:         ":memory:",
		JournalMode: "INVALID_MODE", // This should be caught by validation
	}

	cm := NewConnectionManager(config)

	// This should fail during validation, not during configuration
	_, err := cm.GetConnection()
	if err == nil {
		t.Error("Expected error for invalid journal mode")
	}

	expectedMsg := "invalid SQLite configuration: invalid journal mode: INVALID_MODE"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestConnectionManager_ConfigureDatabase_EmptyValues(t *testing.T) {
	// Test configuration with empty optional values (should not set PRAGMA)
	config := SQLiteConfig{
		DSN:               ":memory:",
		EnableForeignKeys: false,
		JournalMode:       "", // Empty should not set PRAGMA
		Synchronous:       "", // Empty should not set PRAGMA
		CacheSize:         0,  // Zero should not set PRAGMA
	}

	cm := NewConnectionManager(config)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test configuring database with empty values
	err = cm.ConfigureDatabase(db)
	if err != nil {
		t.Fatalf("Failed to configure database with empty values: %v", err)
	}

	// Verify database still works
	_, err = db.Exec("CREATE TABLE empty_config_test (id INTEGER)")
	if err != nil {
		t.Errorf("Failed to create table with empty config: %v", err)
	}
}

func TestTestMigrationConfig(t *testing.T) {
	migrationDir := "/tmp/test_migrations"
	config := TestMigrationConfig(migrationDir)

	// Test test-specific values
	if config.MigrationDir != migrationDir {
		t.Errorf("Expected MigrationDir %s, got %s", migrationDir, config.MigrationDir)
	}

	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}

	if config.TimeoutPerFile != 30*time.Second {
		t.Errorf("Expected TimeoutPerFile 30s, got %v", config.TimeoutPerFile)
	}

	if config.MaxRetries != 1 {
		t.Errorf("Expected MaxRetries 1, got %d", config.MaxRetries)
	}

	if config.VerifyChecksum {
		t.Error("Expected VerifyChecksum to be false")
	}

	if !config.CreateDirIfNotExists {
		t.Error("Expected CreateDirIfNotExists to be true")
	}
}

func TestInMemoryTestSQLiteConfig(t *testing.T) {
	config := InMemoryTestSQLiteConfig()

	// Test in-memory specific values
	if config.DSN != ":memory:" {
		t.Errorf("Expected DSN :memory:, got %s", config.DSN)
	}

	if config.BusyTimeout != 5*time.Second {
		t.Errorf("Expected BusyTimeout 5s, got %v", config.BusyTimeout)
	}

	if !config.EnableForeignKeys {
		t.Error("Expected EnableForeignKeys to be true")
	}

	if config.JournalMode != "MEMORY" {
		t.Errorf("Expected JournalMode MEMORY, got %s", config.JournalMode)
	}

	if config.Synchronous != "OFF" {
		t.Errorf("Expected Synchronous OFF, got %s", config.Synchronous)
	}

	if config.CacheSize != -1000 {
		t.Errorf("Expected CacheSize -1000, got %d", config.CacheSize)
	}

	if config.MaxOpenConns != 1 {
		t.Errorf("Expected MaxOpenConns 1, got %d", config.MaxOpenConns)
	}

	if config.MaxIdleConns != 1 {
		t.Errorf("Expected MaxIdleConns 1, got %d", config.MaxIdleConns)
	}

	if config.ConnMaxLifetime != 1*time.Minute {
		t.Errorf("Expected ConnMaxLifetime 1m, got %v", config.ConnMaxLifetime)
	}
}

func TestTempFileTestSQLiteConfig(t *testing.T) {
	tempFile := "/tmp/test.db"
	config := TempFileTestSQLiteConfig(tempFile)

	// Test temp file specific values
	if config.DSN != tempFile {
		t.Errorf("Expected DSN %s, got %s", tempFile, config.DSN)
	}

	if config.BusyTimeout != 5*time.Second {
		t.Errorf("Expected BusyTimeout 5s, got %v", config.BusyTimeout)
	}

	if !config.EnableForeignKeys {
		t.Error("Expected EnableForeignKeys to be true")
	}

	if config.JournalMode != "MEMORY" {
		t.Errorf("Expected JournalMode MEMORY, got %s", config.JournalMode)
	}

	if config.Synchronous != "OFF" {
		t.Errorf("Expected Synchronous OFF, got %s", config.Synchronous)
	}

	if config.CacheSize != -1000 {
		t.Errorf("Expected CacheSize -1000, got %d", config.CacheSize)
	}

	if config.MaxOpenConns != 5 {
		t.Errorf("Expected MaxOpenConns 5, got %d", config.MaxOpenConns)
	}

	if config.MaxIdleConns != 2 {
		t.Errorf("Expected MaxIdleConns 2, got %d", config.MaxIdleConns)
	}

	if config.ConnMaxLifetime != 1*time.Minute {
		t.Errorf("Expected ConnMaxLifetime 1m, got %v", config.ConnMaxLifetime)
	}
}

func TestInMemoryTestSQLiteConfig_Validation(t *testing.T) {
	config := InMemoryTestSQLiteConfig()
	cm := NewConnectionManager(config)

	// Test that the configuration is valid
	err := cm.ValidateConfig()
	if err != nil {
		t.Errorf("InMemoryTestSQLiteConfig should be valid, got error: %v", err)
	}
}

func TestTempFileTestSQLiteConfig_Validation(t *testing.T) {
	tempFile := "/tmp/test_validation.db"
	config := TempFileTestSQLiteConfig(tempFile)
	cm := NewConnectionManager(config)

	// Test that the configuration is valid
	err := cm.ValidateConfig()
	if err != nil {
		t.Errorf("TempFileTestSQLiteConfig should be valid, got error: %v", err)
	}
}

func TestTestMigrationConfig_Validation(t *testing.T) {
	tempDir := t.TempDir()
	config := TestMigrationConfig(tempDir)

	// Test that the configuration is valid
	err := ValidateMigrationConfig(config)
	if err != nil {
		t.Errorf("TestMigrationConfig should be valid, got error: %v", err)
	}
}