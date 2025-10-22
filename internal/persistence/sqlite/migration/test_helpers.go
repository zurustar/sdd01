package migration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

// TestMigrationHelper provides utilities for testing migrations with in-memory databases
type TestMigrationHelper struct {
	db              *sql.DB
	migrationManager MigrationManager
	testDataDir     string
}

// NewTestMigrationHelper creates a new helper for testing migrations with in-memory SQLite database
func NewTestMigrationHelper() (*TestMigrationHelper, error) {
	// Use optimized in-memory configuration for testing
	config := InMemoryTestSQLiteConfig()
	cm := NewConnectionManager(config)
	
	// Get configured database connection
	db, err := cm.GetConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-memory database: %w", err)
	}

	// Get the path to test migration files
	_, currentFile, _, _ := runtime.Caller(0)
	testDataDir := filepath.Join(filepath.Dir(currentFile), "testdata")

	// Create migration components with test-optimized configuration
	scanner := NewFileScanner()
	executor := NewSQLiteExecutor(db)
	manager := NewMigrationManager(scanner, executor, testDataDir)

	return &TestMigrationHelper{
		db:              db,
		migrationManager: manager,
		testDataDir:     testDataDir,
	}, nil
}

// NewTestMigrationHelperWithRealDB creates a helper with a real SQLite database for integration testing
func NewTestMigrationHelperWithRealDB() (*TestMigrationHelper, string, error) {
	// Create temporary database file
	tempFile := fmt.Sprintf("/tmp/test_migration_%d.db", time.Now().UnixNano())
	
	// Use optimized temporary file configuration for testing
	config := TempFileTestSQLiteConfig(tempFile)
	cm := NewConnectionManager(config)
	
	// Get configured database connection
	db, err := cm.GetConnection()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temporary database: %w", err)
	}

	// Get the path to test migration files
	_, currentFile, _, _ := runtime.Caller(0)
	testDataDir := filepath.Join(filepath.Dir(currentFile), "testdata")

	// Create migration components with test-optimized configuration
	scanner := NewFileScanner()
	executor := NewSQLiteExecutor(db)
	manager := NewMigrationManager(scanner, executor, testDataDir)

	helper := &TestMigrationHelper{
		db:              db,
		migrationManager: manager,
		testDataDir:     testDataDir,
	}

	return helper, tempFile, nil
}

// Close closes the test database connection
func (h *TestMigrationHelper) Close() error {
	if h.db != nil {
		return h.db.Close()
	}
	return nil
}

// GetDB returns the underlying database connection for direct queries
func (h *TestMigrationHelper) GetDB() *sql.DB {
	return h.db
}

// GetMigrationManager returns the migration manager for testing
func (h *TestMigrationHelper) GetMigrationManager() MigrationManager {
	return h.migrationManager
}

// GetTestDataDir returns the path to test migration files
func (h *TestMigrationHelper) GetTestDataDir() string {
	return h.testDataDir
}

// RunTestMigrations executes all test migrations
func (h *TestMigrationHelper) RunTestMigrations(ctx context.Context) error {
	return h.migrationManager.RunMigrations(ctx)
}

// VerifyTestData checks that test data was inserted correctly
func (h *TestMigrationHelper) VerifyTestData(ctx context.Context) error {
	// Check users table
	var userCount int
	err := h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_users").Scan(&userCount)
	if err != nil {
		return fmt.Errorf("failed to count test users: %w", err)
	}
	if userCount != 2 {
		return fmt.Errorf("expected 2 test users, got %d", userCount)
	}

	// Check posts table
	var postCount int
	err = h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_posts").Scan(&postCount)
	if err != nil {
		return fmt.Errorf("failed to count test posts: %w", err)
	}
	if postCount != 3 {
		return fmt.Errorf("expected 3 test posts, got %d", postCount)
	}

	// Check comments table
	var commentCount int
	err = h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_comments").Scan(&commentCount)
	if err != nil {
		return fmt.Errorf("failed to count test comments: %w", err)
	}
	if commentCount != 4 {
		return fmt.Errorf("expected 4 test comments, got %d", commentCount)
	}

	// Verify foreign key constraints work
	var postUserCount int
	err = h.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM test_posts p 
		JOIN test_users u ON p.user_id = u.id
	`).Scan(&postUserCount)
	if err != nil {
		return fmt.Errorf("failed to verify post-user relationships: %w", err)
	}
	if postUserCount != 3 {
		return fmt.Errorf("expected 3 posts with valid user relationships, got %d", postUserCount)
	}

	return nil
}

// ResetDatabase clears all data from the test database while keeping the schema
func (h *TestMigrationHelper) ResetDatabase(ctx context.Context) error {
	// Get list of all tables
	rows, err := h.db.QueryContext(ctx, `
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name != 'schema_migrations'
	`)
	if err != nil {
		return fmt.Errorf("failed to get table list: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	// Delete data from all tables (in reverse order to handle foreign keys)
	for i := len(tables) - 1; i >= 0; i-- {
		_, err := h.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", tables[i]))
		if err != nil {
			return fmt.Errorf("failed to clear table %s: %w", tables[i], err)
		}
	}

	return nil
}

// GetMigrationStatus returns the current migration status for testing
func (h *TestMigrationHelper) GetMigrationStatus(ctx context.Context) (*MigrationStatus, error) {
	return h.migrationManager.GetMigrationStatus(ctx)
}

// IsInMemory returns true if this helper is using an in-memory database
func (h *TestMigrationHelper) IsInMemory() bool {
	// Check if we can determine the DSN from the database
	// This is a simple heuristic - in practice, we'd store this information
	return true // For now, assume most test helpers use in-memory databases
}

// ExecuteSQL executes arbitrary SQL for testing purposes
func (h *TestMigrationHelper) ExecuteSQL(ctx context.Context, sql string, args ...interface{}) error {
	_, err := h.db.ExecContext(ctx, sql, args...)
	return err
}

// QuerySQL executes a query and returns the result for testing purposes
func (h *TestMigrationHelper) QuerySQL(ctx context.Context, sql string, args ...interface{}) (*sql.Rows, error) {
	return h.db.QueryContext(ctx, sql, args...)
}

// NewTestMigrationHelperWithCustomDir creates a helper with custom migration directory for testing
func NewTestMigrationHelperWithCustomDir(migrationDir string, useInMemory bool) (*TestMigrationHelper, string, error) {
	var db *sql.DB
	var tempFile string
	var err error
	
	if useInMemory {
		// Use in-memory database
		config := InMemoryTestSQLiteConfig()
		cm := NewConnectionManager(config)
		db, err = cm.GetConnection()
		if err != nil {
			return nil, "", fmt.Errorf("failed to create in-memory database: %w", err)
		}
		tempFile = ":memory:"
	} else {
		// Use temporary file database
		tempFile = fmt.Sprintf("/tmp/test_migration_%d.db", time.Now().UnixNano())
		config := TempFileTestSQLiteConfig(tempFile)
		cm := NewConnectionManager(config)
		db, err = cm.GetConnection()
		if err != nil {
			return nil, "", fmt.Errorf("failed to create temporary database: %w", err)
		}
	}

	// Create migration components with custom directory
	scanner := NewFileScanner()
	executor := NewSQLiteExecutor(db)
	manager := NewMigrationManager(scanner, executor, migrationDir)

	helper := &TestMigrationHelper{
		db:              db,
		migrationManager: manager,
		testDataDir:     migrationDir,
	}

	return helper, tempFile, nil
}

// NewTestMigrationHelperWithSharedDB creates a helper that shares a database file with other helpers
func NewTestMigrationHelperWithSharedDB(sharedDBPath string) (*TestMigrationHelper, string, error) {
	// Use shared database file configuration
	config := TempFileTestSQLiteConfig(sharedDBPath)
	cm := NewConnectionManager(config)
	
	// Get configured database connection
	db, err := cm.GetConnection()
	if err != nil {
		return nil, "", fmt.Errorf("failed to connect to shared database: %w", err)
	}

	// Get the path to test migration files
	_, currentFile, _, _ := runtime.Caller(0)
	testDataDir := filepath.Join(filepath.Dir(currentFile), "testdata")

	// Create migration components with test-optimized configuration
	scanner := NewFileScanner()
	executor := NewSQLiteExecutor(db)
	manager := NewMigrationManager(scanner, executor, testDataDir)

	helper := &TestMigrationHelper{
		db:              db,
		migrationManager: manager,
		testDataDir:     testDataDir,
	}

	return helper, sharedDBPath, nil
}

// CreateTemporaryDatabase creates a temporary file-based SQLite database for testing
func CreateTemporaryDatabase() (*sql.DB, string, error) {
	// Create temporary database file with unique name
	tempFile := fmt.Sprintf("/tmp/test_migration_%d.db", time.Now().UnixNano())
	
	// Use optimized configuration for temporary database
	config := TempFileTestSQLiteConfig(tempFile)
	cm := NewConnectionManager(config)
	
	// Get configured database connection
	db, err := cm.GetConnection()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temporary database: %w", err)
	}

	return db, tempFile, nil
}

// CreateTestMigrationDirectory creates a temporary directory with sample migration files for testing
func CreateTestMigrationDirectory() (string, error) {
	// Create temporary directory
	tempDir := fmt.Sprintf("/tmp/test_migrations_%d", time.Now().UnixNano())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create test migration directory: %w", err)
	}

	// Sample migration files for testing
	migrations := map[string]string{
		"001_create_test_users.sql": `-- Migration: 001_create_test_users.sql
-- Description: Create test users table

CREATE TABLE IF NOT EXISTS test_users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert test data
INSERT INTO test_users (id, username, email) VALUES 
    ('user1', 'testuser1', 'test1@example.com'),
    ('user2', 'testuser2', 'test2@example.com');`,

		"002_create_test_posts.sql": `-- Migration: 002_create_test_posts.sql
-- Description: Create test posts table with foreign key

CREATE TABLE IF NOT EXISTS test_posts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES test_users(id) ON DELETE CASCADE
);

-- Create index for performance
CREATE INDEX IF NOT EXISTS idx_test_posts_user_id ON test_posts(user_id);

-- Insert test data
INSERT INTO test_posts (id, user_id, title, content) VALUES 
    ('post1', 'user1', 'First Test Post', 'Content of first post'),
    ('post2', 'user1', 'Second Test Post', 'Content of second post'),
    ('post3', 'user2', 'User 2 Post', 'Content by user 2');`,

		"003_create_test_comments.sql": `-- Migration: 003_create_test_comments.sql
-- Description: Create test comments table with foreign keys

CREATE TABLE IF NOT EXISTS test_comments (
    id TEXT PRIMARY KEY,
    post_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id) REFERENCES test_posts(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES test_users(id) ON DELETE CASCADE
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_test_comments_post_id ON test_comments(post_id);
CREATE INDEX IF NOT EXISTS idx_test_comments_user_id ON test_comments(user_id);

-- Insert test data
INSERT INTO test_comments (id, post_id, user_id, content) VALUES 
    ('comment1', 'post1', 'user2', 'Great post!'),
    ('comment2', 'post1', 'user1', 'Thanks for feedback'),
    ('comment3', 'post2', 'user2', 'Interesting perspective'),
    ('comment4', 'post3', 'user1', 'Nice work!');`,
	}

	// Write migration files
	for filename, content := range migrations {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write migration file %s: %w", filename, err)
		}
	}

	return tempDir, nil
}

// CleanupTestMigrationDirectory removes a test migration directory and all its contents
func CleanupTestMigrationDirectory(dir string) error {
	if dir == "" || dir == "/" {
		return fmt.Errorf("invalid directory path for cleanup: %s", dir)
	}
	
	// Only clean up directories that look like test directories
	if !filepath.HasPrefix(dir, "/tmp/test_migrations_") {
		return fmt.Errorf("directory does not appear to be a test migration directory: %s", dir)
	}
	
	return os.RemoveAll(dir)
}

// configureTestDatabase applies SQLite configuration suitable for testing
// This function is deprecated - use the configuration-based approach instead
func configureTestDatabase(db *sql.DB) error {
	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Set journal mode to memory for faster testing
	if _, err := db.Exec("PRAGMA journal_mode = MEMORY"); err != nil {
		return fmt.Errorf("failed to set journal mode: %w", err)
	}

	// Set synchronous mode to OFF for faster testing (safe for testing only)
	if _, err := db.Exec("PRAGMA synchronous = OFF"); err != nil {
		return fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	// Set temp store to memory
	if _, err := db.Exec("PRAGMA temp_store = MEMORY"); err != nil {
		return fmt.Errorf("failed to set temp store: %w", err)
	}

	return nil
}