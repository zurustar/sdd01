package migration

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileScanner_ScanMigrations(t *testing.T) {
	tests := []struct {
		name          string
		setupFiles    map[string]string // filename -> content
		expectedCount int
		expectedOrder []string // expected version order
		expectError   bool
		errorContains string
	}{
		{
			name: "valid migration directory with multiple files",
			setupFiles: map[string]string{
				"001_initial_schema.sql": "CREATE TABLE users (id TEXT PRIMARY KEY);",
				"002_add_rooms.sql":      "CREATE TABLE rooms (id TEXT PRIMARY KEY);",
				"003_add_indexes.sql":    "CREATE INDEX idx_users_email ON users(email);",
			},
			expectedCount: 3,
			expectedOrder: []string{"001", "002", "003"},
			expectError:   false,
		},
		{
			name: "single migration file",
			setupFiles: map[string]string{
				"001_initial_schema.sql": "CREATE TABLE users (id TEXT PRIMARY KEY);",
			},
			expectedCount: 1,
			expectedOrder: []string{"001"},
			expectError:   false,
		},
		{
			name: "empty migration directory",
			setupFiles: map[string]string{
				// No migration files, but create directory
			},
			expectedCount: 0,
			expectedOrder: []string{},
			expectError:   false,
		},
		{
			name: "directory with non-SQL files (should be ignored)",
			setupFiles: map[string]string{
				"001_initial_schema.sql": "CREATE TABLE users (id TEXT PRIMARY KEY);",
				"README.md":              "# Migration README",
				"config.json":            `{"version": 1}`,
				"002_add_rooms.sql":      "CREATE TABLE rooms (id TEXT PRIMARY KEY);",
			},
			expectedCount: 2,
			expectedOrder: []string{"001", "002"},
			expectError:   false,
		},
		{
			name: "migrations with non-sequential versions (should sort correctly)",
			setupFiles: map[string]string{
				"005_add_indexes.sql":    "CREATE INDEX idx_users_email ON users(email);",
				"001_initial_schema.sql": "CREATE TABLE users (id TEXT PRIMARY KEY);",
				"003_add_rooms.sql":      "CREATE TABLE rooms (id TEXT PRIMARY KEY);",
			},
			expectedCount: 3,
			expectedOrder: []string{"001", "003", "005"},
			expectError:   false,
		},
		{
			name: "invalid filename format",
			setupFiles: map[string]string{
				"001_initial_schema.sql": "CREATE TABLE users (id TEXT PRIMARY KEY);",
				"invalid_name.sql":       "CREATE TABLE test (id TEXT);",
			},
			expectError:   true,
			errorContains: "does not match pattern",
		},
		{
			name: "duplicate version numbers",
			setupFiles: map[string]string{
				"001_initial_schema.sql": "CREATE TABLE users (id TEXT PRIMARY KEY);",
				"001_duplicate.sql":      "CREATE TABLE rooms (id TEXT PRIMARY KEY);",
			},
			expectError:   true,
			errorContains: "duplicate migration version",
		},
		{
			name: "empty SQL file",
			setupFiles: map[string]string{
				"001_initial_schema.sql": "",
			},
			expectError:   true,
			errorContains: "migration file is empty",
		},
		{
			name: "SQL file with only whitespace",
			setupFiles: map[string]string{
				"001_initial_schema.sql": "   \n\t  \n  ",
			},
			expectError:   true,
			errorContains: "migration file is empty",
		},
		{
			name: "SQL file with only comments",
			setupFiles: map[string]string{
				"001_initial_schema.sql": "-- This is just a comment\n-- Another comment",
			},
			expectError:   true,
			errorContains: "no SQL statements found",
		},
		{
			name: "SQL with unmatched parentheses",
			setupFiles: map[string]string{
				"001_initial_schema.sql": "CREATE TABLE users (id TEXT PRIMARY KEY;", // missing closing parenthesis
			},
			expectError:   true,
			errorContains: "unmatched opening parenthesis",
		},
		{
			name: "SQL with unterminated string",
			setupFiles: map[string]string{
				"001_initial_schema.sql": "SELECT 'unterminated string",
			},
			expectError:   true,
			errorContains: "unterminated string literal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tempDir := t.TempDir()
			
			// Create test files
			for filename, content := range tt.setupFiles {
				filePath := filepath.Join(tempDir, filename)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}
			
			// Create file scanner
			scanner := NewFileScanner()
			
			// Execute ScanMigrations
			migrations, err := scanner.ScanMigrations(tempDir)
			
			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', but got: %v", tt.errorContains, err)
				}
				return
			}
			
			// Check for unexpected errors
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Check migration count
			if len(migrations) != tt.expectedCount {
				t.Errorf("Expected %d migrations, got %d", tt.expectedCount, len(migrations))
			}
			
			// Check migration order
			if len(migrations) > 0 && len(tt.expectedOrder) > 0 {
				for i, expectedVersion := range tt.expectedOrder {
					if i >= len(migrations) {
						t.Errorf("Expected migration %d with version %s, but only got %d migrations", 
							i, expectedVersion, len(migrations))
						break
					}
					if migrations[i].Version != expectedVersion {
						t.Errorf("Expected migration %d to have version %s, got %s", 
							i, expectedVersion, migrations[i].Version)
					}
				}
			}
			
			// Verify migration properties
			for _, migration := range migrations {
				if migration.Version == "" {
					t.Errorf("Migration has empty version")
				}
				if migration.Description == "" {
					t.Errorf("Migration %s has empty description", migration.Version)
				}
				if migration.SQL == "" {
					t.Errorf("Migration %s has empty SQL", migration.Version)
				}
				if migration.FilePath == "" {
					t.Errorf("Migration %s has empty file path", migration.Version)
				}
				if migration.Checksum == "" {
					t.Errorf("Migration %s has empty checksum", migration.Version)
				}
			}
		})
	}
}

func TestFileScanner_ScanMigrations_NonExistentDirectory(t *testing.T) {
	scanner := NewFileScanner()
	
	// Try to scan a non-existent directory
	_, err := scanner.ScanMigrations("/non/existent/directory")
	
	if err == nil {
		t.Errorf("Expected error for non-existent directory, but got none")
		return
	}
	
	// Check that it's a FileSystemError
	var fsErr *FileSystemError
	if !errors.As(err, &fsErr) {
		t.Errorf("Expected FileSystemError, got %T: %v", err, err)
	}
	
	if !strings.Contains(err.Error(), "migration directory does not exist") {
		t.Errorf("Expected error about non-existent directory, got: %v", err)
	}
}

func TestFileScanner_ValidateFileName(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid filename with numeric version",
			filename:    "001_initial_schema.sql",
			expectError: false,
		},
		{
			name:        "valid filename with multi-digit version",
			filename:    "123_add_user_roles.sql",
			expectError: false,
		},
		{
			name:        "valid filename with underscores in description",
			filename:    "002_add_user_session_table.sql",
			expectError: false,
		},
		{
			name:        "valid filename with hyphens in description",
			filename:    "003_add-user-indexes.sql",
			expectError: false,
		},
		{
			name:        "valid filename with mixed characters",
			filename:    "004_add_user_roles_and_permissions.sql",
			expectError: false,
		},
		{
			name:          "invalid filename without version",
			filename:      "initial_schema.sql",
			expectError:   true,
			errorContains: "does not match pattern",
		},
		{
			name:          "invalid filename without description",
			filename:      "001_.sql",
			expectError:   true,
			errorContains: "does not match pattern",
		},
		{
			name:          "invalid filename without extension",
			filename:      "001_initial_schema",
			expectError:   true,
			errorContains: "does not match pattern",
		},
		{
			name:          "invalid filename with wrong extension",
			filename:      "001_initial_schema.txt",
			expectError:   true,
			errorContains: "does not match pattern",
		},
		{
			name:          "invalid filename with non-numeric version",
			filename:      "abc_initial_schema.sql",
			expectError:   true,
			errorContains: "does not match pattern",
		},
		{
			name:          "invalid filename with special characters in description",
			filename:      "001_initial@schema.sql",
			expectError:   true,
			errorContains: "does not match pattern",
		},
		{
			name:          "invalid filename with spaces",
			filename:      "001_initial schema.sql",
			expectError:   true,
			errorContains: "does not match pattern",
		},
		{
			name:          "empty filename",
			filename:      "",
			expectError:   true,
			errorContains: "does not match pattern",
		},
		{
			name:          "filename with leading zeros (should be valid)",
			filename:      "001_initial_schema.sql",
			expectError:   false,
		},
		{
			name:          "filename with version zero",
			filename:      "000_initial_schema.sql",
			expectError:   false,
		},
	}

	scanner := NewFileScanner()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scanner.ValidateFileName(tt.filename)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for filename '%s', but got none", tt.filename)
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', but got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for filename '%s': %v", tt.filename, err)
				}
			}
		})
	}
}

func TestFileScanner_ParseMigrationFile(t *testing.T) {
	tests := []struct {
		name               string
		filename           string
		content            string
		expectedVersion    string
		expectedDesc       string
		expectError        bool
		errorContains      string
	}{
		{
			name:            "valid migration file",
			filename:        "001_initial_schema.sql",
			content:         "CREATE TABLE users (id TEXT PRIMARY KEY);",
			expectedVersion: "001",
			expectedDesc:    "initial schema", // underscores converted to spaces
			expectError:     false,
		},
		{
			name:            "migration with comments",
			filename:        "002_add_rooms.sql",
			content:         "-- Create rooms table\nCREATE TABLE rooms (id TEXT PRIMARY KEY);",
			expectedVersion: "002",
			expectedDesc:    "add rooms",
			expectError:     false,
		},
		{
			name:            "migration with multiple statements",
			filename:        "003_add_indexes.sql",
			content:         "CREATE INDEX idx_users_email ON users(email);\nCREATE INDEX idx_rooms_name ON rooms(name);",
			expectedVersion: "003",
			expectedDesc:    "add indexes",
			expectError:     false,
		},
		{
			name:          "empty file",
			filename:      "004_empty.sql",
			content:       "",
			expectError:   true,
			errorContains: "migration file is empty",
		},
		{
			name:          "file with only whitespace",
			filename:      "005_whitespace.sql",
			content:       "   \n\t  \n  ",
			expectError:   true,
			errorContains: "migration file is empty",
		},
		{
			name:          "file with only comments",
			filename:      "006_comments_only.sql",
			content:       "-- Just a comment\n-- Another comment",
			expectError:   true,
			errorContains: "no SQL statements found",
		},
		{
			name:          "invalid filename format",
			filename:      "invalid_name.sql",
			content:       "CREATE TABLE test (id TEXT);",
			expectError:   true,
			errorContains: "does not match pattern",
		},
		{
			name:            "migration with complex SQL",
			filename:        "007_complex_migration.sql",
			content:         `CREATE TABLE complex_table (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
			INSERT INTO complex_table (id, name) VALUES ('1', 'test');`,
			expectedVersion: "007",
			expectedDesc:    "complex migration",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, tt.filename)
			
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			
			scanner := NewFileScanner()
			
			// Parse the migration file
			migration, err := scanner.ParseMigrationFile(filePath)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', but got: %v", tt.errorContains, err)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Verify migration properties
			if migration.Version != tt.expectedVersion {
				t.Errorf("Expected version %s, got %s", tt.expectedVersion, migration.Version)
			}
			
			if migration.Description != tt.expectedDesc {
				t.Errorf("Expected description '%s', got '%s'", tt.expectedDesc, migration.Description)
			}
			
			if migration.SQL != tt.content {
				t.Errorf("Expected SQL content to match original")
			}
			
			if migration.FilePath != filePath {
				t.Errorf("Expected file path %s, got %s", filePath, migration.FilePath)
			}
			
			if migration.Checksum == "" {
				t.Errorf("Expected non-empty checksum")
			}
		})
	}
}

func TestFileScanner_VersionNumberParsing(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedOrder   []string
	}{
		{
			name: "sequential versions",
			files: map[string]string{
				"001_first.sql":  "CREATE TABLE first (id TEXT);",
				"002_second.sql": "CREATE TABLE second (id TEXT);",
				"003_third.sql":  "CREATE TABLE third (id TEXT);",
			},
			expectedOrder: []string{"001", "002", "003"},
		},
		{
			name: "non-sequential versions",
			files: map[string]string{
				"001_first.sql": "CREATE TABLE first (id TEXT);",
				"005_fifth.sql": "CREATE TABLE fifth (id TEXT);",
				"003_third.sql": "CREATE TABLE third (id TEXT);",
			},
			expectedOrder: []string{"001", "003", "005"},
		},
		{
			name: "mixed digit lengths",
			files: map[string]string{
				"1_first.sql":   "CREATE TABLE first (id TEXT);",
				"10_tenth.sql":  "CREATE TABLE tenth (id TEXT);",
				"2_second.sql":  "CREATE TABLE second (id TEXT);",
				"100_hundred.sql": "CREATE TABLE hundred (id TEXT);",
			},
			expectedOrder: []string{"1", "2", "10", "100"},
		},
		{
			name: "versions with leading zeros",
			files: map[string]string{
				"001_first.sql":  "CREATE TABLE first (id TEXT);",
				"010_tenth.sql":  "CREATE TABLE tenth (id TEXT);",
				"002_second.sql": "CREATE TABLE second (id TEXT);",
			},
			expectedOrder: []string{"001", "002", "010"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			
			// Create test files
			for filename, content := range tt.files {
				filePath := filepath.Join(tempDir, filename)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}
			
			scanner := NewFileScanner()
			migrations, err := scanner.ScanMigrations(tempDir)
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if len(migrations) != len(tt.expectedOrder) {
				t.Errorf("Expected %d migrations, got %d", len(tt.expectedOrder), len(migrations))
				return
			}
			
			// Verify order
			for i, expectedVersion := range tt.expectedOrder {
				if migrations[i].Version != expectedVersion {
					t.Errorf("Expected migration %d to have version %s, got %s", 
						i, expectedVersion, migrations[i].Version)
				}
			}
		})
	}
}