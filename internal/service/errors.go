package service

import "errors"

// ErrColdIndexUnavailable is returned when cold hybrid search is requested but no cold index is configured.
var ErrColdIndexUnavailable = errors.New("cold document index not available")

// ErrIngestValidation is returned when document ingest fails configurable policy checks (format, size, chunking).
var ErrIngestValidation = errors.New("ingest validation failed")
