# Cold document index: algorithms and design

This document describes the **core algorithms** for cold-document **chapter extraction**, **dual indexing** (BM25 + dense vectors), and **hybrid retrieval**. It complements the endpoint-oriented notes in [CORE_API_FLOWS.md](CORE_API_FLOWS.md). When in doubt, **source code is authoritative** (`internal/storage/coldindex/`).

**中文版：** [COLD_INDEX_zh.md](COLD_INDEX_zh.md)

---

## 1. Role and contracts

- **Cold documents** are stored in the DB with `status = cold` and ingested into an in-memory **`storage.IColdIndex`** implementation (`coldindex.Index`).
- The index exposes **documents + plain-text search** only; ranking details are internal. See `internal/storage/interface.go` (`IColdIndex`, `ColdIndexHit`).
- **No full LLM analysis** on the cold ingest path; optional **text embeddings** (MiniLM or simple fallback) feed the vector side of search.

---

## 2. Chapter extraction (markdown → cold chapters)

**Primary code:** `internal/storage/coldindex/markdown_chapter_splitter_impl.go`, `chapter_split_stride.go`, splitter tests under `internal/storage/coldindex/*_test.go`.

### 2.1 Parse tree

- Input is Markdown body lines. **Fenced code blocks** (`` ``` ``) toggle a flag so `#` inside fences is **not** treated as a heading.
- Non-fence lines matching `^(#{1,6})\s+(.+)$` start a **heading node** with level, title, and `pathTitles` = parent path + this title.
- Inter-heading and pre-heading text accumulate into each node’s **`localBody`**.

### 2.2 Bottom-up merge (`postOrderMergeSplit`)

- Children are merged **post-order**. For each node, children’s chapter texts can be concatenated (with local prefix) if the combined **`EstimateTokens`** is ≤ **`maxTokens`** (from `Index.coldChapterMaxTokens`, default `types.DefaultColdChapterMaxTokens`).
- If merged text still exceeds the budget (or the node is a **leaf** with oversized body), the text is passed to **`splitOversizedRaw`**.

### 2.3 Token estimate

- **`EstimateTokens(s)`** ≈ `(utf8.RuneCountInString(s) + 3) / 4` — a cheap heuristic aligned with **4 runes per token** used for rune-window sizing below.

### 2.4 Sliding windows (`splitOversizedRaw`)

Used when a single logical body still exceeds **`maxTokens`** after tree logic.

- **Window width (runes):** `maxTokens * 4`.
- **Stride (runes):** `strideTokens * 4`, where **`strideTokens`** comes from `SetColdMarkdownSlidingStrideTokens` / `cold_index.markdown.sliding_stride_tokens` (default **100** when unset), clamped to `[1, maxTokens]`.
- **Consecutive windows** start `strideTokens` (estimated tokens) apart; **overlap** ≈ `maxTokens - strideTokens` in token space.
- **Paths:**
  - With parent headings: append **`"1"`, `"2"`, …** as the last path segment (e.g. `docId/Section/1`).
  - With **no** heading path: use synthetic **`__root__`** + index (e.g. `docId/__root__/1`).
- Output rows are **`rawSplitChapter`** → **`ColdChapter`** with `Path = docID + "/" + join(pathTitles, "/")` and trimmed `Text`.

### 2.5 Splitter interface

- **`IColdChapterSplitter`**: `Split(docID, docTitle, markdown, maxTokens) []ColdChapter`.
- Default: **`MarkdownSplitter`** (may set **`SlidingStrideTokens`** for tests; otherwise global stride applies).
- **`SplitMarkdown`** is the package-level entry used when no per-splitter override is needed; it uses **`effectiveSlidingStrideTokens`**.

---

## 3. Dual indexing (per chapter row)

**Primary code:** `internal/storage/coldindex/cold_index_impl.go`, `cold_inverted_index_bleve_impl.go`, `cold_vector_index_hnsw_impl.go`.

### 3.1 Row model

Each indexed unit is one **cold chapter** (not whole document):

- **`DocumentIndex`**: `id` / `path` (same as chapter key), `document_id`, `title` (document title), `content` (full chapter text), `embedding`.

### 3.2 Bleve (BM25, lexical)

- In-memory Bleve index with **Chinese jieba** tokenizer / analyzer on `title` + `content`.
- **External document ID** for each chapter row = **`ch.Path`** (globally unique across documents).
- Stored fields include `id`, `path`, `document_id`, `title`, `content` for search hit reconstruction.

### 3.3 HNSW (vector)

- **Graph key** = same chapter path string; **vector** = embedding of chapter body (same dimension as `types.ColdEmbeddingVectorDimension`).
- **Cosine** distance; **`M`** / **`EfSearch`** from defaults in `NewIndex` (not yet viper-driven).

### 3.4 `AddDocument` pipeline

1. Lock; **remove** any existing chapters for `doc.ID` (Bleve delete + HNSW delete + map cleanup).
2. **`splitter.Split(..., maxTok)`** → list of **`ColdChapter`**.
3. For each non-empty chapter text:
   - **Embedding:** `IColdTextEmbedder.Embed` if set, else **`GenerateSimpleEmbedding`** (deterministic fallback).
   - **Bleve:** `indexChapter(path, docIdx)` (embedding stripped from stored payload for Bleve).
   - **HNSW:** `vector.add(path, vec)`.
   - **`documents[path]`** and per-doc **`docChapterPaths[docID]`** updated.

### 3.5 Rebuild and lifecycle

- **`RebuildFromDocuments`**: closes old indexes, constructs a new `Index`, then restores **embedder, chapter token budget, branch-recall settings, splitter** before re-adding all cold docs.

---

## 4. Query embeddings (optional)

- If **`SetTextEmbedder`** was called at startup (`cmd/main.go` → `NewTextEmbedderFromViper`), **`Search`** embeds the **query string** with the same fallback rules as ingest (`FallbackColdTextEmbedding`).
- If embedding length ≠ `VectorDimension`, the **vector branch is skipped** (BM25-only hybrid degenerates to text side + empty vector list).

---

## 5. Hybrid search and merge

**Primary code:** `cold_index_impl.go` — `Search`, `hybridSearch`, `searchWithBleve`, `searchWithVector`, `mergeHybridResults`, `branchRecallSize`.

### 5.1 Branch recall (why > topK)

- For final **`topK`**, each branch requests **`branchRecallSize(topK)`** hits, where:
  - `recall = clamp(topK * multiplier, floor, ceiling)` from **`cold_index.search`** (`SetColdSearchRecall` / defaults in `Index`).
- This lets **BM25** and **vector** each propose a larger pool so **`mergeHybridResults`** can fuse overlapping chapters before cutting to **`topK`**.

### 5.2 Branch scores

- **BM25:** raw Bleve score; normalized by **max BM25 score** in the batch, then × **0.5**.
- **Vector:** cosine similarity in **[0, 1]** (clamped), × **0.5**.
- Same **`path`**: BM25 row and vector row **merge**; scores add; `source` becomes **`hybrid`**.

### 5.3 Dedupe key

- **`mergeResultKeyChapter`**: **`path`** if non-empty, else **`document_id`** (legacy safety).

### 5.4 Output

- Sorted by **descending fused score**; truncated to **`topK`**.
- Mapped to **`[]storage.ColdIndexHit`** (`document_id`, `path`, `title`, `content`, `score`, optional `source`).

---

## 6. Configuration (reference)

| Key | Purpose |
|-----|---------|
| `cold_index.markdown.chapter_max_tokens` | Per-chapter token budget for tree merge + window width for oversized splits |
| `cold_index.markdown.sliding_stride_tokens` | Step between sliding-window starts (tokens); unset / `<1` → default **100** |
| `cold_index.search.branch_recall_multiplier` | `recall = topK * multiplier` before floor/ceiling |
| `cold_index.search.branch_recall_floor` | Minimum branch recall |
| `cold_index.search.branch_recall_ceiling` | Maximum branch recall |
| `cold_index.embedding.*` | Cold text embedder (MiniLM / simple / auto) and ONNX paths |
| *(no viper keys yet)* | HNSW `M` / `EfSearch` use defaults in `internal/storage/coldindex/cold_index_impl.go` |

See `configs/config.example.yaml`.

---

## 7. Tests and fixtures

| Asset | Purpose |
|-------|---------|
| `internal/storage/coldindex/testdata/chapter_split_boundaries/*.input.md` + `*.golden.json` | Golden **boundary** cases (no parent / parent / nested + sliding); run `TestChapterSplitBoundaryGolden`; regenerate with `UPDATE_CHAPTER_SPLIT_GOLDEN=1` |
| `internal/storage/coldindex/chapter_split_sliding_test.go` | Sliding overlap and stride override |
| `internal/storage/coldindex/index_chapter_test.go` | Index add / merge key / stub splitter |
| `TestSplitMarkdown_KafkaZkEtcdFixtures_IO` | Optional bulk IO under `testdata/split_io_out/` (skipped if <10 `.md` in `testdata/`) |

---

## 8. REST surface

- **`GET /api/v1/cold/chapter_hits`**: comma-separated **`q`** terms → **`IColdIndex.Search`** → JSON hits with **full chapter** `context`. See [CORE_API_FLOWS.md §5](CORE_API_FLOWS.md).

---

## 9. File map

| File | Responsibility |
|------|------------------|
| `markdown_chapter_splitter_impl.go` | Tree parse, merge, `splitOversizedRaw`, `MarkdownSplitter` |
| `chapter_split_stride.go` | Global sliding stride config |
| `cold_index_impl.go` | `Index`, `AddDocument`, `Search`, hybrid merge, branch recall |
| `cold_inverted_index_bleve_impl.go` | Bleve index/delete/search |
| `cold_vector_index_hnsw_impl.go` | HNSW add/delete/search |
| `cold_text_embedder_*_impl.go`, `cold_text_embedder_factory.go`, `cold_text_embedding_fallback.go` | Cold embedder implementations, factory, and embedding fallback |
| `internal/storage/interface.go` | `IColdIndex`, `ColdIndexHit` |
| `cmd/main.go` | `SetColdChapterMaxTokens`, `SetColdMarkdownSlidingStrideTokens`, `SetColdSearchRecall`, `SetTextEmbedder`, load cold docs |

---

*Last updated to match cold-index behavior in `internal/storage/coldindex`; treat code as the source of truth if this document drifts.*
