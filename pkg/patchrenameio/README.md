# renameio (TierSum fork)

This tree under `pkg/patchrenameio` replaces `github.com/google/renameio@v1.0.1` via `go.mod` `replace`.

Upstream intentionally omits `TempFile` / `WriteFile` on Windows (`//go:build !windows`), but
`github.com/coder/hnsw` calls `renameio.TempFile` unconditionally when persisting vector graphs,
which breaks `GOOS=windows` builds. This fork keeps the same API and behavior as the Unix
implementation, including `CloseAtomicallyReplace` with `Sync` before `Rename` (see upstream
comments in `tempfile.go`). Windows atomic-rename caveats from upstream issue #1 still apply at
the OS level; this is sufficient for TierSum’s cold-index HNSW persistence.

Source is derived from Google’s renameio v1.0.1 (Apache-2.0). See `LICENSE`.
