#!/bin/bash

# Health Check Script for Enterprise Scheduler
# This script performs comprehensive health checks on the running application

set -e

# Configuration
HEALTH_ENDPOINT="${HEALTH_ENDPOINT:-http://localhost:8080/health}"
TIMEOUT="${TIMEOUT:-10}"
RETRY_COUNT="${RETRY_COUNT:-3}"
RETRY_DELAY="${RETRY_DELAY:-5}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
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

# Health check functions
check_http_endpoint() {
    local endpoint=$1
    local expected_status=${2:-200}
    
    log "Checking HTTP endpoint: $endpoint"
    
    for i in $(seq 1 $RETRY_COUNT); do
        if response=$(curl -s -w "%{http_code}" --max-time $TIMEOUT "$endpoint" 2>/dev/null); then
            http_code="${response: -3}"
            body="${response%???}"
            
            if [ "$http_code" = "$expected_status" ]; then
                success "HTTP endpoint $endpoint returned $http_code"
                echo "Response body: $body"
                return 0
            else
                warning "HTTP endpoint $endpoint returned $http_code (expected $expected_status)"
            fi
        else
            warning "Failed to connect to $endpoint (attempt $i/$RETRY_COUNT)"
        fi
        
        if [ $i -lt $RETRY_COUNT ]; then
            log "Retrying in $RETRY_DELAY seconds..."
            sleep $RETRY_DELAY
        fi
    done
    
    error "HTTP endpoint $endpoint failed after $RETRY_COUNT attempts"
    return 1
}

check_database_connection() {
    log "Checking database connection..."
    
    # Check if database file exists and is accessible
    if [ -f "scheduler.db" ]; then
        success "Database file exists: scheduler.db"
        
        # Check database integrity
        if sqlite3 scheduler.db "PRAGMA integrity_check;" | grep -q "ok"; then
            success "Database integrity check passed"
        else
            error "Database integrity check failed"
            return 1
        fi
        
        # Check if essential tables exist
        tables=("users" "rooms" "schedules" "sessions" "schema_migrations")
        for table in "${tables[@]}"; do
            if sqlite3 scheduler.db ".tables" | grep -q "$table"; then
                success "Table '$table' exists"
            else
                error "Table '$table' is missing"
                return 1
            fi
        done
    else
        error "Database file not found: scheduler.db"
        return 1
    fi
}

check_disk_space() {
    log "Checking disk space..."
    
    # Check available disk space (warn if less than 1GB)
    available_space=$(df . | tail -1 | awk '{print $4}')
    available_gb=$((available_space / 1024 / 1024))
    
    if [ $available_gb -lt 1 ]; then
        warning "Low disk space: ${available_gb}GB available"
        return 1
    else
        success "Sufficient disk space: ${available_gb}GB available"
    fi
}

check_memory_usage() {
    log "Checking memory usage..."
    
    # Check if the scheduler process is running and get memory usage
    if pgrep -f "scheduler" > /dev/null; then
        memory_usage=$(ps -o pid,rss,comm -p $(pgrep -f "scheduler") | tail -n +2)
        success "Scheduler process is running:"
        echo "$memory_usage"
        
        # Check if memory usage is reasonable (warn if > 500MB)
        memory_kb=$(echo "$memory_usage" | awk '{print $2}')
        memory_mb=$((memory_kb / 1024))
        
        if [ $memory_mb -gt 500 ]; then
            warning "High memory usage: ${memory_mb}MB"
        else
            success "Memory usage is normal: ${memory_mb}MB"
        fi
    else
        error "Scheduler process is not running"
        return 1
    fi
}

check_log_files() {
    log "Checking log files..."
    
    # Check if log directory exists and has recent entries
    if [ -d "logs" ]; then
        success "Log directory exists"
        
        # Check for recent log entries (within last hour)
        recent_logs=$(find logs -name "*.log" -mmin -60 2>/dev/null | wc -l)
        if [ $recent_logs -gt 0 ]; then
            success "Found $recent_logs recent log files"
        else
            warning "No recent log files found"
        fi
        
        # Check log file sizes (warn if any log > 100MB)
        large_logs=$(find logs -name "*.log" -size +100M 2>/dev/null)
        if [ -n "$large_logs" ]; then
            warning "Large log files found:"
            echo "$large_logs"
        fi
    else
        warning "Log directory not found"
    fi
}

check_configuration() {
    log "Checking configuration..."
    
    # Check if configuration file exists
    config_files=("config.yml" "config.yaml" ".env")
    config_found=false
    
    for config in "${config_files[@]}"; do
        if [ -f "$config" ]; then
            success "Configuration file found: $config"
            config_found=true
            break
        fi
    done
    
    if [ "$config_found" = false ]; then
        warning "No configuration file found (using defaults)"
    fi
    
    # Check environment variables
    required_vars=("DATABASE_PATH" "PORT")
    for var in "${required_vars[@]}"; do
        if [ -n "${!var}" ]; then
            success "Environment variable $var is set"
        else
            warning "Environment variable $var is not set"
        fi
    done
}

# Main health check execution
main() {
    log "Starting Enterprise Scheduler health check..."
    echo "=============================================="
    
    local exit_code=0
    
    # Run all health checks
    check_http_endpoint "$HEALTH_ENDPOINT" || exit_code=1
    echo ""
    
    check_database_connection || exit_code=1
    echo ""
    
    check_disk_space || exit_code=1
    echo ""
    
    check_memory_usage || exit_code=1
    echo ""
    
    check_log_files || exit_code=1
    echo ""
    
    check_configuration || exit_code=1
    echo ""
    
    # Summary
    echo "=============================================="
    if [ $exit_code -eq 0 ]; then
        success "All health checks passed! 🎉"
    else
        error "Some health checks failed! ❌"
        echo ""
        echo "Troubleshooting steps:"
        echo "1. Check application logs for errors"
        echo "2. Verify database connectivity and integrity"
        echo "3. Ensure sufficient system resources"
        echo "4. Review configuration settings"
    fi
    
    exit $exit_code
}

# Handle script arguments
case "${1:-}" in
    --endpoint)
        HEALTH_ENDPOINT="$2"
        shift 2
        ;;
    --timeout)
        TIMEOUT="$2"
        shift 2
        ;;
    --help)
        echo "Usage: $0 [options]"
        echo ""
        echo "Options:"
        echo "  --endpoint URL    Health check endpoint (default: $HEALTH_ENDPOINT)"
        echo "  --timeout SECONDS Request timeout (default: $TIMEOUT)"
        echo "  --help           Show this help message"
        echo ""
        echo "Environment variables:"
        echo "  HEALTH_ENDPOINT  Health check endpoint URL"
        echo "  TIMEOUT          Request timeout in seconds"
        echo "  RETRY_COUNT      Number of retry attempts"
        echo "  RETRY_DELAY      Delay between retries in seconds"
        exit 0
        ;;
esac

# Run main function
main "$@"