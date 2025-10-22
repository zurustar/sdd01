package migration

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// fileScannerImpl implements the FileScanner interface
type fileScannerImpl struct {
	// migrationFilePattern defines the expected migration file naming pattern
	migrationFilePattern *regexp.Regexp
}

// NewFileScanner creates a new FileScanner implementation
func NewFileScanner() FileScanner {
	// Pattern matches: {version}_{description}.sql
	// Version must be numeric (001, 002, etc.)
	// Description can contain letters, numbers, underscores, and hyphens
	pattern := regexp.MustCompile(`^(\d+)_([a-zA-Z0-9_-]+)\.sql$`)
	
	return &fileScannerImpl{
		migrationFilePattern: pattern,
	}
}

// ScanMigrations scans the migration directory for migration files
func (fs *fileScannerImpl) ScanMigrations(migrationDir string) ([]Migration, error) {
	// Check if migration directory exists
	if _, err := os.Stat(migrationDir); os.IsNotExist(err) {
		return nil, NewFileSystemError(migrationDir, "scan directory", fmt.Errorf("migration directory does not exist"))
	}
	
	// Read directory contents
	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		return nil, NewFileSystemError(migrationDir, "read directory", err)
	}
	
	var migrations []Migration
	versionMap := make(map[string]string) // version -> filename for duplicate detection
	
	// Process each file in the directory
	for _, entry := range entries {
		// Skip directories and non-SQL files
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		
		// Validate filename format
		if err := fs.ValidateFileName(entry.Name()); err != nil {
			return nil, NewMigrationError("", entry.Name(), "validate filename", err)
		}
		
		// Parse migration file
		filePath := filepath.Join(migrationDir, entry.Name())
		migration, err := fs.ParseMigrationFile(filePath)
		if err != nil {
			return nil, err
		}
		
		// Check for duplicate versions
		if existingFile, exists := versionMap[migration.Version]; exists {
			return nil, NewMigrationError(migration.Version, entry.Name(), "check duplicates", 
				fmt.Errorf("%w: version %s found in both %s and %s", 
					ErrDuplicateVersion, migration.Version, existingFile, entry.Name()))
		}
		versionMap[migration.Version] = entry.Name()
		
		migrations = append(migrations, *migration)
	}
	
	// Sort migrations by version number in ascending order
	sort.Slice(migrations, func(i, j int) bool {
		versionI, _ := strconv.Atoi(migrations[i].Version)
		versionJ, _ := strconv.Atoi(migrations[j].Version)
		return versionI < versionJ
	})
	
	return migrations, nil
}

// ValidateFileName checks if migration file follows naming convention
func (fs *fileScannerImpl) ValidateFileName(filename string) error {
	if !fs.migrationFilePattern.MatchString(filename) {
		return fmt.Errorf("%w: filename '%s' does not match pattern '{version}_{description}.sql'", 
			ErrInvalidMigrationFile, filename)
	}
	
	// Extract version and validate it's numeric
	matches := fs.migrationFilePattern.FindStringSubmatch(filename)
	if len(matches) != 3 {
		return fmt.Errorf("%w: failed to parse filename '%s'", ErrInvalidMigrationFile, filename)
	}
	
	version := matches[1]
	description := matches[2]
	
	// Validate version is numeric and not empty
	if _, err := strconv.Atoi(version); err != nil {
		return fmt.Errorf("%w: version '%s' in filename '%s' is not a valid number", 
			ErrInvalidVersion, version, filename)
	}
	
	// Validate description is not empty
	if strings.TrimSpace(description) == "" {
		return fmt.Errorf("%w: description in filename '%s' cannot be empty", 
			ErrInvalidMigrationFile, filename)
	}
	
	return nil
}

// ParseMigrationFile reads and parses a single migration file
func (fs *fileScannerImpl) ParseMigrationFile(filePath string) (*Migration, error) {
	// Extract filename from path
	filename := filepath.Base(filePath)
	
	// Validate filename format
	if err := fs.ValidateFileName(filename); err != nil {
		return nil, NewMigrationError("", filePath, "validate filename", err)
	}
	
	// Parse version from filename
	matches := fs.migrationFilePattern.FindStringSubmatch(filename)
	version := matches[1]
	filenameDescription := matches[2]
	
	// Read file contents
	file, err := os.Open(filePath)
	if err != nil {
		return nil, NewFileSystemError(filePath, "open file", err)
	}
	defer file.Close()
	
	// Read SQL content
	sqlBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, NewFileSystemError(filePath, "read file", err)
	}
	
	sqlContent := string(sqlBytes)
	
	// Validate SQL content is not empty
	if strings.TrimSpace(sqlContent) == "" {
		return nil, NewMigrationError(version, filePath, "validate content", 
			fmt.Errorf("%w: migration file is empty", ErrInvalidMigrationFile))
	}
	
	// Extract description from file content (prefer file content over filename)
	description := fs.extractDescriptionFromContent(sqlContent)
	if description == "" {
		// Fallback to filename description if no description found in file
		description = strings.ReplaceAll(filenameDescription, "_", " ")
	}
	
	// Basic SQL syntax validation - check for common issues
	if err := fs.validateSQLSyntax(sqlContent); err != nil {
		return nil, NewMigrationError(version, filePath, "validate SQL syntax", err)
	}
	
	// Calculate checksum for file integrity
	checksum := fs.calculateChecksum(sqlContent)
	
	// Create migration object
	migration := &Migration{
		Version:     version,
		Description: description,
		SQL:         sqlContent,
		FilePath:    filePath,
		Checksum:    checksum,
	}
	
	return migration, nil
}

// validateSQLSyntax performs basic SQL syntax validation
func (fs *fileScannerImpl) validateSQLSyntax(sql string) error {
	// Remove comments and whitespace for validation
	cleanSQL := fs.cleanSQLForValidation(sql)
	
	// Check for empty SQL after cleaning
	if strings.TrimSpace(cleanSQL) == "" {
		return fmt.Errorf("%w: no SQL statements found after removing comments", ErrInvalidMigrationFile)
	}
	
	// Basic checks for common SQL syntax issues
	
	// Check for unmatched parentheses
	if err := fs.checkUnmatchedParentheses(cleanSQL); err != nil {
		return err
	}
	
	// Check for unterminated strings
	if err := fs.checkUnterminatedStrings(cleanSQL); err != nil {
		return err
	}
	
	return nil
}

// cleanSQLForValidation removes comments and normalizes whitespace
func (fs *fileScannerImpl) cleanSQLForValidation(sql string) string {
	lines := strings.Split(sql, "\n")
	var cleanLines []string
	
	for _, line := range lines {
		// Remove single-line comments (-- comments)
		if commentIndex := strings.Index(line, "--"); commentIndex != -1 {
			line = line[:commentIndex]
		}
		
		// Trim whitespace
		line = strings.TrimSpace(line)
		
		// Skip empty lines
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	
	return strings.Join(cleanLines, " ")
}

// checkUnmatchedParentheses validates parentheses are properly matched
func (fs *fileScannerImpl) checkUnmatchedParentheses(sql string) error {
	count := 0
	for _, char := range sql {
		switch char {
		case '(':
			count++
		case ')':
			count--
			if count < 0 {
				return fmt.Errorf("%w: unmatched closing parenthesis", ErrInvalidMigrationFile)
			}
		}
	}
	
	if count != 0 {
		return fmt.Errorf("%w: unmatched opening parenthesis", ErrInvalidMigrationFile)
	}
	
	return nil
}

// checkUnterminatedStrings validates string literals are properly terminated
func (fs *fileScannerImpl) checkUnterminatedStrings(sql string) error {
	inString := false
	var stringChar rune
	
	for i, char := range sql {
		switch char {
		case '\'', '"':
			if !inString {
				inString = true
				stringChar = char
			} else if char == stringChar {
				// Check if it's escaped (previous character is backslash)
				if i > 0 && rune(sql[i-1]) != '\\' {
					inString = false
				}
			}
		}
	}
	
	if inString {
		return fmt.Errorf("%w: unterminated string literal", ErrInvalidMigrationFile)
	}
	
	return nil
}

// extractDescriptionFromContent extracts description from migration file comments
func (fs *fileScannerImpl) extractDescriptionFromContent(content string) string {
	lines := strings.Split(content, "\n")
	
	// Look for description in various comment formats
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines
		if line == "" {
			continue
		}
		
		// Check for "-- Description: ..." format
		if strings.HasPrefix(line, "-- Description:") {
			description := strings.TrimSpace(strings.TrimPrefix(line, "-- Description:"))
			if description != "" {
				return description
			}
		}
		
		// Check for "-- Migration: filename" followed by "-- Description: ..." format
		if strings.HasPrefix(line, "-- Migration:") {
			// Look for description in the next few lines
			for i := 1; i < len(lines) && i < 5; i++ { // Check next 4 lines
				nextLine := strings.TrimSpace(lines[i])
				if strings.HasPrefix(nextLine, "-- Description:") {
					description := strings.TrimSpace(strings.TrimPrefix(nextLine, "-- Description:"))
					if description != "" {
						return description
					}
				}
				// Stop if we hit a non-comment line
				if !strings.HasPrefix(nextLine, "--") && nextLine != "" {
					break
				}
			}
		}
		
		// Stop processing if we hit the first non-comment line
		if !strings.HasPrefix(line, "--") {
			break
		}
	}
	
	return ""
}

// calculateChecksum calculates SHA256 checksum of the SQL content
func (fs *fileScannerImpl) calculateChecksum(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}