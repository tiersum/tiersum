# Cold Document Index

This directory documents the cold document indexing algorithms.

## Documents

| Document | Language | Content |
|----------|----------|---------|
| [cold-index.md](cold-index.md) | English | Chapter extraction, BM25 + HNSW hybrid search, embeddings |
| [cold-index.zh.md](cold-index.zh.md) | 中文 | 同上，中文版本 |
| [chapter-tree-quality.md](chapter-tree-quality.md) | Mixed | Chapter tree parsing quality improvement plan |

## Overview

Cold documents are stored with `status = cold` and indexed in-memory with:

- **Bleve (BM25)** — lexical search with Chinese jieba tokenization
- **HNSW** — approximate nearest neighbor vector search
- **Hybrid merge** — deduplicated by chapter path, normalized score blend

See [Core API Flows](../core-api-flows.md) for endpoint call chains.
