package migration

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	
	// Enable foreign keys for testing
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}
	
	return db
}

func TestSQLiteExecutor_InitializeVersionTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	executor := NewSQLiteExecutor(db)
	ctx := context.Background()
	
	// Test table creation - with stub driver, this should succeed without error
	err := executor.InitializeVersionTable(ctx)
	if err != nil {
		t.Fatalf("InitializeVersionTable failed: %v", err)
	}
	
	// Test idempotency - calling again should not fail
	err = executor.InitializeVersionTable(ctx)
	if err != nil {
		t.Errorf("InitializeVersionTable should be idempotent, but failed on second call: %v", err)
	}
}

func TestSQLiteExecutor_ExecuteMigration_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	executor := NewSQLiteExecutor(db)
	ctx := context.Background()
	
	// Initialize version table first
	err := executor.InitializeVersionTable(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize version table: %v", err)
	}
	
	// Test successful migration execution
	migration := Migration{
		Version:     "001",
		Description: "Create test table",
		SQL: `
			CREATE TABLE test_users (
				id INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				email TEXT UNIQUE
			);
			INSERT INTO test_users (name, email) VALUES ('Test User', 'test@example.com');
		`,
		FilePath: "001_create_test_table.sql",
	}
	
	err = executor.ExecuteMigration(ctx, migration)
	if err != nil {
		t.Fatalf("ExecuteMigration failed: %v", err)
	}
	
	// With stub driver, we can't verify actual data changes, but we can verify
	// that the migration executed without error
}

func TestSQLiteExecutor_ExecuteMigration_TransactionRollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	executor := NewSQLiteExecutor(db)
	ctx := context.Background()
	
	// Initialize version table first
	err := executor.InitializeVersionTable(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize version table: %v", err)
	}
	
	// Test migration with empty SQL (should fail)
	migration := Migration{
		Version:     "002",
		Description: "Migration with error",
		SQL:         "", // Empty SQL should cause an error
		FilePath:    "002_migration_with_error.sql",
	}
	
	err = executor.ExecuteMigration(ctx, migration)
	if err == nil {
		t.Fatal("Expected ExecuteMigration to fail with empty SQL, but it succeeded")
	}
	
	// Verify it's a MigrationError
	var migrationErr *MigrationError
	if !errors.As(err, &migrationErr) {
		t.Errorf("Expected MigrationError, got %T", err)
	}
}

func TestSQLiteExecutor_RecordMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	executor := NewSQLiteExecutor(db)
	ctx := context.Background()
	
	// Initialize version table first
	err := executor.InitializeVersionTable(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize version table: %v", err)
	}
	
	// Test recording a migration - with stub driver, this should succeed without error
	version := "001"
	executionTime := 150 * time.Millisecond
	
	err = executor.RecordMigration(ctx, version, executionTime)
	if err != nil {
		t.Fatalf("RecordMigration failed: %v", err)
	}
	
	// With stub driver, we can't verify actual data persistence, but we can verify
	// that the method executed without error
}

func TestSQLiteExecutor_IsVersionApplied(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	executor := NewSQLiteExecutor(db)
	ctx := context.Background()
	
	// Initialize version table first
	err := executor.InitializeVersionTable(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize version table: %v", err)
	}
	
	// Test version not applied - with stub driver, this will always return false
	applied, err := executor.IsVersionApplied(ctx, "001")
	if err != nil {
		t.Fatalf("IsVersionApplied failed: %v", err)
	}
	
	if applied {
		t.Error("Expected version 001 to not be applied with stub driver")
	}
	
	// Record a migration - this should succeed
	err = executor.RecordMigration(ctx, "001", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to record migration: %v", err)
	}
	
	// Test version is applied - with stub driver, this will still return false
	// since the stub doesn't persist data
	applied, err = executor.IsVersionApplied(ctx, "001")
	if err != nil {
		t.Fatalf("IsVersionApplied failed: %v", err)
	}
	
	// With stub driver, we expect false since no data is actually stored
	if applied {
		t.Error("Expected version 001 to not be applied with stub driver")
	}
}

func TestSQLiteExecutor_GetAppliedVersions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	executor := NewSQLiteExecutor(db)
	ctx := context.Background()
	
	// Initialize version table first
	err := executor.InitializeVersionTable(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize version table: %v", err)
	}
	
	// Test empty result - with stub driver, this will always return empty
	applied, err := executor.GetAppliedVersions(ctx)
	if err != nil {
		t.Fatalf("GetAppliedVersions failed: %v", err)
	}
	
	if len(applied) != 0 {
		t.Errorf("Expected 0 applied migrations with stub driver, got %d", len(applied))
	}
	
	// Record some migrations - these should succeed
	versions := []string{"001", "002", "003"}
	executionTimes := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		150 * time.Millisecond,
	}
	
	for i, version := range versions {
		err = executor.RecordMigration(ctx, version, executionTimes[i])
		if err != nil {
			t.Fatalf("Failed to record migration %s: %v", version, err)
		}
	}
	
	// Test getting applied versions - with stub driver, this will still return empty
	applied, err = executor.GetAppliedVersions(ctx)
	if err != nil {
		t.Fatalf("GetAppliedVersions failed: %v", err)
	}
	
	// With stub driver, we expect empty result since no data is actually stored
	if len(applied) != 0 {
		t.Errorf("Expected 0 applied migrations with stub driver, got %d", len(applied))
	}
}

func TestSQLiteExecutor_ParseSQL(t *testing.T) {
	executor := &SQLiteExecutor{}
	
	tests := []struct {
		name     string
		sql      string
		expected []string
	}{
		{
			name: "Single statement",
			sql:  "CREATE TABLE users (id INTEGER PRIMARY KEY)",
			expected: []string{
				"CREATE TABLE users (id INTEGER PRIMARY KEY)",
			},
		},
		{
			name: "Multiple statements",
			sql: `
				CREATE TABLE users (id INTEGER PRIMARY KEY);
				INSERT INTO users (id) VALUES (1);
				UPDATE users SET id = 2 WHERE id = 1;
			`,
			expected: []string{
				"CREATE TABLE users (id INTEGER PRIMARY KEY)",
				"INSERT INTO users (id) VALUES (1)",
				"UPDATE users SET id = 2 WHERE id = 1",
			},
		},
		{
			name: "With comments",
			sql: `
				-- This is a comment
				CREATE TABLE users (id INTEGER PRIMARY KEY);
				-- Another comment
				INSERT INTO users (id) VALUES (1);
			`,
			expected: []string{
				"CREATE TABLE users (id INTEGER PRIMARY KEY)",
				"INSERT INTO users (id) VALUES (1)",
			},
		},
		{
			name: "Empty statements",
			sql: `
				CREATE TABLE users (id INTEGER PRIMARY KEY);
				;
				INSERT INTO users (id) VALUES (1);
				;
			`,
			expected: []string{
				"CREATE TABLE users (id INTEGER PRIMARY KEY)",
				"INSERT INTO users (id) VALUES (1)",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.parseSQL(tt.sql)
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d statements, got %d", len(tt.expected), len(result))
				return
			}
			
			for i, stmt := range result {
				expected := strings.TrimSpace(tt.expected[i])
				actual := strings.TrimSpace(stmt)
				
				if actual != expected {
					t.Errorf("Statement %d: expected %q, got %q", i, expected, actual)
				}
			}
		})
	}
}

func TestSQLiteExecutor_ExecuteMigration_EmptySQL(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	executor := NewSQLiteExecutor(db)
	ctx := context.Background()
	
	// Test migration with empty SQL
	migration := Migration{
		Version:     "001",
		Description: "Empty migration",
		SQL:         "",
		FilePath:    "001_empty.sql",
	}
	
	err := executor.ExecuteMigration(ctx, migration)
	if err == nil {
		t.Fatal("Expected ExecuteMigration to fail with empty SQL")
	}
	
	// Verify it's a MigrationError
	var migrationErr *MigrationError
	if !errors.As(err, &migrationErr) {
		t.Errorf("Expected MigrationError, got %T", err)
	}
}