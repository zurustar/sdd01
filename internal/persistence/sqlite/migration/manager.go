package migration

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"
)

// migrationManagerImpl implements the MigrationManager interface
type migrationManagerImpl struct {
	scanner  FileScanner
	executor Executor
	migrationDir string
}

// NewMigrationManager creates a new MigrationManager implementation
func NewMigrationManager(scanner FileScanner, executor Executor, migrationDir string) MigrationManager {
	return &migrationManagerImpl{
		scanner:      scanner,
		executor:     executor,
		migrationDir: migrationDir,
	}
}

// RunMigrations executes all pending migrations in sequential order
func (m *migrationManagerImpl) RunMigrations(ctx context.Context) error {
	startTime := time.Now()
	
	log.Printf("Migration system starting - initializing version tracking table")
	
	// Initialize the version tracking table with error handling
	if err := m.executor.InitializeVersionTable(ctx); err != nil {
		log.Printf("ERROR: Failed to initialize schema_migrations table: %v", err)
		return fmt.Errorf("failed to initialize version table: %w", err)
	}
	
	log.Printf("Version tracking table initialized successfully")
	
	// Log current schema version before checking for pending migrations
	if err := m.LogCurrentSchemaVersion(ctx); err != nil {
		log.Printf("WARNING: Could not log current schema version: %v", err)
	}
	
	// Get pending migrations with comprehensive error handling
	log.Printf("Scanning migration directory: %s", m.migrationDir)
	
	// Log pending migrations with detailed information
	if err := m.LogPendingMigrations(ctx); err != nil {
		log.Printf("ERROR: Failed to log pending migrations: %v", err)
		return fmt.Errorf("failed to log pending migrations: %w", err)
	}
	
	// Get the actual pending migrations for execution
	pendingMigrations, err := m.GetPendingMigrations(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to scan for pending migrations: %v", err)
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}
	
	// Early return if no migrations are pending
	if len(pendingMigrations) == 0 {
		return nil
	}
	
	log.Printf("Starting migration execution sequence")
	
	// Execute each pending migration in sequential order
	for i, migration := range pendingMigrations {
		migrationStartTime := time.Now()
		
		log.Printf("=== Executing migration %s: %s (%d/%d) ===", 
			migration.Version, migration.Description, i+1, len(pendingMigrations))
		log.Printf("Migration file: %s", migration.FilePath)
		log.Printf("Migration checksum: %s", migration.Checksum)
		
		// Execute the migration with detailed error context
		if err := m.executor.ExecuteMigration(ctx, migration); err != nil {
			log.Printf("ERROR: Migration %s failed during execution: %v", migration.Version, err)
			log.Printf("Migration file: %s", migration.FilePath)
			
			// Provide detailed error context
			migrationErr := NewMigrationError(migration.Version, migration.FilePath, 
				"execute migration", fmt.Errorf("%w: %v", ErrMigrationFailed, err))
			
			log.Printf("Migration execution aborted due to failure in migration %s", migration.Version)
			return migrationErr
		}
		
		// Record the successful migration
		executionTime := time.Since(migrationStartTime)
		if err := m.executor.RecordMigration(ctx, migration.Version, executionTime); err != nil {
			log.Printf("ERROR: Failed to record migration %s in version table: %v", migration.Version, err)
			return NewMigrationError(migration.Version, migration.FilePath, 
				"record migration", fmt.Errorf("failed to record migration: %w", err))
		}
		
		log.Printf("Migration %s completed successfully in %v", 
			migration.Version, executionTime)
	}
	
	totalTime := time.Since(startTime)
	log.Printf("=== All %d migrations completed successfully in %v ===", 
		len(pendingMigrations), totalTime)
	
	// Log final status using the dedicated method
	if err := m.LogCurrentSchemaVersion(ctx); err != nil {
		log.Printf("WARNING: Could not log final migration status: %v", err)
	}
	
	return nil
}

// GetAppliedVersions returns list of migration versions that have been applied
func (m *migrationManagerImpl) GetAppliedVersions(ctx context.Context) ([]string, error) {
	// Initialize version table if it doesn't exist
	if err := m.executor.InitializeVersionTable(ctx); err != nil {
		log.Printf("ERROR: Failed to initialize version table: %v", err)
		return nil, fmt.Errorf("failed to initialize version table: %w", err)
	}
	
	// Get applied migrations from executor with error handling
	appliedMigrations, err := m.executor.GetAppliedVersions(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to query applied versions from database: %v", err)
		return nil, fmt.Errorf("failed to get applied versions: %w", err)
	}
	
	// Extract just the version strings
	versions := make([]string, len(appliedMigrations))
	for i, migration := range appliedMigrations {
		versions[i] = migration.Version
	}
	
	if len(versions) > 0 {
		log.Printf("Retrieved %d applied migration versions from database", len(versions))
	}
	
	return versions, nil
}

// GetPendingMigrations returns list of migrations that need to be applied
func (m *migrationManagerImpl) GetPendingMigrations(ctx context.Context) ([]Migration, error) {
	// Scan available migrations from filesystem with error handling
	log.Printf("Scanning for available migration files in directory: %s", m.migrationDir)
	availableMigrations, err := m.scanner.ScanMigrations(m.migrationDir)
	if err != nil {
		log.Printf("ERROR: Failed to scan migration directory %s: %v", m.migrationDir, err)
		return nil, fmt.Errorf("failed to scan migrations: %w", err)
	}
	
	log.Printf("Found %d available migration files", len(availableMigrations))
	
	// Get applied versions with error handling
	log.Printf("Retrieving applied migration versions from database")
	appliedVersions, err := m.GetAppliedVersions(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to retrieve applied versions from database: %v", err)
		return nil, fmt.Errorf("failed to get applied versions: %w", err)
	}
	
	log.Printf("Found %d previously applied migrations", len(appliedVersions))
	
	// Create a map of applied versions for quick lookup
	appliedMap := make(map[string]bool)
	for _, version := range appliedVersions {
		appliedMap[version] = true
	}
	
	// Filter out already applied migrations
	var pendingMigrations []Migration
	for _, migration := range availableMigrations {
		if !appliedMap[migration.Version] {
			pendingMigrations = append(pendingMigrations, migration)
			log.Printf("Pending migration found: %s - %s", migration.Version, migration.Description)
		} else {
			log.Printf("Migration %s already applied, skipping", migration.Version)
		}
	}
	
	// Validate migration sequence - ensure no gaps in version numbers
	log.Printf("Validating migration sequence integrity")
	if err := m.validateMigrationSequence(availableMigrations, appliedVersions); err != nil {
		log.Printf("ERROR: Migration sequence validation failed: %v", err)
		return nil, fmt.Errorf("migration sequence validation failed: %w", err)
	}
	
	log.Printf("Migration sequence validation passed")
	
	// Sort pending migrations by version number to ensure correct execution order
	sort.Slice(pendingMigrations, func(i, j int) bool {
		versionI, _ := strconv.Atoi(pendingMigrations[i].Version)
		versionJ, _ := strconv.Atoi(pendingMigrations[j].Version)
		return versionI < versionJ
	})
	
	return pendingMigrations, nil
}

// GetMigrationStatus returns status information about migrations
func (m *migrationManagerImpl) GetMigrationStatus(ctx context.Context) (*MigrationStatus, error) {
	// Get applied migrations with full details
	appliedMigrations, err := m.executor.GetAppliedVersions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}
	
	// Get pending migrations
	pendingMigrations, err := m.GetPendingMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending migrations: %w", err)
	}
	
	// Determine current version (latest applied migration)
	currentVersion := ""
	if len(appliedMigrations) > 0 {
		// Find the highest version number
		maxVersion := 0
		for _, migration := range appliedMigrations {
			if version, err := strconv.Atoi(migration.Version); err == nil {
				if version > maxVersion {
					maxVersion = version
					currentVersion = migration.Version
				}
			}
		}
	}
	
	status := &MigrationStatus{
		CurrentVersion:    currentVersion,
		PendingCount:      len(pendingMigrations),
		AppliedMigrations: appliedMigrations,
		PendingMigrations: pendingMigrations,
	}
	
	return status, nil
}

// validateMigrationSequence ensures there are no gaps in migration version numbers
func (m *migrationManagerImpl) validateMigrationSequence(availableMigrations []Migration, appliedVersions []string) error {
	if len(availableMigrations) == 0 {
		log.Printf("No migrations available for sequence validation")
		return nil // No migrations to validate
	}
	
	log.Printf("Validating migration sequence for %d available and %d applied migrations", 
		len(availableMigrations), len(appliedVersions))
	
	// Convert version strings to integers for validation
	var availableVersions []int
	for _, migration := range availableMigrations {
		version, err := strconv.Atoi(migration.Version)
		if err != nil {
			log.Printf("ERROR: Invalid version number '%s' in migration file %s", 
				migration.Version, migration.FilePath)
			return NewMigrationError(migration.Version, migration.FilePath, 
				"validate sequence", fmt.Errorf("%w: version '%s' is not numeric", ErrInvalidVersion, migration.Version))
		}
		availableVersions = append(availableVersions, version)
	}
	
	var appliedVersionInts []int
	for _, versionStr := range appliedVersions {
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			log.Printf("ERROR: Invalid version number '%s' found in schema_migrations table", versionStr)
			return NewDatabaseError(versionStr, "", "validate sequence", 
				fmt.Errorf("%w: applied version '%s' is not numeric", ErrVersionTableCorrupt, versionStr))
		}
		appliedVersionInts = append(appliedVersionInts, version)
	}
	
	// Check for gaps in available migrations
	if len(availableVersions) > 0 {
		minVersion := availableVersions[0]
		maxVersion := availableVersions[len(availableVersions)-1]
		
		log.Printf("Checking for gaps in migration sequence from version %d to %d", minVersion, maxVersion)
		
		// Ensure all versions from min to max exist
		versionMap := make(map[int]bool)
		for _, version := range availableVersions {
			versionMap[version] = true
		}
		
		for version := minVersion; version <= maxVersion; version++ {
			if !versionMap[version] {
				log.Printf("ERROR: Missing migration version %03d in sequence", version)
				return fmt.Errorf("%w: missing migration version %03d in sequence", ErrVersionConflict, version)
			}
		}
		
		log.Printf("Migration sequence is continuous from %d to %d", minVersion, maxVersion)
	}
	
	// Check that no applied migration is missing from available migrations
	availableMap := make(map[int]bool)
	for _, version := range availableVersions {
		availableMap[version] = true
	}
	
	for _, appliedVersion := range appliedVersionInts {
		if !availableMap[appliedVersion] {
			log.Printf("ERROR: Applied migration %03d not found in available migration files", appliedVersion)
			return fmt.Errorf("%w: applied migration %03d not found in available migrations", 
				ErrVersionConflict, appliedVersion)
		}
	}
	
	log.Printf("All applied migrations have corresponding migration files")
	
	return nil
}

// ListAppliedMigrations returns all applied migrations with timestamps and execution details
func (m *migrationManagerImpl) ListAppliedMigrations(ctx context.Context) ([]AppliedMigration, error) {
	// Initialize version table if it doesn't exist
	if err := m.executor.InitializeVersionTable(ctx); err != nil {
		log.Printf("ERROR: Failed to initialize version table: %v", err)
		return nil, fmt.Errorf("failed to initialize version table: %w", err)
	}
	
	// Get applied migrations from executor
	appliedMigrations, err := m.executor.GetAppliedVersions(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to retrieve applied migrations: %v", err)
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}
	
	log.Printf("Retrieved %d applied migrations from database", len(appliedMigrations))
	
	return appliedMigrations, nil
}

// LogCurrentSchemaVersion logs the current database schema version
func (m *migrationManagerImpl) LogCurrentSchemaVersion(ctx context.Context) error {
	status, err := m.GetMigrationStatus(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to get migration status: %v", err)
		return fmt.Errorf("failed to get migration status: %w", err)
	}
	
	if status.CurrentVersion == "" {
		log.Printf("Database schema: No migrations applied (empty database)")
	} else {
		log.Printf("Database schema: Current version is %s", status.CurrentVersion)
		
		// Log additional details about the current version
		appliedMigrations, err := m.ListAppliedMigrations(ctx)
		if err != nil {
			log.Printf("WARNING: Could not retrieve applied migration details: %v", err)
		} else {
			// Find the current version details
			for _, migration := range appliedMigrations {
				if migration.Version == status.CurrentVersion {
					log.Printf("Current version %s applied at %s (execution time: %v)", 
						migration.Version, 
						migration.AppliedAt.Format("2006-01-02 15:04:05 UTC"), 
						migration.ExecutionTime)
					break
				}
			}
		}
	}
	
	if status.PendingCount > 0 {
		log.Printf("Database schema: %d pending migrations available", status.PendingCount)
	} else {
		log.Printf("Database schema: Up to date (no pending migrations)")
	}
	
	return nil
}

// LogPendingMigrations logs information about pending migrations before execution
func (m *migrationManagerImpl) LogPendingMigrations(ctx context.Context) error {
	pendingMigrations, err := m.GetPendingMigrations(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to get pending migrations: %v", err)
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}
	
	if len(pendingMigrations) == 0 {
		log.Printf("Migration status: No pending migrations - database is up to date")
		return nil
	}
	
	log.Printf("Migration status: %d pending migrations found", len(pendingMigrations))
	log.Printf("Pending migrations to be executed:")
	
	for i, migration := range pendingMigrations {
		log.Printf("  %d. Version %s: %s", i+1, migration.Version, migration.Description)
		log.Printf("     File: %s", migration.FilePath)
		if migration.Checksum != "" {
			log.Printf("     Checksum: %s", migration.Checksum)
		}
	}
	
	return nil
}