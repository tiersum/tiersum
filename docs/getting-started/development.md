# Development Guide

## Prerequisites

- Go 1.23+ (with CGO enabled for SQLite)
- `air` (optional, for hot reload)
- `golangci-lint` (optional, for linting)

## Quick Commands

```bash
# Run tests with race detection + coverage
make test

# Run linter
make lint

# Format code
make fmt

# Run with hot reload (requires air)
make dev

# Build for current platform
make build

# Build for multiple platforms
make build-all

# Docker build
make docker-build
```

## Testing

- Test files: `*_test.go` alongside source files
- Uses `testify/assert` and `testify/require`
- Run with race detection: `go test -race ./...`

## Code Style

- Standard Go formatting (`go fmt`)
- Imports grouped: standard lib, third-party, internal
- Comments: English only, complete sentences
- Error handling: explicit error returns, wrapped with context

## Adding New Features

1. Define interface in layer's `interface.go`
2. Implement in a service implementation package
3. Wire in `internal/di/container.go`
4. Add tests in `*_test.go`
5. Update docs if affecting non-trivial API flows (see `../algorithms/core-api-flows.md`)

## Project Layout

See [project-structure.md](project-structure.md) for the full directory tree.
