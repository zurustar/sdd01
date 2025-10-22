package migration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Mock implementations for testing

type mockFileScanner struct {
	migrations []Migration
	scanError  error
}

func (m *mockFileScanner) ScanMigrations(migrationDir string) ([]Migration, error) {
	if m.scanError != nil {
		return nil, m.scanError
	}
	return m.migrations, nil
}

func (m *mockFileScanner) ValidateFileName(filename string) error {
	return nil
}

func (m *mockFileScanner) ParseMigrationFile(filePath string) (*Migration, error) {
	return nil, nil
}

type mockExecutor struct {
	appliedVersions    []AppliedMigration
	executionError     error
	recordError        error
	initError          error
	isVersionAppliedFn func(version string) bool
	executionOrder     []string
}

func (m *mockExecutor) ExecuteMigration(ctx context.Context, migration Migration) error {
	if m.executionOrder != nil {
		m.executionOrder = append(m.executionOrder, migration.Version)
	}
	return m.executionError
}

func (m *mockExecutor) InitializeVersionTable(ctx context.Context) error {
	return m.initError
}

func (m *mockExecutor) RecordMigration(ctx context.Context, version string, executionTime time.Duration) error {
	return m.recordError
}

func (m *mockExecutor) IsVersionApplied(ctx context.Context, version string) (bool, error) {
	if m.isVersionAppliedFn != nil {
		return m.isVersionAppliedFn(version), nil
	}
	return false, nil
}

func (m *mockExecutor) GetAppliedVersions(ctx context.Context) ([]AppliedMigration, error) {
	return m.appliedVersions, nil
}

func TestMigrationManager_RunMigrations_Success(t *testing.T) {
	// Setup test data
	availableMigrations := []Migration{
		{Version: "001", Description: "Initial schema", SQL: "CREATE TABLE users (id INTEGER);", FilePath: "001_initial.sql"},
		{Version: "002", Description: "Add indexes", SQL: "CREATE INDEX idx_users ON users(id);", FilePath: "002_indexes.sql"},
	}
	
	appliedVersions := []AppliedMigration{
		{Version: "001", AppliedAt: time.Now(), ExecutionTime: 100 * time.Millisecond},
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{appliedVersions: appliedVersions}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test successful migration execution
	ctx := context.Background()
	err := manager.RunMigrations(ctx)
	
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestMigrationManager_RunMigrations_NoMigrations(t *testing.T) {
	// Setup with no pending migrations
	availableMigrations := []Migration{
		{Version: "001", Description: "Initial schema", SQL: "CREATE TABLE users (id INTEGER);", FilePath: "001_initial.sql"},
	}
	
	appliedVersions := []AppliedMigration{
		{Version: "001", AppliedAt: time.Now(), ExecutionTime: 100 * time.Millisecond},
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{appliedVersions: appliedVersions}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test when no migrations are pending
	ctx := context.Background()
	err := manager.RunMigrations(ctx)
	
	if err != nil {
		t.Errorf("Expected no error when no migrations pending, got: %v", err)
	}
}

func TestMigrationManager_RunMigrations_InitializationError(t *testing.T) {
	scanner := &mockFileScanner{}
	executor := &mockExecutor{initError: errors.New("failed to create table")}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test initialization error
	ctx := context.Background()
	err := manager.RunMigrations(ctx)
	
	if err == nil {
		t.Error("Expected error during initialization, got nil")
	}
	
	expectedMsg := "failed to initialize version table"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}
}

func TestMigrationManager_RunMigrations_ExecutionError(t *testing.T) {
	// Setup test data with execution error
	availableMigrations := []Migration{
		{Version: "001", Description: "Initial schema", SQL: "INVALID SQL;", FilePath: "001_initial.sql"},
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{
		appliedVersions: []AppliedMigration{},
		executionError:  errors.New("SQL syntax error"),
	}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test migration execution error
	ctx := context.Background()
	err := manager.RunMigrations(ctx)
	
	if err == nil {
		t.Error("Expected error during migration execution, got nil")
	}
	
	var migrationErr *MigrationError
	if !errors.As(err, &migrationErr) {
		t.Errorf("Expected MigrationError, got: %T", err)
	}
	
	if migrationErr.Version != "001" {
		t.Errorf("Expected error for version 001, got version: %s", migrationErr.Version)
	}
}

func TestMigrationManager_RunMigrations_RecordError(t *testing.T) {
	// Setup test data with record error
	availableMigrations := []Migration{
		{Version: "001", Description: "Initial schema", SQL: "CREATE TABLE users (id INTEGER);", FilePath: "001_initial.sql"},
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{
		appliedVersions: []AppliedMigration{},
		recordError:     errors.New("failed to record migration"),
	}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test migration record error
	ctx := context.Background()
	err := manager.RunMigrations(ctx)
	
	if err == nil {
		t.Error("Expected error during migration recording, got nil")
	}
	
	var migrationErr *MigrationError
	if !errors.As(err, &migrationErr) {
		t.Errorf("Expected MigrationError, got: %T", err)
	}
}

func TestMigrationManager_GetPendingMigrations_Success(t *testing.T) {
	// Setup test data
	availableMigrations := []Migration{
		{Version: "001", Description: "Initial schema", SQL: "CREATE TABLE users (id INTEGER);", FilePath: "001_initial.sql"},
		{Version: "002", Description: "Add indexes", SQL: "CREATE INDEX idx_users ON users(id);", FilePath: "002_indexes.sql"},
		{Version: "003", Description: "Add constraints", SQL: "ALTER TABLE users ADD CONSTRAINT pk_users PRIMARY KEY (id);", FilePath: "003_constraints.sql"},
	}
	
	appliedVersions := []AppliedMigration{
		{Version: "001", AppliedAt: time.Now(), ExecutionTime: 100 * time.Millisecond},
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{appliedVersions: appliedVersions}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test getting pending migrations
	ctx := context.Background()
	pending, err := manager.GetPendingMigrations(ctx)
	
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending migrations, got: %d", len(pending))
	}
	
	// Check that the correct migrations are pending
	expectedVersions := []string{"002", "003"}
	for i, migration := range pending {
		if migration.Version != expectedVersions[i] {
			t.Errorf("Expected version %s at index %d, got: %s", expectedVersions[i], i, migration.Version)
		}
	}
}

func TestMigrationManager_GetPendingMigrations_ScanError(t *testing.T) {
	scanner := &mockFileScanner{scanError: errors.New("directory not found")}
	executor := &mockExecutor{}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test scan error
	ctx := context.Background()
	_, err := manager.GetPendingMigrations(ctx)
	
	if err == nil {
		t.Error("Expected error during scanning, got nil")
	}
}

func TestMigrationManager_GetPendingMigrations_SequenceValidationError(t *testing.T) {
	// Setup test data with gap in sequence
	availableMigrations := []Migration{
		{Version: "001", Description: "Initial schema", SQL: "CREATE TABLE users (id INTEGER);", FilePath: "001_initial.sql"},
		{Version: "003", Description: "Add constraints", SQL: "ALTER TABLE users ADD CONSTRAINT pk_users PRIMARY KEY (id);", FilePath: "003_constraints.sql"}, // Missing 002
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{appliedVersions: []AppliedMigration{}}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test sequence validation error
	ctx := context.Background()
	_, err := manager.GetPendingMigrations(ctx)
	
	if err == nil {
		t.Error("Expected sequence validation error, got nil")
	}
	
	if !errors.Is(err, ErrVersionConflict) {
		t.Errorf("Expected ErrVersionConflict, got: %v", err)
	}
}

func TestMigrationManager_GetAppliedVersions_Success(t *testing.T) {
	appliedVersions := []AppliedMigration{
		{Version: "001", AppliedAt: time.Now(), ExecutionTime: 100 * time.Millisecond},
		{Version: "002", AppliedAt: time.Now(), ExecutionTime: 150 * time.Millisecond},
	}
	
	scanner := &mockFileScanner{}
	executor := &mockExecutor{appliedVersions: appliedVersions}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test getting applied versions
	ctx := context.Background()
	versions, err := manager.GetAppliedVersions(ctx)
	
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	if len(versions) != 2 {
		t.Errorf("Expected 2 applied versions, got: %d", len(versions))
	}
	
	expectedVersions := []string{"001", "002"}
	for i, version := range versions {
		if version != expectedVersions[i] {
			t.Errorf("Expected version %s at index %d, got: %s", expectedVersions[i], i, version)
		}
	}
}

func TestMigrationManager_GetMigrationStatus_Success(t *testing.T) {
	availableMigrations := []Migration{
		{Version: "001", Description: "Initial schema", SQL: "CREATE TABLE users (id INTEGER);", FilePath: "001_initial.sql"},
		{Version: "002", Description: "Add indexes", SQL: "CREATE INDEX idx_users ON users(id);", FilePath: "002_indexes.sql"},
		{Version: "003", Description: "Add constraints", SQL: "ALTER TABLE users ADD CONSTRAINT pk_users PRIMARY KEY (id);", FilePath: "003_constraints.sql"},
	}
	
	appliedVersions := []AppliedMigration{
		{Version: "001", AppliedAt: time.Now(), ExecutionTime: 100 * time.Millisecond},
		{Version: "002", AppliedAt: time.Now(), ExecutionTime: 150 * time.Millisecond},
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{appliedVersions: appliedVersions}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test getting migration status
	ctx := context.Background()
	status, err := manager.GetMigrationStatus(ctx)
	
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	if status.CurrentVersion != "002" {
		t.Errorf("Expected current version 002, got: %s", status.CurrentVersion)
	}
	
	if status.PendingCount != 1 {
		t.Errorf("Expected 1 pending migration, got: %d", status.PendingCount)
	}
	
	if len(status.AppliedMigrations) != 2 {
		t.Errorf("Expected 2 applied migrations, got: %d", len(status.AppliedMigrations))
	}
	
	if len(status.PendingMigrations) != 1 {
		t.Errorf("Expected 1 pending migration, got: %d", len(status.PendingMigrations))
	}
}

func TestMigrationManager_RunMigrations_Idempotency(t *testing.T) {
	// Setup test data
	availableMigrations := []Migration{
		{Version: "001", Description: "Initial schema", SQL: "CREATE TABLE users (id INTEGER);", FilePath: "001_initial.sql"},
	}
	
	appliedVersions := []AppliedMigration{
		{Version: "001", AppliedAt: time.Now(), ExecutionTime: 100 * time.Millisecond},
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{appliedVersions: appliedVersions}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test idempotency - running migrations multiple times should not cause errors
	ctx := context.Background()
	
	// First run
	err := manager.RunMigrations(ctx)
	if err != nil {
		t.Errorf("Expected no error on first run, got: %v", err)
	}
	
	// Second run (should be idempotent)
	err = manager.RunMigrations(ctx)
	if err != nil {
		t.Errorf("Expected no error on second run (idempotency), got: %v", err)
	}
}

func TestMigrationManager_ValidateMigrationSequence_InvalidVersion(t *testing.T) {
	// Setup test data with invalid version
	availableMigrations := []Migration{
		{Version: "abc", Description: "Invalid version", SQL: "CREATE TABLE users (id INTEGER);", FilePath: "abc_invalid.sql"},
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{appliedVersions: []AppliedMigration{}}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test invalid version error
	ctx := context.Background()
	_, err := manager.GetPendingMigrations(ctx)
	
	if err == nil {
		t.Error("Expected error for invalid version, got nil")
	}
	
	var migrationErr *MigrationError
	if !errors.As(err, &migrationErr) {
		t.Errorf("Expected MigrationError, got: %T", err)
	}
	
	if !errors.Is(migrationErr.Err, ErrInvalidVersion) {
		t.Errorf("Expected ErrInvalidVersion, got: %v", migrationErr.Err)
	}
}

func TestMigrationManager_ValidateMigrationSequence_MissingAppliedMigration(t *testing.T) {
	// Setup test data where applied migration is not in available migrations
	availableMigrations := []Migration{
		{Version: "002", Description: "Second migration", SQL: "CREATE TABLE posts (id INTEGER);", FilePath: "002_posts.sql"},
	}
	
	appliedVersions := []AppliedMigration{
		{Version: "001", AppliedAt: time.Now(), ExecutionTime: 100 * time.Millisecond}, // This migration file is missing
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{appliedVersions: appliedVersions}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test missing applied migration error
	ctx := context.Background()
	_, err := manager.GetPendingMigrations(ctx)
	
	if err == nil {
		t.Error("Expected error for missing applied migration, got nil")
	}
	
	if !errors.Is(err, ErrVersionConflict) {
		t.Errorf("Expected ErrVersionConflict, got: %v", err)
	}
}

func TestMigrationManager_ExecutionOrder(t *testing.T) {
	// Setup test data to verify execution order
	availableMigrations := []Migration{
		{Version: "003", Description: "Third migration", SQL: "CREATE TABLE comments (id INTEGER);", FilePath: "003_comments.sql"},
		{Version: "001", Description: "First migration", SQL: "CREATE TABLE users (id INTEGER);", FilePath: "001_users.sql"},
		{Version: "002", Description: "Second migration", SQL: "CREATE TABLE posts (id INTEGER);", FilePath: "002_posts.sql"},
	}
	
	scanner := &mockFileScanner{migrations: availableMigrations}
	executor := &mockExecutor{
		appliedVersions: []AppliedMigration{},
		executionError:  nil,
		executionOrder:  make([]string, 0), // Initialize slice to track execution order
	}
	
	manager := NewMigrationManager(scanner, executor, "/test/migrations")
	
	// Test that migrations are executed in correct order
	ctx := context.Background()
	err := manager.RunMigrations(ctx)
	
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	expectedOrder := []string{"001", "002", "003"}
	if len(executor.executionOrder) != len(expectedOrder) {
		t.Errorf("Expected %d migrations executed, got: %d", len(expectedOrder), len(executor.executionOrder))
	}
	
	for i, version := range executor.executionOrder {
		if version != expectedOrder[i] {
			t.Errorf("Expected version %s at position %d, got: %s", expectedOrder[i], i, version)
		}
	}
}