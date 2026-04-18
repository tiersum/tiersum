# Roadmap

## Implemented

- [x] Hot/Cold document tiering with auto-promotion
- [x] BM25 + Vector hybrid search over cold chapters (full chapter text)
- [x] Three-level summarization (document summary + chapter summaries + source text)
- [x] Topics + catalog tags (deterministic regroup; richer LLM-driven regroup optional future work)
- [x] Progressive query with LLM filtering at each step
- [x] LLM auto-tagging for documents
- [x] REST API + MCP Server
- [x] SQLite/PostgreSQL + in-memory cache storage
- [x] Vue 3 CDN frontend with Tailwind + DaisyUI

## Planned

- [ ] OpenClaw skill pack (convert + update)
- [ ] Real-time collaborative editing
- [ ] Multi-modal support (images, diagrams)
- [ ] Enterprise SSO + audit logs
- [ ] Additional document format parsers (LaTeX, AsciiDoc, PDF/HTML/Docs)
- [ ] Enhanced local LLM adapters (vLLM, etc.)

## Contributing

We welcome contributions! See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

**Good first issues**:

- Additional document format parsers
- Local LLM adapter improvements
- Enhanced Web UI features
