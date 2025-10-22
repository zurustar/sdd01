#!/bin/bash

# Backup Verification Script for Enterprise Scheduler
# This script creates, verifies, and manages database backups

set -e

# Configuration
DATABASE_PATH="${DATABASE_PATH:-scheduler.db}"
BACKUP_DIR="${BACKUP_DIR:-backups}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
VERIFY_BACKUP="${VERIFY_BACKUP:-true}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] ERROR:${NC} $1" >&2
}

success() {
    echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')] SUCCESS:${NC} $1"
}

warning() {
    echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')] WARNING:${NC} $1"
}

# Utility functions
check_dependencies() {
    log "Checking dependencies..."
    
    if ! command -v sqlite3 >/dev/null 2>&1; then
        error "sqlite3 is required but not installed"
        exit 1
    fi
    
    if ! command -v gzip >/dev/null 2>&1; then
        error "gzip is required but not installed"
        exit 1
    fi
    
    success "All dependencies are available"
}

create_backup_dir() {
    if [ ! -d "$BACKUP_DIR" ]; then
        log "Creating backup directory: $BACKUP_DIR"
        mkdir -p "$BACKUP_DIR"
        success "Backup directory created"
    fi
}

# Backup functions
create_backup() {
    local timestamp=$(date '+%Y%m%d_%H%M%S')
    local backup_file="$BACKUP_DIR/scheduler_backup_$timestamp.db"
    local compressed_backup="$backup_file.gz"
    
    log "Creating backup of $DATABASE_PATH..."
    
    # Check if source database exists
    if [ ! -f "$DATABASE_PATH" ]; then
        error "Source database not found: $DATABASE_PATH"
        return 1
    fi
    
    # Create backup using SQLite's backup command
    if sqlite3 "$DATABASE_PATH" ".backup $backup_file"; then
        success "Backup created: $backup_file"
        
        # Compress the backup
        log "Compressing backup..."
        if gzip "$backup_file"; then
            success "Backup compressed: $compressed_backup"
            echo "$compressed_backup"
        else
            error "Failed to compress backup"
            return 1
        fi
    else
        error "Failed to create backup"
        return 1
    fi
}

verify_backup() {
    local backup_file="$1"
    
    log "Verifying backup: $backup_file"
    
    # Decompress if needed
    local temp_backup="$backup_file"
    if [[ "$backup_file" == *.gz ]]; then
        temp_backup="${backup_file%.gz}"
        log "Decompressing backup for verification..."
        gunzip -c "$backup_file" > "$temp_backup"
    fi
    
    # Verify database integrity
    if sqlite3 "$temp_backup" "PRAGMA integrity_check;" | grep -q "ok"; then
        success "Backup integrity check passed"
    else
        error "Backup integrity check failed"
        [ "$temp_backup" != "$backup_file" ] && rm -f "$temp_backup"
        return 1
    fi
    
    # Verify essential tables exist
    local tables=("users" "rooms" "schedules" "sessions" "schema_migrations")
    for table in "${tables[@]}"; do
        if sqlite3 "$temp_backup" ".tables" | grep -q "$table"; then
            success "Table '$table' exists in backup"
        else
            error "Table '$table' is missing from backup"
            [ "$temp_backup" != "$backup_file" ] && rm -f "$temp_backup"
            return 1
        fi
    done
    
    # Verify data counts
    log "Verifying data consistency..."
    local user_count=$(sqlite3 "$temp_backup" "SELECT COUNT(*) FROM users;")
    local room_count=$(sqlite3 "$temp_backup" "SELECT COUNT(*) FROM rooms;")
    local schedule_count=$(sqlite3 "$temp_backup" "SELECT COUNT(*) FROM schedules;")
    
    success "Backup contains: $user_count users, $room_count rooms, $schedule_count schedules"
    
    # Compare with original database if available
    if [ -f "$DATABASE_PATH" ]; then
        local orig_user_count=$(sqlite3 "$DATABASE_PATH" "SELECT COUNT(*) FROM users;")
        local orig_room_count=$(sqlite3 "$DATABASE_PATH" "SELECT COUNT(*) FROM rooms;")
        local orig_schedule_count=$(sqlite3 "$DATABASE_PATH" "SELECT COUNT(*) FROM schedules;")
        
        if [ "$user_count" = "$orig_user_count" ] && [ "$room_count" = "$orig_room_count" ] && [ "$schedule_count" = "$orig_schedule_count" ]; then
            success "Backup data counts match original database"
        else
            warning "Backup data counts differ from original database"
            log "Original: $orig_user_count users, $orig_room_count rooms, $orig_schedule_count schedules"
            log "Backup:   $user_count users, $room_count rooms, $schedule_count schedules"
        fi
    fi
    
    # Clean up temporary file
    [ "$temp_backup" != "$backup_file" ] && rm -f "$temp_backup"
    
    success "Backup verification completed successfully"
}

list_backups() {
    log "Listing available backups in $BACKUP_DIR:"
    
    if [ ! -d "$BACKUP_DIR" ]; then
        warning "Backup directory does not exist: $BACKUP_DIR"
        return 1
    fi
    
    local backups=$(find "$BACKUP_DIR" -name "scheduler_backup_*.db*" -type f | sort -r)
    
    if [ -z "$backups" ]; then
        warning "No backups found in $BACKUP_DIR"
        return 1
    fi
    
    echo ""
    printf "%-30s %-15s %-10s\n" "Backup File" "Date" "Size"
    echo "--------------------------------------------------------"
    
    for backup in $backups; do
        local filename=$(basename "$backup")
        local filesize=$(du -h "$backup" | cut -f1)
        local timestamp=$(echo "$filename" | sed 's/scheduler_backup_\(.*\)\.db.*/\1/')
        local formatted_date=$(echo "$timestamp" | sed 's/\([0-9]\{4\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)_\([0-9]\{2\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)/\1-\2-\3 \4:\5:\6/')
        
        printf "%-30s %-15s %-10s\n" "$filename" "$formatted_date" "$filesize"
    done
    echo ""
}

cleanup_old_backups() {
    log "Cleaning up backups older than $RETENTION_DAYS days..."
    
    if [ ! -d "$BACKUP_DIR" ]; then
        warning "Backup directory does not exist: $BACKUP_DIR"
        return 0
    fi
    
    local old_backups=$(find "$BACKUP_DIR" -name "scheduler_backup_*.db*" -type f -mtime +$RETENTION_DAYS)
    
    if [ -z "$old_backups" ]; then
        success "No old backups to clean up"
        return 0
    fi
    
    local count=0
    for backup in $old_backups; do
        log "Removing old backup: $(basename "$backup")"
        rm -f "$backup"
        count=$((count + 1))
    done
    
    success "Cleaned up $count old backup(s)"
}

restore_backup() {
    local backup_file="$1"
    local target_db="${2:-$DATABASE_PATH.restored}"
    
    log "Restoring backup: $backup_file to $target_db"
    
    if [ ! -f "$backup_file" ]; then
        error "Backup file not found: $backup_file"
        return 1
    fi
    
    # Decompress if needed
    local temp_backup="$backup_file"
    if [[ "$backup_file" == *.gz ]]; then
        temp_backup="/tmp/$(basename "${backup_file%.gz}")"
        log "Decompressing backup..."
        gunzip -c "$backup_file" > "$temp_backup"
    fi
    
    # Copy the backup to target location
    if cp "$temp_backup" "$target_db"; then
        success "Backup restored to: $target_db"
        
        # Verify the restored database
        if verify_backup "$target_db"; then
            success "Restored database verification passed"
        else
            error "Restored database verification failed"
            return 1
        fi
    else
        error "Failed to restore backup"
        return 1
    fi
    
    # Clean up temporary file
    [ "$temp_backup" != "$backup_file" ] && rm -f "$temp_backup"
}

# Main functions
backup_and_verify() {
    log "Starting backup and verification process..."
    
    check_dependencies
    create_backup_dir
    
    # Create backup
    local backup_file
    if backup_file=$(create_backup); then
        success "Backup created successfully"
        
        # Verify backup if requested
        if [ "$VERIFY_BACKUP" = "true" ]; then
            if verify_backup "$backup_file"; then
                success "Backup verification passed"
            else
                error "Backup verification failed"
                return 1
            fi
        fi
        
        # Cleanup old backups
        cleanup_old_backups
        
        success "Backup process completed successfully"
        echo "Backup file: $backup_file"
    else
        error "Backup process failed"
        return 1
    fi
}

# Command line interface
case "${1:-backup}" in
    backup)
        backup_and_verify
        ;;
    verify)
        if [ -z "$2" ]; then
            error "Please specify a backup file to verify"
            echo "Usage: $0 verify <backup_file>"
            exit 1
        fi
        verify_backup "$2"
        ;;
    list)
        list_backups
        ;;
    restore)
        if [ -z "$2" ]; then
            error "Please specify a backup file to restore"
            echo "Usage: $0 restore <backup_file> [target_db]"
            exit 1
        fi
        restore_backup "$2" "$3"
        ;;
    cleanup)
        cleanup_old_backups
        ;;
    help|--help)
        echo "Enterprise Scheduler Backup Verification Script"
        echo ""
        echo "Usage: $0 [command] [options]"
        echo ""
        echo "Commands:"
        echo "  backup          Create and verify a new backup (default)"
        echo "  verify <file>   Verify an existing backup file"
        echo "  list            List all available backups"
        echo "  restore <file>  Restore a backup to a new database file"
        echo "  cleanup         Remove old backups based on retention policy"
        echo "  help            Show this help message"
        echo ""
        echo "Environment variables:"
        echo "  DATABASE_PATH   Path to the source database (default: scheduler.db)"
        echo "  BACKUP_DIR      Directory for backups (default: backups)"
        echo "  RETENTION_DAYS  Days to keep backups (default: 7)"
        echo "  VERIFY_BACKUP   Verify backups after creation (default: true)"
        echo ""
        echo "Examples:"
        echo "  $0                                    # Create and verify backup"
        echo "  $0 verify backups/scheduler_backup_20231201_120000.db.gz"
        echo "  $0 restore backups/scheduler_backup_20231201_120000.db.gz"
        echo "  $0 list                               # List all backups"
        echo "  $0 cleanup                            # Remove old backups"
        ;;
    *)
        error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac