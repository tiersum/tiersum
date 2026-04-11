package service

import "errors"

// ErrColdIndexUnavailable is returned when cold hybrid search is requested but no cold index is configured.
var ErrColdIndexUnavailable = errors.New("cold document index not available")
