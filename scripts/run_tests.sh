#!/bin/bash
# Run unit tests for the service package (interfaces only until implementations return).

set -e

echo "Running internal/service tests..."

go test -v ./internal/service/...

echo ""
echo "Running tests with coverage..."
go test -cover ./internal/service/...

echo ""
echo "Generating coverage report..."
go test -coverprofile=coverage.out ./internal/service/...
go tool cover -html=coverage.out -o coverage.html

echo ""
echo "Tests completed! Coverage report: coverage.html"
