# Implementation Plan

- [x] 1. Create migration infrastructure interfaces and core types
  - Define Migration, MigrationManager, FileScanner, and Executor interfaces in `internal/persistence/sqlite/migration/`
  - Implement core Migration struct with version, description, SQL, and file path fields
  - Create error types for migration-specific failures (ErrMigrationFailed, ErrInvalidMigrationFile, etc.)
  - _Requirements: 1.1, 2.1, 3.1, 5.1_

- [-] 2. Implement migration file scanner
  - [x] 2.1 Create FileScanner implementation to scan migration directory
    - Implement directory scanning logic to find .sql files
    - Parse version numbers from filenames using pattern `{version}_{description}.sql`
    - Sort migrations by version number in ascending order
    - _Requirements: 3.1, 3.2, 3.3_

  - [x] 2.2 Add migration file validation
    - Validate filename format matches expected pattern
    - Check for duplicate version numbers
    - Verify SQL file readability and basic syntax
    - _Requirements: 3.4, 5.2_

  - [x] 2.3 Write unit tests for file scanner
    - Test valid migration directory scanning
    - Test invalid filename handling
    - Test version number parsing and sorting
    - _Requirements: 3.1, 3.2, 3.3, 3.4_

- [x] 3. Implement migration executor with transaction support
  - [x] 3.1 Create Executor implementation for SQL execution
    - Execute migration SQL statements within database transactions
    - Handle transaction rollback on SQL execution errors
    - Parse and execute multiple SQL statements from migration files
    - _Requirements: 4.1, 4.2, 4.3_

  - [x] 3.2 Add schema_migrations table management
    - Create schema_migrations table if it doesn't exist
    - Record successful migrations with version and timestamp
    - Query applied migrations to determine pending migrations
    - _Requirements: 2.1, 2.2, 2.3_

  - [x] 3.3 Write unit tests for migration executor
    - Test successful migration execution and version recording
    - Test transaction rollback on SQL errors
    - Test schema_migrations table creation and updates
    - _Requirements: 4.1, 4.2, 4.3, 2.1, 2.2_

- [x] 4. Implement migration manager orchestration
  - [x] 4.1 Create MigrationManager implementation
    - Coordinate file scanning, version checking, and migration execution
    - Implement GetPendingMigrations by comparing available vs applied migrations
    - Execute pending migrations in sequential order with proper error handling
    - _Requirements: 1.1, 1.2, 1.3, 2.3_

  - [x] 4.2 Add comprehensive logging and error handling
    - Log migration start, progress, and completion with execution time
    - Provide detailed error context including migration version and SQL statement
    - Handle file system errors and database connection issues gracefully
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [x] 4.3 Write unit tests for migration manager
    - Test pending migration detection and execution order
    - Test error handling and logging for failed migrations
    - Test idempotency (running migrations multiple times)
    - _Requirements: 1.1, 1.2, 1.3, 2.3, 5.1, 5.2_

- [x] 5. Add SQLite-specific configuration and connection management
  - [x] 5.1 Create ConnectionManager for SQLite configuration
    - Configure SQLite connection with foreign key constraints enabled
    - Set appropriate timeout and busy handler settings
    - Apply SQLite-specific PRAGMA settings for optimal performance
    - _Requirements: 6.1, 6.2, 6.3_

  - [x] 5.2 Add migration configuration support
    - Create MigrationConfig struct with directory path, timeout, and retry settings
    - Support enabling/disabling migrations via configuration
    - Add validation for migration configuration parameters
    - _Requirements: 6.4, 8.3_

  - [x] 5.3 Write unit tests for configuration management
    - Test SQLite connection configuration and PRAGMA settings
    - Test migration configuration validation and defaults
    - Test database file creation with proper permissions
    - _Requirements: 6.1, 6.2, 6.3, 6.4_

- [x] 6. Integrate migration system with existing Storage.Migrate method
  - [x] 6.1 Modify sqlite.Storage.Migrate to use new migration system
    - Replace embedded schema.sql execution with migration manager
    - Initialize migration manager with appropriate configuration
    - Handle migration failures by preventing application startup
    - _Requirements: 1.1, 1.3, 8.1, 8.2_

  - [x] 6.2 Create initial migration file from existing schema
    - Convert current schema.sql to 001_initial_schema.sql migration file
    - Place migration file in `internal/storage/sqlite/migrations/` directory
    - Ensure migration file includes all current table definitions and indexes
    - _Requirements: 3.1, 3.2_

  - [x] 6.3 Write integration tests for Storage.Migrate
    - Test migration system integration with existing Storage interface
    - Test application startup with pending migrations
    - Test migration failure handling and application exit behavior
    - _Requirements: 1.1, 1.3, 8.1, 8.2_

- [-] 7. Add migration status visibility and operational features
  - [x] 7.1 Implement migration status reporting
    - Add method to list all applied migrations with timestamps
    - Log current schema version during application startup
    - Provide visibility into pending migrations before execution
    - _Requirements: 7.1, 7.2, 7.3_

  - [x] 7.2 Add support for test database migrations
    - Support in-memory database migration for testing scenarios
    - Create test migration directory with sample migration files
    - Ensure migration system works with temporary test databases
    - _Requirements: 8.4_

  - [-] 7.3 Write end-to-end integration tests
    - [x] 7.3.1 Create basic migration workflow tests with stub database
      - Test complete migration workflow from file scanning to execution using stub driver
      - Test migration file scanning with real files on filesystem
      - Test basic migration idempotency with stub database
      - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2_

    - [x] 7.3.2 Create error recovery and validation tests
      - Test invalid migration file name handling
      - Test corrupted migration file handling
      - Test missing migration file detection
      - Test migration sequence validation (gaps, duplicates)
      - _Requirements: 1.3, 2.3, 2.4_

    - [x] 7.3.3 Create file operation tests
      - Test large migration file handling
      - Test migration file permissions
      - Test UTF-8 encoded migration files
      - _Requirements: 1.1, 1.2_

    - [x] 7.3.4 Fix migration description parsing issues
      - Fix description extraction from migration file comments
      - Ensure proper handling of multi-line descriptions
      - Test description parsing with various comment formats
      - _Requirements: 2.1, 2.2_

    - [x] 7.3.5 Fix concurrent migration test issues
      - Resolve database connection sharing issues in concurrent tests
      - Fix migration status reporting with stub driver
      - Test proper locking behavior during concurrent access
      - _Requirements: 1.4, 2.4_

    - [x] 7.3.6 Add real database integration tests (optional)
      - Create tests that work with actual SQLite database operations
      - Test data integrity after migrations
      - Test foreign key constraints and indexes
      - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 8. Update application startup integration
  - [x] 8.1 Modify cmd/scheduler/main.go to use migration system
    - Initialize migration manager before HTTP server startup
    - Configure migration system with appropriate settings
    - Handle migration failures by exiting with non-zero status code
    - _Requirements: 8.1, 8.2_

  - [x] 8.2 Add migration logging to application startup
    - Log migration execution progress during application startup
    - Log successful migration completion with schema version
    - Log when no migrations are pending (database up to date)
    - _Requirements: 7.2, 7.4_

  - [x] 8.3 Write application startup integration tests
    - Test application startup with pending migrations
    - Test application startup failure when migrations fail
    - Test application startup with no pending migrations
    - _Requirements: 8.1, 8.2, 7.2, 7.4_