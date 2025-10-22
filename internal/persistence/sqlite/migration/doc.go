// Package migration provides a database migration system for SQLite databases.
//
// This package implements a file-based migration system that allows for versioned
// database schema changes. It supports:
//
//   - Sequential migration execution with version tracking
//   - Transactional migration execution with rollback on failure
//   - File-based migration storage with structured naming conventions
//   - Comprehensive error handling and logging
//   - Integration with existing SQLite storage implementations
//
// Migration files should be placed in a designated directory and follow the naming
// convention: {version}_{description}.sql (e.g., "001_initial_schema.sql").
//
// The migration system maintains a schema_migrations table to track applied
// migrations and prevent duplicate execution.
//
// Example usage:
//
//	manager := NewMigrationManager(db, scanner, executor)
//	if err := manager.RunMigrations(ctx); err != nil {
//		log.Fatalf("Migration failed: %v", err)
//	}
package migration