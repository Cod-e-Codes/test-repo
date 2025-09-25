#!/bin/bash

# Marchat Test Runner
# This script runs all tests in the Marchat project

set -e

echo "Running Marchat Test Suite"
echo "================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}$1${NC}"
}

print_warning() {
    echo -e "${YELLOW}$1${NC}"
}

print_error() {
    echo -e "${RED}$1${NC}"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version)
print_status "Using Go: $GO_VERSION"

# Run tests with verbose output
echo ""
echo "Running tests..."
echo "================="

# Run all tests
if go test ./...; then
    echo ""
    print_status "All tests passed!"
    
    # Run test coverage
    echo ""
    echo "Generating test coverage report..."
    echo "=================================="
    
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    
    print_status "Coverage report generated: coverage.html"
    
    # Show coverage summary
    echo ""
    echo "Coverage Summary:"
    echo "================="
    go tool cover -func=coverage.out | tail -1
    
    echo ""
    print_status "Test suite completed successfully!"
    
else
    echo ""
    print_error "Some tests failed!"
    exit 1
fi
