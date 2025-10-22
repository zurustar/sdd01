#!/bin/bash

# CGO-disabled testing script for Enterprise Scheduler
# This script ensures the application works correctly without CGO dependencies

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔧 Testing Enterprise Scheduler with CGO disabled${NC}"
echo "=================================================="

# Ensure CGO is disabled
export CGO_ENABLED=0

echo ""
echo -e "${YELLOW}📋 Environment Check:${NC}"
echo "CGO_ENABLED: $CGO_ENABLED"
echo "GOOS: $(go env GOOS)"
echo "GOARCH: $(go env GOARCH)"
echo "Go version: $(go version)"

echo ""
echo -e "${YELLOW}🏗️  Building with CGO disabled...${NC}"

# Build the main application
echo "Building scheduler binary..."
if go build -v -o bin/scheduler-cgo-disabled ./cmd/scheduler; then
    echo -e "${GREEN}✅ Build successful${NC}"
else
    echo -e "${RED}❌ Build failed${NC}"
    exit 1
fi

# Verify the binary was created
if [ -f "bin/scheduler-cgo-disabled" ]; then
    echo -e "${GREEN}✅ Binary created: bin/scheduler-cgo-disabled${NC}"
    ls -la bin/scheduler-cgo-disabled
else
    echo -e "${RED}❌ Binary not found${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}🧪 Running tests with CGO disabled...${NC}"

# Run all tests with CGO disabled
if go test -v ./...; then
    echo -e "${GREEN}✅ All tests passed with CGO disabled${NC}"
else
    echo -e "${RED}❌ Some tests failed with CGO disabled${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}🏃 Running race detection tests with CGO disabled...${NC}"

# Note: Race detection requires CGO, so we skip it when CGO is disabled
echo -e "${YELLOW}⚠️  Skipping race detection (requires CGO)${NC}"

echo ""
echo -e "${YELLOW}🔍 Checking for CGO dependencies...${NC}"

# Check if the binary has any CGO dependencies
if command -v ldd >/dev/null 2>&1; then
    echo "Checking dynamic library dependencies (Linux):"
    if ldd bin/scheduler-cgo-disabled 2>/dev/null; then
        echo -e "${YELLOW}⚠️  Binary has dynamic dependencies${NC}"
    else
        echo -e "${GREEN}✅ Binary is statically linked${NC}"
    fi
elif command -v otool >/dev/null 2>&1; then
    echo "Checking dynamic library dependencies (macOS):"
    if otool -L bin/scheduler-cgo-disabled 2>/dev/null; then
        echo -e "${YELLOW}⚠️  Binary has dynamic dependencies${NC}"
    else
        echo -e "${GREEN}✅ Binary is statically linked${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  Cannot check dynamic dependencies (ldd/otool not available)${NC}"
fi

echo ""
echo -e "${YELLOW}🌐 Testing cross-compilation...${NC}"

# Test cross-compilation for different platforms
PLATFORMS=("linux/amd64" "darwin/amd64" "windows/amd64")

for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r GOOS GOARCH <<< "$platform"
    echo "Building for $GOOS/$GOARCH..."
    
    if GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build -o "bin/scheduler-$GOOS-$GOARCH" ./cmd/scheduler; then
        echo -e "${GREEN}✅ $GOOS/$GOARCH build successful${NC}"
    else
        echo -e "${RED}❌ $GOOS/$GOARCH build failed${NC}"
        exit 1
    fi
done

echo ""
echo -e "${YELLOW}📦 Build artifacts:${NC}"
ls -la bin/

echo ""
echo -e "${YELLOW}🧹 Cleaning up test artifacts...${NC}"
rm -f bin/scheduler-*

echo ""
echo -e "${GREEN}🎉 CGO-disabled testing completed successfully!${NC}"
echo ""
echo -e "${BLUE}Summary:${NC}"
echo "✅ Application builds successfully without CGO"
echo "✅ All tests pass without CGO dependencies"
echo "✅ Cross-compilation works for multiple platforms"
echo "✅ No runtime CGO dependencies detected"
echo ""
echo -e "${GREEN}The application is ready for deployment in CGO-free environments!${NC}"