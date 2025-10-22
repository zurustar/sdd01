# Design Document

## Overview

データベースマイグレーションシステムは、Enterprise Scheduler アプリケーションのSQLiteデータベーススキーマを自動的に管理するシステムです。現在の実装では、埋め込まれたスキーマファイルを使用していますが、本設計では、バージョン管理されたマイグレーションファイルシステムに移行し、段階的なスキーマ変更を安全に適用できるようにします。

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Startup                      │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────┐
│                Migration Manager                            │
│  - Check pending migrations                                 │
│  - Execute migrations in order                              │
│  - Track applied versions                                   │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────┐
│              Migration File Scanner                         │
│  - Scan migration directory                                 │
│  - Parse version numbers                                    │
│  - Validate file format                                     │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────┐
│              Migration Executor                             │
│  - Execute SQL statements                                   │
│  - Handle transactions                                      │
│  - Update version tracking                                  │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────┐
│                SQLite Database                              │
│  - Application tables                                       │
│  - schema_migrations table                                  │
└─────────────────────────────────────────────────────────────┘
```

### Integration with Existing Code

現在の `internal/persistence/sqlite/sqlite.go` の `Migrate` メソッドを拡張し、埋め込まれたスキーマファイルの代わりに、ファイルシステムベースのマイグレーションシステムを使用します。

## Components and Interfaces

### Migration Manager

```go
type MigrationManager interface {
    // RunMigrations executes all pending migrations
    RunMigrations(ctx context.Context) error
    
    // GetAppliedVersions returns list of applied migration versions
    GetAppliedVersions(ctx context.Context) ([]string, error)
    
    // GetPendingMigrations returns list of migrations to be applied
    GetPendingMigrations(ctx context.Context) ([]Migration, error)
}

type Migration struct {
    Version     string
    Description string
    SQL         string
    FilePath    string
}
```

### Migration File Scanner

```go
type FileScanner interface {
    // ScanMigrations scans the migration directory for migration files
    ScanMigrations(migrationDir string) ([]Migration, error)
    
    // ValidateFileName checks if migration file follows naming convention
    ValidateFileName(filename string) error
}
```

### Migration Executor

```go
type Executor interface {
    // ExecuteMigration runs a single migration in a transaction
    ExecuteMigration(ctx context.Context, migration Migration) error
    
    // InitializeVersionTable creates schema_migrations table if needed
    InitializeVersionTable(ctx context.Context) error
    
    // RecordMigration records successful migration in version table
    RecordMigration(ctx context.Context, version string) error
}
```

### Database Connection Manager

```go
type ConnectionManager interface {
    // GetConnection returns configured SQLite connection
    GetConnection() (*sql.DB, error)
    
    // ConfigureDatabase applies SQLite-specific settings
    ConfigureDatabase(db *sql.DB) error
}
```

## Data Models

### Schema Migrations Table

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    checksum TEXT,
    execution_time_ms INTEGER
);
```

### Migration File Structure

```
internal/storage/sqlite/migrations/
├── 001_initial_schema.sql
├── 002_add_user_roles.sql
├── 003_add_session_indexes.sql
└── 004_add_recurrence_constraints.sql
```

### Migration File Format

```sql
-- Migration: 001_initial_schema.sql
-- Description: Create initial database schema with users, rooms, schedules

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Additional tables...
```

## Error Handling

### Error Types

```go
var (
    ErrMigrationFailed     = errors.New("migration execution failed")
    ErrInvalidMigrationFile = errors.New("invalid migration file format")
    ErrMigrationNotFound   = errors.New("migration file not found")
    ErrVersionConflict     = errors.New("migration version conflict")
    ErrDatabaseLocked      = errors.New("database is locked")
)
```

### Error Recovery Strategy

1. **Transaction Rollback**: Each migration runs in its own transaction
2. **Detailed Logging**: Log migration progress and errors with context
3. **Graceful Degradation**: Application startup fails if migrations fail
4. **Error Context**: Wrap errors with migration version and SQL context

## Testing Strategy

### Unit Tests

```go
func TestMigrationManager_RunMigrations(t *testing.T) {
    // Test successful migration execution
    // Test handling of already applied migrations
    // Test error handling for failed migrations
}

func TestFileScanner_ScanMigrations(t *testing.T) {
    // Test scanning valid migration directory
    // Test handling of invalid file names
    // Test sorting by version number
}

func TestExecutor_ExecuteMigration(t *testing.T) {
    // Test successful SQL execution
    // Test transaction rollback on error
    // Test version tracking updates
}
```

### Integration Tests

```go
func TestMigrationSystem_EndToEnd(t *testing.T) {
    // Create temporary database
    // Apply migrations from test directory
    // Verify schema matches expected state
    // Test idempotency (running migrations twice)
}
```

### Test Migration Files

```
internal/storage/sqlite/migrations/testdata/
├── valid/
│   ├── 001_test_schema.sql
│   └── 002_test_data.sql
├── invalid/
│   ├── invalid_name.sql
│   └── 999_syntax_error.sql
└── fixtures/
    └── expected_schema.sql
```

## Configuration

### Migration Configuration

```go
type MigrationConfig struct {
    MigrationDir     string        // Directory containing migration files
    Enabled          bool          // Enable/disable migrations
    TimeoutPerFile   time.Duration // Timeout for each migration
    MaxRetries       int           // Retry count for failed migrations
    VerifyChecksum   bool          // Verify migration file checksums
}
```

### SQLite Configuration

```go
type SQLiteConfig struct {
    DSN              string        // Database file path
    BusyTimeout      time.Duration // SQLite busy timeout
    EnableForeignKeys bool         // Enable foreign key constraints
    JournalMode      string        // WAL, DELETE, etc.
    Synchronous      string        // FULL, NORMAL, OFF
}
```

## Implementation Plan

### Phase 1: Core Migration Infrastructure

1. Create migration manager interface and implementation
2. Implement file scanner for migration directory
3. Create migration executor with transaction support
4. Add schema_migrations table management

### Phase 2: Integration with Existing Code

1. Modify `sqlite.Storage.Migrate()` method to use new system
2. Update application startup to use migration manager
3. Create initial migration file from existing schema.sql
4. Add configuration support for migration settings

### Phase 3: Enhanced Features

1. Add migration checksum verification
2. Implement migration timeout and retry logic
3. Add detailed migration logging and metrics
4. Create migration status reporting endpoints

## Security Considerations

### File System Security

- Migration files are read-only after deployment
- Validate migration file paths to prevent directory traversal
- Use embedded files in production builds when possible

### SQL Injection Prevention

- Migration files are trusted content (not user input)
- Use parameterized queries where applicable
- Validate SQL syntax before execution

### Access Control

- Migration execution requires database write permissions
- Log all migration activities for audit trail
- Restrict migration directory access in production

## Performance Considerations

### Migration Execution

- Each migration runs in its own transaction for isolation
- Large migrations may require chunking for memory efficiency
- Consider migration timeouts for long-running operations

### Database Locking

- SQLite exclusive locks during schema changes
- Minimize migration execution time
- Consider maintenance windows for large migrations

### Monitoring

- Track migration execution time
- Monitor database size growth during migrations
- Alert on migration failures or timeouts

## Operational Considerations

### Deployment Process

1. Deploy new application version with migration files
2. Application startup automatically applies pending migrations
3. Verify migration success through health checks
4. Rollback deployment if migrations fail

### Backup Strategy

- Backup database before applying migrations
- Store migration backups with version information
- Test restore procedures with migration rollback

### Monitoring and Alerting

- Monitor migration execution status
- Alert on migration failures or timeouts
- Track migration performance metrics
- Log migration activities for audit purposes