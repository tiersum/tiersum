// Package svcimpl hosts concrete service implementations split by domain:
//
//   - common — cross-domain helpers (LLM summarization core, quota, config redaction, progressive-query OTel context, OTel-wrapped LLM)
//   - auth — program API keys and browser/admin auth
//   - document — ingest, hot async, chapter materialization, maintenance jobs
//   - query — progressive query, relevance filter, progressive answer prompt helpers
//   - topic — catalog topic regroup
//   - catalog — read facades for tags and chapters
//   - observability — cold-index / monitoring reads (IObservabilityService)
//   - admin — admin config snapshot
//   - stubs — shared test doubles for *_test.go in the subpackages above
//
// The composition root wires constructors from these subpackages in internal/di/container.go.
package svcimpl
