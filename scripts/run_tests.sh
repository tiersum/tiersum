#!/bin/bash
# Run all unit tests for the service layer

set -e

echo "Running service layer unit tests..."

# Run all tests in svcimpl package
go test -v ./internal/service/svcimpl/...

# Run with coverage
echo ""
echo "Running tests with coverage..."
go test -cover ./internal/service/svcimpl/...

# Generate coverage report
echo ""
echo "Generating coverage report..."
go test -coverprofile=coverage.out ./internal/service/svcimpl/...
go tool cover -html=coverage.out -o coverage.html

echo ""
echo "Tests completed! Coverage report: coverage.html"
