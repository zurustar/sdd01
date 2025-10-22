#!/bin/bash

# Coverage threshold check script for Enterprise Scheduler
# This script runs tests with coverage and checks if the coverage meets the minimum threshold

set -e

# Configuration
COVERAGE_FILE="coverage.out"
THRESHOLD=${COVERAGE_THRESHOLD:-80}
EXCLUDE_PATTERNS=${COVERAGE_EXCLUDE:-""}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🧪 Running tests with coverage..."

# Run tests with coverage
go test -v -coverprofile="$COVERAGE_FILE" ./...

if [ ! -f "$COVERAGE_FILE" ]; then
    echo -e "${RED}❌ Coverage file not found: $COVERAGE_FILE${NC}"
    exit 1
fi

echo ""
echo "📊 Coverage Analysis:"
echo "===================="

# Get total coverage percentage
COVERAGE=$(go tool cover -func="$COVERAGE_FILE" | grep total | awk '{print $3}' | sed 's/%//')

# Display coverage by package
echo ""
echo "Coverage by package:"
go tool cover -func="$COVERAGE_FILE" | grep -v "total:" | while read line; do
    PACKAGE=$(echo "$line" | awk '{print $1}' | cut -d'/' -f1-3)
    FUNC=$(echo "$line" | awk '{print $2}')
    COV=$(echo "$line" | awk '{print $3}')
    printf "  %-50s %s\n" "$PACKAGE/$FUNC" "$COV"
done

echo ""
echo "📈 Overall Coverage Summary:"
echo "============================"
echo -e "Current coverage: ${YELLOW}${COVERAGE}%${NC}"
echo -e "Required threshold: ${YELLOW}${THRESHOLD}%${NC}"

# Check if coverage meets threshold
if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
    echo ""
    echo -e "${RED}❌ COVERAGE CHECK FAILED${NC}"
    echo -e "${RED}Coverage ${COVERAGE}% is below the required threshold of ${THRESHOLD}%${NC}"
    echo ""
    echo "💡 To improve coverage:"
    echo "  1. Add more unit tests for uncovered functions"
    echo "  2. Add integration tests for complex workflows"
    echo "  3. Review the coverage report: go tool cover -html=$COVERAGE_FILE"
    echo ""
    
    # Generate HTML report for detailed analysis
    echo "📋 Generating detailed HTML coverage report..."
    go tool cover -html="$COVERAGE_FILE" -o coverage.html
    echo "   Report saved to: coverage.html"
    
    exit 1
else
    echo ""
    echo -e "${GREEN}✅ COVERAGE CHECK PASSED${NC}"
    echo -e "${GREEN}Coverage ${COVERAGE}% meets the required threshold of ${THRESHOLD}%${NC}"
    echo ""
fi

# Optional: Generate HTML report for successful runs too
if [ "${GENERATE_HTML:-false}" = "true" ]; then
    echo "📋 Generating HTML coverage report..."
    go tool cover -html="$COVERAGE_FILE" -o coverage.html
    echo "   Report saved to: coverage.html"
fi

# Clean up
if [ "${KEEP_COVERAGE_FILE:-false}" != "true" ]; then
    rm -f "$COVERAGE_FILE"
fi

echo "🎉 Coverage check completed successfully!"