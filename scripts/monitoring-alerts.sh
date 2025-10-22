#!/bin/bash

# Monitoring and Alerting Script for Enterprise Scheduler
# This script monitors system health and sends alerts when issues are detected

set -e

# Configuration
ALERT_EMAIL="${ALERT_EMAIL:-admin@example.com}"
ALERT_WEBHOOK="${ALERT_WEBHOOK:-}"
SLACK_WEBHOOK="${SLACK_WEBHOOK:-}"
ALERT_THRESHOLD_CPU="${ALERT_THRESHOLD_CPU:-80}"
ALERT_THRESHOLD_MEMORY="${ALERT_THRESHOLD_MEMORY:-80}"
ALERT_THRESHOLD_DISK="${ALERT_THRESHOLD_DISK:-90}"
ALERT_THRESHOLD_RESPONSE_TIME="${ALERT_THRESHOLD_RESPONSE_TIME:-5000}"
CHECK_INTERVAL="${CHECK_INTERVAL:-300}"
LOG_FILE="${LOG_FILE:-logs/monitoring.log}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log() {
    local message="[$(date '+%Y-%m-%d %H:%M:%S')] $1"
    echo -e "${BLUE}$message${NC}"
    echo "$message" >> "$LOG_FILE"
}

error() {
    local message="[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1"
    echo -e "${RED}$message${NC}" >&2
    echo "$message" >> "$LOG_FILE"
}

warning() {
    local message="[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: $1"
    echo -e "${YELLOW}$message${NC}"
    echo "$message" >> "$LOG_FILE"
}

success() {
    local message="[$(date '+%Y-%m-%d %H:%M:%S')] SUCCESS: $1"
    echo -e "${GREEN}$message${NC}"
    echo "$message" >> "$LOG_FILE"
}

# Alert functions
send_email_alert() {
    local subject="$1"
    local body="$2"
    
    if [ -n "$ALERT_EMAIL" ] && command -v mail >/dev/null 2>&1; then
        echo "$body" | mail -s "$subject" "$ALERT_EMAIL"
        log "Email alert sent to $ALERT_EMAIL"
    else
        warning "Email alerts not configured or mail command not available"
    fi
}

send_webhook_alert() {
    local title="$1"
    local message="$2"
    local severity="$3"
    
    if [ -n "$ALERT_WEBHOOK" ]; then
        local payload=$(cat <<EOF
{
    "title": "$title",
    "message": "$message",
    "severity": "$severity",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "service": "enterprise-scheduler"
}
EOF
)
        
        if curl -s -X POST -H "Content-Type: application/json" -d "$payload" "$ALERT_WEBHOOK" >/dev/null; then
            log "Webhook alert sent"
        else
            warning "Failed to send webhook alert"
        fi
    fi
}

send_slack_alert() {
    local message="$1"
    local color="$2"
    
    if [ -n "$SLACK_WEBHOOK" ]; then
        local payload=$(cat <<EOF
{
    "attachments": [
        {
            "color": "$color",
            "title": "Enterprise Scheduler Alert",
            "text": "$message",
            "ts": $(date +%s)
        }
    ]
}
EOF
)
        
        if curl -s -X POST -H "Content-Type: application/json" -d "$payload" "$SLACK_WEBHOOK" >/dev/null; then
            log "Slack alert sent"
        else
            warning "Failed to send Slack alert"
        fi
    fi
}

send_alert() {
    local title="$1"
    local message="$2"
    local severity="${3:-warning}"
    
    log "Sending alert: $title"
    
    # Determine colors based on severity
    local slack_color="warning"
    case "$severity" in
        critical) slack_color="danger" ;;
        warning) slack_color="warning" ;;
        info) slack_color="good" ;;
    esac
    
    # Send alerts through all configured channels
    send_email_alert "$title" "$message"
    send_webhook_alert "$title" "$message" "$severity"
    send_slack_alert "$message" "$slack_color"
}

# Monitoring functions
check_application_health() {
    local health_endpoint="${HEALTH_ENDPOINT:-http://localhost:8080/health}"
    
    log "Checking application health..."
    
    local start_time=$(date +%s%3N)
    local response=$(curl -s -w "%{http_code}" --max-time 10 "$health_endpoint" 2>/dev/null || echo "000")
    local end_time=$(date +%s%3N)
    local response_time=$((end_time - start_time))
    
    local http_code="${response: -3}"
    local body="${response%???}"
    
    if [ "$http_code" != "200" ]; then
        send_alert "Application Health Check Failed" \
                  "Health endpoint $health_endpoint returned HTTP $http_code. Response: $body" \
                  "critical"
        return 1
    fi
    
    if [ $response_time -gt $ALERT_THRESHOLD_RESPONSE_TIME ]; then
        send_alert "Slow Response Time" \
                  "Health endpoint response time: ${response_time}ms (threshold: ${ALERT_THRESHOLD_RESPONSE_TIME}ms)" \
                  "warning"
    fi
    
    success "Application health check passed (${response_time}ms)"
}

check_database_health() {
    local db_path="${DATABASE_PATH:-scheduler.db}"
    
    log "Checking database health..."
    
    if [ ! -f "$db_path" ]; then
        send_alert "Database File Missing" \
                  "Database file not found: $db_path" \
                  "critical"
        return 1
    fi
    
    # Check database integrity
    if ! sqlite3 "$db_path" "PRAGMA integrity_check;" | grep -q "ok"; then
        send_alert "Database Integrity Check Failed" \
                  "Database integrity check failed for $db_path" \
                  "critical"
        return 1
    fi
    
    # Check database size growth
    local db_size=$(du -m "$db_path" | cut -f1)
    if [ $db_size -gt 1000 ]; then  # Alert if database > 1GB
        send_alert "Large Database Size" \
                  "Database size is ${db_size}MB, consider maintenance" \
                  "warning"
    fi
    
    success "Database health check passed (${db_size}MB)"
}

check_system_resources() {
    log "Checking system resources..."
    
    # Check CPU usage
    local cpu_usage=$(top -l 1 -n 0 | grep "CPU usage" | awk '{print $3}' | sed 's/%//' 2>/dev/null || echo "0")
    if (( $(echo "$cpu_usage > $ALERT_THRESHOLD_CPU" | bc -l 2>/dev/null || echo 0) )); then
        send_alert "High CPU Usage" \
                  "CPU usage is ${cpu_usage}% (threshold: ${ALERT_THRESHOLD_CPU}%)" \
                  "warning"
    fi
    
    # Check memory usage
    local memory_info=$(vm_stat 2>/dev/null || free 2>/dev/null)
    if [ -n "$memory_info" ]; then
        # This is a simplified check - in production, you'd want more sophisticated memory monitoring
        success "Memory check completed"
    fi
    
    # Check disk space
    local disk_usage=$(df . | tail -1 | awk '{print $5}' | sed 's/%//')
    if [ $disk_usage -gt $ALERT_THRESHOLD_DISK ]; then
        send_alert "High Disk Usage" \
                  "Disk usage is ${disk_usage}% (threshold: ${ALERT_THRESHOLD_DISK}%)" \
                  "critical"
    fi
    
    success "System resource check completed (CPU: ${cpu_usage}%, Disk: ${disk_usage}%)"
}

check_log_errors() {
    log "Checking for recent errors in logs..."
    
    local log_dir="logs"
    if [ ! -d "$log_dir" ]; then
        warning "Log directory not found: $log_dir"
        return 0
    fi
    
    # Check for errors in the last 5 minutes
    local recent_errors=$(find "$log_dir" -name "*.log" -mmin -5 -exec grep -l "ERROR\|FATAL\|PANIC" {} \; 2>/dev/null)
    
    if [ -n "$recent_errors" ]; then
        local error_count=$(find "$log_dir" -name "*.log" -mmin -5 -exec grep -c "ERROR\|FATAL\|PANIC" {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}')
        
        if [ "$error_count" -gt 10 ]; then
            send_alert "High Error Rate" \
                      "Found $error_count errors in logs in the last 5 minutes" \
                      "warning"
        fi
    fi
    
    success "Log error check completed"
}

check_process_status() {
    log "Checking process status..."
    
    if ! pgrep -f "scheduler" > /dev/null; then
        send_alert "Application Process Not Running" \
                  "The scheduler process is not running" \
                  "critical"
        return 1
    fi
    
    # Check if process is consuming too much memory
    local memory_usage=$(ps -o rss -p $(pgrep -f "scheduler") | tail -n +2 | awk '{sum+=$1} END {print sum}')
    local memory_mb=$((memory_usage / 1024))
    
    if [ $memory_mb -gt 1000 ]; then  # Alert if > 1GB
        send_alert "High Memory Usage" \
                  "Application is using ${memory_mb}MB of memory" \
                  "warning"
    fi
    
    success "Process status check passed (${memory_mb}MB memory)"
}

# Main monitoring loop
run_monitoring_checks() {
    log "Starting monitoring checks..."
    
    local failed_checks=0
    
    check_application_health || failed_checks=$((failed_checks + 1))
    check_database_health || failed_checks=$((failed_checks + 1))
    check_system_resources || failed_checks=$((failed_checks + 1))
    check_log_errors || failed_checks=$((failed_checks + 1))
    check_process_status || failed_checks=$((failed_checks + 1))
    
    if [ $failed_checks -eq 0 ]; then
        success "All monitoring checks passed"
    else
        warning "$failed_checks monitoring checks failed"
    fi
    
    return $failed_checks
}

# Continuous monitoring mode
continuous_monitoring() {
    log "Starting continuous monitoring (interval: ${CHECK_INTERVAL}s)"
    
    # Ensure log directory exists
    mkdir -p "$(dirname "$LOG_FILE")"
    
    while true; do
        run_monitoring_checks
        log "Sleeping for $CHECK_INTERVAL seconds..."
        sleep $CHECK_INTERVAL
    done
}

# Command line interface
case "${1:-check}" in
    check)
        run_monitoring_checks
        ;;
    monitor)
        continuous_monitoring
        ;;
    test-alert)
        send_alert "Test Alert" "This is a test alert from the monitoring system" "info"
        ;;
    help|--help)
        echo "Enterprise Scheduler Monitoring and Alerting Script"
        echo ""
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  check        Run monitoring checks once (default)"
        echo "  monitor      Run continuous monitoring"
        echo "  test-alert   Send a test alert"
        echo "  help         Show this help message"
        echo ""
        echo "Environment variables:"
        echo "  ALERT_EMAIL                    Email address for alerts"
        echo "  ALERT_WEBHOOK                  Webhook URL for alerts"
        echo "  SLACK_WEBHOOK                  Slack webhook URL"
        echo "  ALERT_THRESHOLD_CPU            CPU usage threshold (default: 80)"
        echo "  ALERT_THRESHOLD_MEMORY         Memory usage threshold (default: 80)"
        echo "  ALERT_THRESHOLD_DISK           Disk usage threshold (default: 90)"
        echo "  ALERT_THRESHOLD_RESPONSE_TIME  Response time threshold in ms (default: 5000)"
        echo "  CHECK_INTERVAL                 Check interval in seconds (default: 300)"
        echo "  LOG_FILE                       Log file path (default: logs/monitoring.log)"
        echo ""
        echo "Examples:"
        echo "  $0                             # Run checks once"
        echo "  $0 monitor                     # Run continuous monitoring"
        echo "  ALERT_EMAIL=admin@example.com $0 test-alert"
        ;;
    *)
        error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac