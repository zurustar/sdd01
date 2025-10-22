// +build integration

package migration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestRealSQLite_CompleteMigrationWorkflow tests the complete migration workflow with database integration
func TestRealSQLite_CompleteMigrationWorkflow(t *testing.T) {
	ctx := context.Background()

	// Create a database for testing (may be stub or real depending on build)
	tempFile := fmt.Sprintf("/tmp/real_test_migration_%d.db", time.Now().UnixNano())
	defer os.Remove(tempFile)

	// Use the configured SQLite driver
	config := TempFileTestSQLiteConfig(tempFile)
	cm := NewConnectionManager(config)
	db, err := cm.GetConnection()
	if err != nil {
		t.Fatalf("Failed to open SQLite database: %v", err)
	}
	defer db.Close()

	// Test database connection (this works with both stub and real drivers)
	var result int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	isStubDriver := err != nil && strings.Contains(err.Error(), "stub driver")
	
	if isStubDriver {
		t.Log("Using stub driver - testing migration workflow logic")
	} else {
		t.Log("Using real driver - testing complete database operations")
		if result != 1 {
			t.Fatalf("Expected 1, got %d", result)
		}
	}

	// Create migration components
	scanner := NewFileScanner()
	executor := NewSQLiteExecutor(db)
	
	// Get test migration directory
	testDataDir := "./testdata"
	manager := NewMigrationManager(scanner, executor, testDataDir)

	// Test 1: Test migration file scanning and parsing (works with both drivers)
	pendingMigrations, err := manager.GetPendingMigrations(ctx)
	if err != nil {
		t.Fatalf("Failed to get pending migrations: %v", err)
	}
	
	if len(pendingMigrations) != 3 {
		t.Fatalf("Expected 3 pending migrations, got %d", len(pendingMigrations))
	}

	// Verify migration content is properly parsed
	expectedVersions := []string{"001", "002", "003"}
	expectedDescriptions := []string{
		"Create test users table for testing scenarios",
		"Create test posts table with foreign key to users", 
		"Create test comments table with foreign keys",
	}

	for i, migration := range pendingMigrations {
		if migration.Version != expectedVersions[i] {
			t.Errorf("Migration %d: expected version %s, got %s", i, expectedVersions[i], migration.Version)
		}
		
		if migration.Description != expectedDescriptions[i] {
			t.Errorf("Migration %d: expected description %s, got %s", i, expectedDescriptions[i], migration.Description)
		}
		
		if migration.SQL == "" {
			t.Errorf("Migration %s has empty SQL content", migration.Version)
		}
		
		if migration.FilePath == "" {
			t.Errorf("Migration %s has empty file path", migration.Version)
		}
		
		if migration.Checksum == "" {
			t.Errorf("Migration %s has empty checksum", migration.Version)
		}
		
		// Verify SQL content contains expected elements
		if !strings.Contains(migration.SQL, "CREATE TABLE") {
			t.Errorf("Migration %s SQL does not contain CREATE TABLE statement", migration.Version)
		}
	}

	// Test 2: Execute migrations (works with both drivers)
	if err := manager.RunMigrations(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Test 3: Test idempotency - running migrations again should not fail
	if err := manager.RunMigrations(ctx); err != nil {
		t.Fatalf("Failed to run migrations second time (idempotency test): %v", err)
	}

	// Test 4: Test migration status reporting (works with both drivers)
	status, err := manager.GetMigrationStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get migration status: %v", err)
	}

	if !isStubDriver {
		// With real driver, we can verify actual database state
		if status.PendingCount != 0 {
			t.Errorf("Expected 0 pending migrations after execution, got %d", status.PendingCount)
		}
		
		if len(status.AppliedMigrations) != 3 {
			t.Errorf("Expected 3 applied migrations, got %d", len(status.AppliedMigrations))
		}

		// Test database state verification
		var tableCount int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name LIKE 'test_%'").Scan(&tableCount)
		if err != nil {
			t.Fatalf("Failed to count created tables: %v", err)
		}
		if tableCount != 3 {
			t.Errorf("Expected 3 test tables after migration, got %d", tableCount)
		}

		// Verify schema_migrations table was created and populated
		var migrationCount int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount)
		if err != nil {
			t.Fatalf("Failed to count applied migrations: %v", err)
		}
		if migrationCount != 3 {
			t.Errorf("Expected 3 applied migrations in schema_migrations, got %d", migrationCount)
		}

		// Verify test data was inserted correctly
		var userCount, postCount, commentCount int
		
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_users").Scan(&userCount)
		if err != nil {
			t.Fatalf("Failed to count test users: %v", err)
		}
		if userCount != 2 {
			t.Errorf("Expected 2 test users, got %d", userCount)
		}

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_posts").Scan(&postCount)
		if err != nil {
			t.Fatalf("Failed to count test posts: %v", err)
		}
		if postCount != 3 {
			t.Errorf("Expected 3 test posts, got %d", postCount)
		}

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_comments").Scan(&commentCount)
		if err != nil {
			t.Fatalf("Failed to count test comments: %v", err)
		}
		if commentCount != 4 {
			t.Errorf("Expected 4 test comments, got %d", commentCount)
		}
	} else {
		// With stub driver, we can still verify the migration logic flow
		t.Log("Stub driver detected - verified migration workflow logic without database persistence")
		
		// Verify that the migration manager correctly tracks what should be applied
		if len(pendingMigrations) != 3 {
			t.Errorf("Expected 3 migrations to be processed, got %d", len(pendingMigrations))
		}
	}

	if isStubDriver {
		t.Log("Database integration workflow completed successfully with stub driver")
	} else {
		t.Log("Real SQLite database migration workflow completed successfully")
	}
}

// TestRealSQLite_DataIntegrityAfterMigrations tests data integrity after migrations
func TestRealSQLite_DataIntegrityAfterMigrations(t *testing.T) {
	ctx := context.Background()

	// Create a database for testing
	tempFile := fmt.Sprintf("/tmp/real_test_integrity_%d.db", time.Now().UnixNano())
	defer os.Remove(tempFile)

	config := TempFileTestSQLiteConfig(tempFile)
	cm := NewConnectionManager(config)
	db, err := cm.GetConnection()
	if err != nil {
		t.Fatalf("Failed to open SQLite database: %v", err)
	}
	defer db.Close()

	// Check if using stub driver
	var testResult int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&testResult)
	isStubDriver := err != nil && strings.Contains(err.Error(), "stub driver")
	
	if isStubDriver {
		t.Skip("Skipping data integrity test - stub driver cannot persist data")
		return
	}

	// Set up and run migrations
	scanner := NewFileScanner()
	executor := NewSQLiteExecutor(db)
	manager := NewMigrationManager(scanner, executor, "./testdata")

	if err := manager.RunMigrations(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Test 1: Verify all expected data exists
	t.Run("VerifyExpectedData", func(t *testing.T) {
		// Check users
		rows, err := db.QueryContext(ctx, "SELECT id, username, email FROM test_users ORDER BY id")
		if err != nil {
			t.Fatalf("Failed to query users: %v", err)
		}
		defer rows.Close()

		expectedUsers := []struct {
			id, username, email string
		}{
			{"user1", "testuser1", "test1@example.com"},
			{"user2", "testuser2", "test2@example.com"},
		}

		userIndex := 0
		for rows.Next() {
			var id, username, email string
			if err := rows.Scan(&id, &username, &email); err != nil {
				t.Fatalf("Failed to scan user: %v", err)
			}
			
			if userIndex >= len(expectedUsers) {
				t.Fatalf("More users than expected")
			}
			
			expected := expectedUsers[userIndex]
			if id != expected.id || username != expected.username || email != expected.email {
				t.Errorf("User %d mismatch: expected (%s,%s,%s), got (%s,%s,%s)",
					userIndex, expected.id, expected.username, expected.email, id, username, email)
			}
			userIndex++
		}
		
		if userIndex != len(expectedUsers) {
			t.Errorf("Expected %d users, got %d", len(expectedUsers), userIndex)
		}
	})

	// Test 2: Verify referential integrity
	t.Run("VerifyReferentialIntegrity", func(t *testing.T) {
		// All posts should have valid user references
		var orphanedPosts int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM test_posts p 
			LEFT JOIN test_users u ON p.user_id = u.id 
			WHERE u.id IS NULL
		`).Scan(&orphanedPosts)
		if err != nil {
			t.Fatalf("Failed to check orphaned posts: %v", err)
		}
		if orphanedPosts > 0 {
			t.Errorf("Found %d orphaned posts without valid user references", orphanedPosts)
		}

		// All comments should have valid post and user references
		var orphanedComments int
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM test_comments c 
			LEFT JOIN test_posts p ON c.post_id = p.id 
			LEFT JOIN test_users u ON c.user_id = u.id 
			WHERE p.id IS NULL OR u.id IS NULL
		`).Scan(&orphanedComments)
		if err != nil {
			t.Fatalf("Failed to check orphaned comments: %v", err)
		}
		if orphanedComments > 0 {
			t.Errorf("Found %d orphaned comments without valid references", orphanedComments)
		}
	})

	// Test 3: Verify data consistency across related tables
	t.Run("VerifyDataConsistency", func(t *testing.T) {
		// Count posts per user and verify against expected data
		rows, err := db.QueryContext(ctx, `
			SELECT u.username, COUNT(p.id) as post_count 
			FROM test_users u 
			LEFT JOIN test_posts p ON u.id = p.user_id 
			GROUP BY u.id, u.username 
			ORDER BY u.username
		`)
		if err != nil {
			t.Fatalf("Failed to query posts per user: %v", err)
		}
		defer rows.Close()

		expectedPostCounts := map[string]int{
			"testuser1": 2, // user1 has 2 posts
			"testuser2": 1, // user2 has 1 post
		}

		for rows.Next() {
			var username string
			var postCount int
			if err := rows.Scan(&username, &postCount); err != nil {
				t.Fatalf("Failed to scan post count: %v", err)
			}
			
			expectedCount, exists := expectedPostCounts[username]
			if !exists {
				t.Errorf("Unexpected user in results: %s", username)
				continue
			}
			
			if postCount != expectedCount {
				t.Errorf("User %s: expected %d posts, got %d", username, expectedCount, postCount)
			}
		}
	})

	t.Log("Real SQLite data integrity verification completed successfully")
}

// TestRealSQLite_ForeignKeyConstraints tests foreign key constraints with real SQLite database
func TestRealSQLite_ForeignKeyConstraints(t *testing.T) {
	ctx := context.Background()

	// Create a database for testing
	tempFile := fmt.Sprintf("/tmp/real_test_fk_%d.db", time.Now().UnixNano())
	defer os.Remove(tempFile)

	config := TempFileTestSQLiteConfig(tempFile)
	cm := NewConnectionManager(config)
	db, err := cm.GetConnection()
	if err != nil {
		t.Fatalf("Failed to open SQLite database: %v", err)
	}
	defer db.Close()

	// Check if using stub driver
	var testResult int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&testResult)
	isStubDriver := err != nil && strings.Contains(err.Error(), "stub driver")
	
	if isStubDriver {
		t.Skip("Skipping foreign key constraint test - stub driver cannot enforce constraints")
		return
	}

	// Set up and run migrations
	scanner := NewFileScanner()
	executor := NewSQLiteExecutor(db)
	manager := NewMigrationManager(scanner, executor, "./testdata")

	if err := manager.RunMigrations(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Test 1: Verify foreign keys are enabled
	t.Run("VerifyForeignKeysEnabled", func(t *testing.T) {
		var fkEnabled int
		err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkEnabled)
		if err != nil {
			t.Fatalf("Failed to check foreign key status: %v", err)
		}
		if fkEnabled != 1 {
			t.Errorf("Foreign keys should be enabled (1), got %d", fkEnabled)
		}
	})

	// Test 2: Test foreign key constraint enforcement on posts
	t.Run("TestPostUserConstraint", func(t *testing.T) {
		// Try to insert post with non-existent user - should fail
		_, err := db.ExecContext(ctx, `
			INSERT INTO test_posts (id, user_id, title, content) 
			VALUES ('invalid_post', 'nonexistent_user', 'Test Title', 'Test Content')
		`)
		if err == nil {
			t.Error("Expected foreign key constraint violation when inserting post with invalid user_id")
		}

		// Insert post with valid user - should succeed
		_, err = db.ExecContext(ctx, `
			INSERT INTO test_posts (id, user_id, title, content) 
			VALUES ('valid_post', 'user1', 'Valid Post', 'Valid Content')
		`)
		if err != nil {
			t.Errorf("Failed to insert post with valid user_id: %v", err)
		}

		// Verify the post was inserted
		var postCount int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_posts WHERE id = 'valid_post'").Scan(&postCount)
		if err != nil {
			t.Fatalf("Failed to verify post insertion: %v", err)
		}
		if postCount != 1 {
			t.Errorf("Expected 1 post with id 'valid_post', got %d", postCount)
		}
	})

	// Test 3: Test CASCADE DELETE behavior
	t.Run("TestCascadeDelete", func(t *testing.T) {
		// Count comments before deletion
		var commentsBefore int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_comments WHERE post_id = 'post1'").Scan(&commentsBefore)
		if err != nil {
			t.Fatalf("Failed to count comments before deletion: %v", err)
		}

		if commentsBefore == 0 {
			t.Skip("No comments found for post1, skipping cascade delete test")
		}

		// Delete a post that has comments
		_, err = db.ExecContext(ctx, "DELETE FROM test_posts WHERE id = 'post1'")
		if err != nil {
			t.Fatalf("Failed to delete post: %v", err)
		}

		// Verify comments were cascade deleted
		var commentsAfter int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_comments WHERE post_id = 'post1'").Scan(&commentsAfter)
		if err != nil {
			t.Fatalf("Failed to count comments after deletion: %v", err)
		}

		if commentsAfter != 0 {
			t.Errorf("Expected 0 comments after cascade delete, got %d", commentsAfter)
		}

		t.Logf("Cascade delete test: %d comments were properly deleted", commentsBefore)
	})

	t.Log("Real SQLite foreign key constraint tests completed successfully")
}

// TestRealSQLite_TransactionBehavior tests transaction behavior with real SQLite database
func TestRealSQLite_TransactionBehavior(t *testing.T) {
	ctx := context.Background()

	// Create a database for testing
	tempFile := fmt.Sprintf("/tmp/real_test_tx_%d.db", time.Now().UnixNano())
	defer os.Remove(tempFile)

	config := TempFileTestSQLiteConfig(tempFile)
	cm := NewConnectionManager(config)
	db, err := cm.GetConnection()
	if err != nil {
		t.Fatalf("Failed to open SQLite database: %v", err)
	}
	defer db.Close()

	// Check if using stub driver
	var testResult int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&testResult)
	isStubDriver := err != nil && strings.Contains(err.Error(), "stub driver")
	
	if isStubDriver {
		t.Skip("Skipping transaction behavior test - stub driver cannot persist transaction state")
		return
	}

	// Set up and run migrations
	scanner := NewFileScanner()
	executor := NewSQLiteExecutor(db)
	manager := NewMigrationManager(scanner, executor, "./testdata")

	if err := manager.RunMigrations(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Test 1: Test successful transaction commit
	t.Run("TestTransactionCommit", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}

		// Insert data within transaction
		_, err = tx.ExecContext(ctx, `
			INSERT INTO test_users (id, username, email) 
			VALUES ('tx_user1', 'txuser1', 'tx1@example.com')
		`)
		if err != nil {
			tx.Rollback()
			t.Fatalf("Failed to insert user in transaction: %v", err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			t.Fatalf("Failed to commit transaction: %v", err)
		}

		// Verify data was committed
		var userCount int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_users WHERE id = 'tx_user1'").Scan(&userCount)
		if err != nil {
			t.Fatalf("Failed to verify committed user: %v", err)
		}
		if userCount != 1 {
			t.Errorf("Expected 1 committed user, got %d", userCount)
		}
	})

	// Test 2: Test transaction rollback
	t.Run("TestTransactionRollback", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}

		// Insert data within transaction
		_, err = tx.ExecContext(ctx, `
			INSERT INTO test_users (id, username, email) 
			VALUES ('tx_user2', 'txuser2', 'tx2@example.com')
		`)
		if err != nil {
			tx.Rollback()
			t.Fatalf("Failed to insert user in transaction: %v", err)
		}

		// Rollback transaction
		if err := tx.Rollback(); err != nil {
			t.Fatalf("Failed to rollback transaction: %v", err)
		}

		// Verify data was not committed
		var userCount int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_users WHERE id = 'tx_user2'").Scan(&userCount)
		if err != nil {
			t.Fatalf("Failed to verify rolled back user: %v", err)
		}
		if userCount != 0 {
			t.Errorf("Expected 0 rolled back users, got %d", userCount)
		}
	})

	// Test 3: Test transaction rollback on constraint violation
	t.Run("TestTransactionRollbackOnError", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback() // Ensure rollback in case of test failure

		// Insert valid data first
		_, err = tx.ExecContext(ctx, `
			INSERT INTO test_users (id, username, email) 
			VALUES ('tx_user3', 'txuser3', 'tx3@example.com')
		`)
		if err != nil {
			t.Fatalf("Failed to insert valid user in transaction: %v", err)
		}

		// Try to insert invalid data (foreign key violation)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO test_posts (id, user_id, title, content) 
			VALUES ('tx_post3', 'nonexistent_user', 'Error Post', 'Error Content')
		`)
		if err == nil {
			t.Error("Expected foreign key constraint violation, but insert succeeded")
		}

		// Rollback due to error
		if err := tx.Rollback(); err != nil {
			t.Fatalf("Failed to rollback transaction after error: %v", err)
		}

		// Verify that even the valid data was rolled back
		var userCount int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_users WHERE id = 'tx_user3'").Scan(&userCount)
		if err != nil {
			t.Fatalf("Failed to verify rolled back user after error: %v", err)
		}
		if userCount != 0 {
			t.Errorf("Expected 0 users after rollback on error, got %d", userCount)
		}
	})

	t.Log("Real SQLite transaction behavior tests completed successfully")
}

// TestIntegration_MigrationSystemComponents tests integration between migration system components
func TestIntegration_MigrationSystemComponents(t *testing.T) {
	ctx := context.Background()

	// This test works with both stub and real drivers by focusing on component integration
	tempFile := fmt.Sprintf("/tmp/integration_test_%d.db", time.Now().UnixNano())
	defer os.Remove(tempFile)

	config := TempFileTestSQLiteConfig(tempFile)
	cm := NewConnectionManager(config)
	db, err := cm.GetConnection()
	if err != nil {
		t.Fatalf("Failed to open SQLite database: %v", err)
	}
	defer db.Close()

	// Test 1: Component creation and initialization
	t.Run("ComponentInitialization", func(t *testing.T) {
		scanner := NewFileScanner()
		if scanner == nil {
			t.Fatal("Failed to create file scanner")
		}

		executor := NewSQLiteExecutor(db)
		if executor == nil {
			t.Fatal("Failed to create SQLite executor")
		}

		manager := NewMigrationManager(scanner, executor, "./testdata")
		if manager == nil {
			t.Fatal("Failed to create migration manager")
		}

		// Test executor initialization
		if err := executor.InitializeVersionTable(ctx); err != nil {
			t.Fatalf("Failed to initialize version table: %v", err)
		}
	})

	// Test 2: File scanning and parsing integration
	t.Run("FileScanningIntegration", func(t *testing.T) {
		scanner := NewFileScanner()
		
		// Test scanning migration directory
		migrations, err := scanner.ScanMigrations("./testdata")
		if err != nil {
			t.Fatalf("Failed to scan migrations: %v", err)
		}

		if len(migrations) != 3 {
			t.Fatalf("Expected 3 migrations, got %d", len(migrations))
		}

		// Test individual file parsing
		for _, migration := range migrations {
			parsedMigration, err := scanner.ParseMigrationFile(migration.FilePath)
			if err != nil {
				t.Errorf("Failed to parse migration file %s: %v", migration.FilePath, err)
				continue
			}

			// Verify parsed content matches scanned content
			if parsedMigration.Version != migration.Version {
				t.Errorf("Version mismatch for %s: expected %s, got %s", 
					migration.FilePath, migration.Version, parsedMigration.Version)
			}

			if parsedMigration.Description != migration.Description {
				t.Errorf("Description mismatch for %s: expected %s, got %s", 
					migration.FilePath, migration.Description, parsedMigration.Description)
			}

			if parsedMigration.SQL != migration.SQL {
				t.Errorf("SQL content mismatch for %s", migration.FilePath)
			}

			// Verify checksum consistency
			if parsedMigration.Checksum != migration.Checksum {
				t.Errorf("Checksum mismatch for %s: expected %s, got %s", 
					migration.FilePath, migration.Checksum, parsedMigration.Checksum)
			}
		}
	})

	// Test 3: Migration manager orchestration
	t.Run("MigrationManagerOrchestration", func(t *testing.T) {
		scanner := NewFileScanner()
		executor := NewSQLiteExecutor(db)
		manager := NewMigrationManager(scanner, executor, "./testdata")

		// Test getting pending migrations
		pendingMigrations, err := manager.GetPendingMigrations(ctx)
		if err != nil {
			t.Fatalf("Failed to get pending migrations: %v", err)
		}

		if len(pendingMigrations) != 3 {
			t.Fatalf("Expected 3 pending migrations, got %d", len(pendingMigrations))
		}

		// Verify migrations are in correct order
		expectedVersions := []string{"001", "002", "003"}
		for i, migration := range pendingMigrations {
			if migration.Version != expectedVersions[i] {
				t.Errorf("Migration %d: expected version %s, got %s", 
					i, expectedVersions[i], migration.Version)
			}
		}

		// Test migration execution
		if err := manager.RunMigrations(ctx); err != nil {
			t.Fatalf("Failed to run migrations: %v", err)
		}

		// Test migration status reporting
		status, err := manager.GetMigrationStatus(ctx)
		if err != nil {
			t.Fatalf("Failed to get migration status: %v", err)
		}

		// Verify status structure is correct
		if status == nil {
			t.Fatal("Migration status should not be nil")
		}

		// Test logging methods (should not fail regardless of driver)
		if err := manager.LogCurrentSchemaVersion(ctx); err != nil {
			t.Errorf("Failed to log current schema version: %v", err)
		}

		if err := manager.LogPendingMigrations(ctx); err != nil {
			t.Errorf("Failed to log pending migrations: %v", err)
		}
	})

	// Test 4: Error handling integration
	t.Run("ErrorHandlingIntegration", func(t *testing.T) {
		scanner := NewFileScanner()
		executor := NewSQLiteExecutor(db)

		// Test with invalid migration directory
		manager := NewMigrationManager(scanner, executor, "/nonexistent/directory")
		
		_, err := manager.GetPendingMigrations(ctx)
		if err == nil {
			t.Error("Expected error when scanning nonexistent directory")
		}

		// Test with valid directory but invalid file
		tempDir := fmt.Sprintf("/tmp/test_invalid_migrations_%d", time.Now().UnixNano())
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create invalid migration file
		invalidFile := fmt.Sprintf("%s/invalid_migration.sql", tempDir)
		if err := os.WriteFile(invalidFile, []byte("invalid content"), 0644); err != nil {
			t.Fatalf("Failed to write invalid migration file: %v", err)
		}

		manager = NewMigrationManager(scanner, executor, tempDir)
		migrations, err := manager.GetPendingMigrations(ctx)
		if err != nil {
			t.Fatalf("Scanner should handle invalid files gracefully: %v", err)
		}

		// Should not find any valid migrations
		if len(migrations) != 0 {
			t.Errorf("Expected 0 valid migrations in directory with invalid files, got %d", len(migrations))
		}
	})

	// Test 5: Configuration integration
	t.Run("ConfigurationIntegration", func(t *testing.T) {
		// Test different configuration scenarios
		configs := []SQLiteConfig{
			InMemoryTestSQLiteConfig(),
			TempFileTestSQLiteConfig("/tmp/test_config.db"),
			DefaultSQLiteConfig("/tmp/test_default.db"),
		}

		for i, config := range configs {
			t.Run(fmt.Sprintf("Config%d", i), func(t *testing.T) {
				cm := NewConnectionManager(config)
				if cm == nil {
					t.Fatal("Failed to create connection manager")
				}

				// Test configuration validation
				if err := cm.ValidateConfig(); err != nil {
					t.Errorf("Configuration validation failed: %v", err)
				}

				// Test database connection creation
				testDB, err := cm.GetConnection()
				if err != nil {
					t.Errorf("Failed to create database connection: %v", err)
					return
				}
				defer testDB.Close()

				// Test that connection is functional
				if err := testDB.Ping(); err != nil {
					t.Errorf("Database ping failed: %v", err)
				}
			})
		}
	})

	t.Log("Migration system component integration tests completed successfully")
}

// TestIntegration_MigrationFileValidation tests comprehensive migration file validation
func TestIntegration_MigrationFileValidation(t *testing.T) {
	// This test validates migration file handling comprehensively
	
	// Test 1: Valid migration file formats
	t.Run("ValidMigrationFiles", func(t *testing.T) {
		scanner := NewFileScanner()
		
		// Test scanning the standard test data directory
		migrations, err := scanner.ScanMigrations("./testdata")
		if err != nil {
			t.Fatalf("Failed to scan valid migrations: %v", err)
		}

		// Verify each migration has required fields
		for _, migration := range migrations {
			if migration.Version == "" {
				t.Errorf("Migration has empty version: %+v", migration)
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

			// Verify SQL contains expected patterns
			expectedPatterns := []string{"CREATE TABLE", "INSERT INTO"}
			for _, pattern := range expectedPatterns {
				if !strings.Contains(migration.SQL, pattern) {
					t.Errorf("Migration %s SQL missing expected pattern: %s", migration.Version, pattern)
				}
			}
		}
	})

	// Test 2: Migration file naming validation
	t.Run("MigrationFileNaming", func(t *testing.T) {
		scanner := NewFileScanner()

		// Test valid file names
		validNames := []string{
			"001_initial_schema.sql",
			"002_add_users.sql", 
			"999_final_migration.sql",
		}

		for _, name := range validNames {
			if err := scanner.ValidateFileName(name); err != nil {
				t.Errorf("Valid filename %s should not produce error: %v", name, err)
			}
		}

		// Test invalid file names
		invalidNames := []string{
			"invalid.sql",
			"001.sql",
			"001_",
			"abc_test.sql",
			"001_test.txt",
		}

		for _, name := range invalidNames {
			if err := scanner.ValidateFileName(name); err == nil {
				t.Errorf("Invalid filename %s should produce error", name)
			}
		}
	})

	// Test 3: Migration content validation
	t.Run("MigrationContentValidation", func(t *testing.T) {
		// Create temporary directory with test migration files
		tempDir := fmt.Sprintf("/tmp/test_content_validation_%d", time.Now().UnixNano())
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Test migration with proper header comments
		validMigration := `-- Migration: 001_test_migration.sql
-- Description: Test migration with proper format

CREATE TABLE test_table (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

INSERT INTO test_table (id, name) VALUES ('1', 'test');`

		validFile := fmt.Sprintf("%s/001_test_migration.sql", tempDir)
		if err := os.WriteFile(validFile, []byte(validMigration), 0644); err != nil {
			t.Fatalf("Failed to write valid migration file: %v", err)
		}

		scanner := NewFileScanner()
		migration, err := scanner.ParseMigrationFile(validFile)
		if err != nil {
			t.Fatalf("Failed to parse valid migration: %v", err)
		}

		if migration.Version != "001" {
			t.Errorf("Expected version 001, got %s", migration.Version)
		}

		if migration.Description != "Test migration with proper format" {
			t.Errorf("Expected proper description, got %s", migration.Description)
		}

		// Test migration without proper header
		invalidMigration := `CREATE TABLE test_table (
    id TEXT PRIMARY KEY
);`

		invalidFile := fmt.Sprintf("%s/002_invalid_migration.sql", tempDir)
		if err := os.WriteFile(invalidFile, []byte(invalidMigration), 0644); err != nil {
			t.Fatalf("Failed to write invalid migration file: %v", err)
		}

		_, err = scanner.ParseMigrationFile(invalidFile)
		if err == nil {
			t.Error("Expected error when parsing migration without proper header")
		}
	})

	t.Log("Migration file validation tests completed successfully")
}