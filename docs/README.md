# Documentation

This directory contains TierSum's technical documentation, organized by topic.

## Structure

| Directory | Content |
|-----------|---------|
| **[getting-started/](getting-started/)** | Project structure, build commands, development guide |
| **[design/](design/)** | Architecture, authentication, and permissions design |
| **[algorithms/](algorithms/)** | Core API flows, cold document indexing algorithms |
| **[ui/](ui/)** | Web UI guide |
| **[planning/](planning/)** | Roadmap and future plans |

## Key Documents

### Getting Started
- [Project Structure](getting-started/project-structure.md) — Directory layout and conventions
- [Development Guide](getting-started/development.md) — Build, test, lint, and code style

### Design
- [Architecture](design/architecture.md) — 5-layer architecture with Interface+Impl pattern
- [Auth and Permissions](design/auth-and-permissions.md) — Dual-track auth design (human vs program)

### Algorithms
- [Core API Flows](algorithms/core-api-flows.md) — Ingest tiering, progressive query, topic regroup, cold search
- [Cold Document Index](algorithms/cold-index/) — Chapter extraction, BM25 + HNSW hybrid search, embeddings

### UI
- [Web UI](ui/web-ui.md) — Vue 3 CDN frontend pages and tech stack

### Planning
- [Roadmap](planning/roadmap.md) — Implemented features and planned work

## Language

Documents are in English by default. Chinese versions are marked with `.zh.md` suffix where available.

---

*For product overview and quick start, see [README.md](../README.md).*
