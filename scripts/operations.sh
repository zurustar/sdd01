#!/bin/bash

# Operations Management Script for Enterprise Scheduler
# This script provides a unified interface for all operational tasks

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

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
check_script_exists() {
    local script="$1"
    if [ ! -f "$SCRIPT_DIR/$script" ]; then
        error "Script not found: $SCRIPT_DIR/$script"
        return 1
    fi
    if [ ! -x "$SCRIPT_DIR/$script" ]; then
        error "Script not executable: $SCRIPT_DIR/$script"
        return 1
    fi
    return 0
}

run_script() {
    local script="$1"
    shift
    
    if check_script_exists "$script"; then
        log "Running $script $*"
        "$SCRIPT_DIR/$script" "$@"
    else
        return 1
    fi
}

# Operation functions
health_check() {
    log "Running comprehensive health check..."
    run_script "health-check.sh" "$@"
}

backup_operations() {
    local operation="${1:-backup}"
    shift
    
    case "$operation" in
        backup|create)
            log "Creating database backup..."
            run_script "backup-verify.sh" backup "$@"
            ;;
        verify)
            log "Verifying backup..."
            run_script "backup-verify.sh" verify "$@"
            ;;
        list)
            log "Listing available backups..."
            run_script "backup-verify.sh" list "$@"
            ;;
        restore)
            log "Restoring from backup..."
            run_script "backup-verify.sh" restore "$@"
            ;;
        cleanup)
            log "Cleaning up old backups..."
            run_script "backup-verify.sh" cleanup "$@"
            ;;
        *)
            error "Unknown backup operation: $operation"
            echo "Available operations: backup, verify, list, restore, cleanup"
            return 1
            ;;
    esac
}

monitoring_operations() {
    local operation="${1:-check}"
    shift
    
    case "$operation" in
        check)
            log "Running monitoring checks..."
            run_script "monitoring-alerts.sh" check "$@"
            ;;
        monitor)
            log "Starting continuous monitoring..."
            run_script "monitoring-alerts.sh" monitor "$@"
            ;;
        test-alert)
            log "Sending test alert..."
            run_script "monitoring-alerts.sh" test-alert "$@"
            ;;
        *)
            error "Unknown monitoring operation: $operation"
            echo "Available operations: check, monitor, test-alert"
            return 1
            ;;
    esac
}

documentation_operations() {
    local operation="${1:-validate}"
    shift
    
    case "$operation" in
        validate)
            log "Validating documentation..."
            run_script "doc-validation.sh" validate "$@"
            ;;
        traceability)
            log "Generating traceability matrix..."
            run_script "doc-validation.sh" traceability "$@"
            ;;
        *)
            error "Unknown documentation operation: $operation"
            echo "Available operations: validate, traceability"
            return 1
            ;;
    esac
}

testing_operations() {
    local operation="${1:-all}"
    shift
    
    case "$operation" in
        coverage)
            log "Running coverage check..."
            run_script "check-coverage.sh" "$@"
            ;;
        cgo-disabled)
            log "Testing with CGO disabled..."
            run_script "test-cgo-disabled.sh" "$@"
            ;;
        all)
            log "Running all tests..."
            cd "$PROJECT_ROOT"
            make test
            ;;
        *)
            error "Unknown testing operation: $operation"
            echo "Available operations: coverage, cgo-disabled, all"
            return 1
            ;;
    esac
}

# Comprehensive operations
full_health_check() {
    log "Running full system health check..."
    
    local failed_checks=0
    
    # Health check
    if ! health_check; then
        failed_checks=$((failed_checks + 1))
    fi
    
    # Backup verification
    if ! backup_operations list >/dev/null 2>&1; then
        warning "No backups found or backup system not working"
        failed_checks=$((failed_checks + 1))
    fi
    
    # Documentation validation
    if ! documentation_operations validate >/dev/null 2>&1; then
        warning "Documentation validation failed"
        failed_checks=$((failed_checks + 1))
    fi
    
    # Test coverage
    if ! testing_operations coverage >/dev/null 2>&1; then
        warning "Test coverage check failed"
        failed_checks=$((failed_checks + 1))
    fi
    
    if [ $failed_checks -eq 0 ]; then
        success "Full health check passed! 🎉"
    else
        warning "Full health check completed with $failed_checks issues"
    fi
    
    return $failed_checks
}

maintenance_mode() {
    local action="${1:-status}"
    local maintenance_file="/tmp/scheduler_maintenance"
    
    case "$action" in
        enable)
            log "Enabling maintenance mode..."
            echo "$(date)" > "$maintenance_file"
            success "Maintenance mode enabled"
            ;;
        disable)
            log "Disabling maintenance mode..."
            rm -f "$maintenance_file"
            success "Maintenance mode disabled"
            ;;
        status)
            if [ -f "$maintenance_file" ]; then
                local enabled_at=$(cat "$maintenance_file")
                warning "Maintenance mode is ENABLED (since $enabled_at)"
                return 1
            else
                success "Maintenance mode is DISABLED"
                return 0
            fi
            ;;
        *)
            error "Unknown maintenance action: $action"
            echo "Available actions: enable, disable, status"
            return 1
            ;;
    esac
}

# Status dashboard
show_status() {
    log "Enterprise Scheduler Operations Dashboard"
    echo "========================================"
    echo ""
    
    # System status
    echo "🖥️  System Status:"
    if pgrep -f "scheduler" > /dev/null; then
        echo "   ✅ Application: Running"
    else
        echo "   ❌ Application: Not running"
    fi
    
    # Maintenance mode
    echo -n "   "
    maintenance_mode status >/dev/null 2>&1 && echo "✅ Maintenance: Disabled" || echo "⚠️  Maintenance: Enabled"
    
    # Database
    if [ -f "scheduler.db" ]; then
        local db_size=$(du -h scheduler.db | cut -f1)
        echo "   ✅ Database: Available (${db_size})"
    else
        echo "   ❌ Database: Not found"
    fi
    
    # Backups
    local backup_count=$(find backups -name "scheduler_backup_*.db*" 2>/dev/null | wc -l)
    if [ $backup_count -gt 0 ]; then
        echo "   ✅ Backups: $backup_count available"
    else
        echo "   ⚠️  Backups: None found"
    fi
    
    # Disk space
    local disk_usage=$(df . | tail -1 | awk '{print $5}')
    echo "   📊 Disk Usage: $disk_usage"
    
    echo ""
    echo "📋 Available Operations:"
    echo "   health        - Run health checks"
    echo "   backup        - Backup operations"
    echo "   monitor       - Monitoring and alerts"
    echo "   docs          - Documentation validation"
    echo "   test          - Testing operations"
    echo "   maintenance   - Maintenance mode control"
    echo "   full-check    - Comprehensive health check"
    echo ""
}

# Main command dispatcher
main() {
    case "${1:-status}" in
        health)
            shift
            health_check "$@"
            ;;
        backup)
            shift
            backup_operations "$@"
            ;;
        monitor)
            shift
            monitoring_operations "$@"
            ;;
        docs)
            shift
            documentation_operations "$@"
            ;;
        test)
            shift
            testing_operations "$@"
            ;;
        maintenance)
            shift
            maintenance_mode "$@"
            ;;
        full-check)
            shift
            full_health_check "$@"
            ;;
        status)
            show_status
            ;;
        help|--help)
            echo "Enterprise Scheduler Operations Management"
            echo ""
            echo "Usage: $0 [operation] [options]"
            echo ""
            echo "Operations:"
            echo "  status              Show system status dashboard (default)"
            echo "  health [options]    Run health checks"
            echo "  backup [operation]  Backup operations (backup, verify, list, restore, cleanup)"
            echo "  monitor [operation] Monitoring operations (check, monitor, test-alert)"
            echo "  docs [operation]    Documentation operations (validate, traceability)"
            echo "  test [operation]    Testing operations (coverage, cgo-disabled, all)"
            echo "  maintenance [action] Maintenance mode (enable, disable, status)"
            echo "  full-check          Run comprehensive health check"
            echo "  help                Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                          # Show status dashboard"
            echo "  $0 health                   # Run health check"
            echo "  $0 backup create            # Create backup"
            echo "  $0 backup list              # List backups"
            echo "  $0 monitor check            # Run monitoring checks"
            echo "  $0 docs validate            # Validate documentation"
            echo "  $0 test coverage            # Check test coverage"
            echo "  $0 maintenance enable       # Enable maintenance mode"
            echo "  $0 full-check               # Run all health checks"
            echo ""
            echo "Environment variables:"
            echo "  See individual script help for specific configuration options"
            ;;
        *)
            error "Unknown operation: $1"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Change to project root directory
cd "$PROJECT_ROOT"

# Run main function
main "$@"