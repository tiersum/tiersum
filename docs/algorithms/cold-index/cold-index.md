# Cold document index: algorithms and design

This document describes the **core algorithms** for cold-document **chapter extraction**, **dual indexing** (BM25 + dense vectors), and **hybrid retrieval**. It complements the endpoint-oriented notes in [../core-api-flows.md](../core-api-flows.md). When in doubt, **source code is authoritative** (`internal/storage/coldindex/`).

**中文版：** [cold-index.zh.md](cold-index.zh.md)

---

## 1. Role and contracts

- **Cold documents** are stored in the DB with `status = cold` and ingested into an in-memory **`storage.IColdIndex`** implementation (`coldindex.Index`).
- The index exposes **documents + plain-text search** only; ranking details are internal. See `internal/storage/interface.go` (`IColdIndex`, `ColdIndexHit`).
- **No full LLM analysis** on the cold ingest path; optional **text embeddings** (MiniLM or simple fallback) feed the vector side of search.

---

## 2. Chapter extraction (markdown → cold chapters)

**Primary code:** `internal/storage/coldindex/markdown_chapter_splitter_impl.go`, `chapter_split_stride.go`, splitter tests under `internal/storage/coldindex/*_test.go`.

### 2.1 Parse tree (goldmark AST + Chinese supplement)

The parser uses a **two-phase heading extraction** strategy to maximize recall while minimizing false positives.

#### Phase 1: Goldmark CommonMark AST (primary)

- **Parser:** `github.com/yuin/goldmark` (`coldChapterParser`). The document is parsed once; chapter boundaries come only from **`ast.Heading`** nodes that **`goldmarkHeadingAdopt`** accepts.
- **Adoption rule ("宁可漏提取，不要误提取"):** a heading is used **only** if it is a **direct child of the document** (`Heading.Parent().Kind() == Document`). Headings inside **blockquotes, lists, tables**, etc. are **ignored** for the tree (their text stays in the parent body region).
- **Code blocks and tables are pre-excluded:** during AST walk, `collectGoldmarkSpans` builds a set of **forbidden byte ranges** for every `CodeBlock`, `FencedCodeBlock`, and table-like region. Any `ast.Heading` whose byte span overlaps a forbidden range is **dropped** before tree construction, so headings that accidentally appear inside code or tables can never become chapter boundaries.
- **ATX and Setext:** both appear as `ast.Heading` with `Level` 1–6; body slices use byte ranges from `Heading.Lines()` so delimiter / underline lines are not duplicated into `localBody`.
- **Numbered “outline” lines** such as `1. Introduction` or `2.1 Methods` are **not** CommonMark headings; they remain **body text** (often under a single `#` chapter). The helper `parseNumberedOutlineHeading` in `markdown_chapter_splitter_impl.go` is retained for **unit tests** only, not for the cold tree.

#### Phase 2: Chinese-numbered heading supplement

Goldmark/CommonMark does not recognize Chinese-numbered lines (`一、系统架构`, `（一）模块设计`) as headings. A second scan collects these as **supplement spans**:

- **Pattern 1:** `^([一二三四五六七八九十百]+)、(.+)$` → **level 2** (e.g. `一、系统架构`)
- **Pattern 2:** `^（([一二三四五六七八九十百]+)）(.+)$` → **level 3** (e.g. `（一）模块设计`)
- Spans that overlap any goldmark heading span are **skipped** to avoid duplication.
- The combined spans (goldmark + Chinese) are **sorted by byte offset** before tree construction.

#### Algorithm flow

```
Input: markdown string
  │
  ├─ normalizeEOL (\r\n → \n)
  ├─ stripYAMLFrontmatter (remove --- metadata blocks)
  │
  ├─ Phase 1: goldmark.Parse → collect ast.Heading (Document children only)
  │     └─ drop any heading overlapping a forbidden span (code block, fenced code block)
  │
  ├─ Phase 2: scan text → collect Chinese-numbered lines
  │     └─ skip if overlapping goldmark heading span OR forbidden span
  │     └─ skip if line starts with "|" (table row guard)
  │
  ├─ mergeAndSortSpans(goldmarkSpans, chineseSpans)
  │
  └─ Build tree:
        for each span in order:
            flushBody(prevEnd, span.start) → current node.localBody
            pop stack while top.level >= span.level
            push new node
        flushBody(prevEnd, EOF)
```

#### Example: Mixed headings

```markdown
# Product Guide

一、系统架构

架构概述……

（一）总体设计

设计说明……

## API Reference

API details……
```

**Tree produced:**

```
root (level 0)
├── "Product Guide" (level 1)
│   └── "一、系统架构" (level 2)
│       └── "（一）总体设计" (level 3)
└── "API Reference" (level 2)
```

**Paths (after merge, if budget allows):**
- `docId/Product Guide`
- `docId/Product Guide/一、系统架构`
- `docId/Product Guide/一、系统架构/（一）总体设计`
- `docId/Product Guide/API Reference`

Inter-heading and pre-heading text accumulate into each node's **`localBody`** from the source between adopted heading spans.

### 2.2 Bottom-up merge (`postOrderMergeSplit`)

The tree is traversed **post-order** (children first, then parent). For each node:

1. Recursively merge children; each child may return one or more `rawSplitChapter` slices.
2. Combine the node's `localBody` (text between its heading and the first child heading) with all child chapters.
3. If `EstimateTokens(combined)` ≤ `maxTokens` and the node is a real heading (`level > 0`), **emit one merged chapter** containing the full subtree text. The merged text re-inserts heading lines (`# Title`) so they are not lost.
4. If the combined text exceeds the budget:
   - If the node has `localBody` text, emit it as separate chapter(s) (may be oversized → `splitOversizedRaw`).
   - Emit each child's chapters separately.
5. For the **root node** (level 0, no title), if no children produced chapters but `localBody` is non-empty, treat the entire document as one oversized leaf.

**Example:**

```markdown
# Chapter A

Intro text……

## Section B

B content……

## Section C

C content……
```

With `maxTokens` large enough, the entire tree merges into:
- **Path:** `docId/Chapter A`
- **Text:** `# Chapter A\n\nIntro text……\n\n## Section B\n\nB content……\n\n## Section C\n\nC content……`

With a small budget, it splits into:
- `docId/Chapter A` (intro only)
- `docId/Chapter A/Section B` (B content)
- `docId/Chapter A/Section C` (C content)

If any single piece still exceeds `maxTokens`, it is passed to **`splitOversizedRaw`**.

### 2.3 Token estimate

- **Purpose:** size cold chapters for **dense vector indexing** (embedder max **sequence** length, e.g. ~512 subwords for MiniLM), **not** for LLM prompt budgeting. Goal is to **fill the embedder budget** while keeping chapters as **structurally whole** as the markdown splitter allows.
- **`EstimateTokens(s)`** uses **mixed units** so cold chapters stay closer to **~512 subword tokens** for MiniLM-sized embedders: **Han / Hiragana / Katakana / Hangul / common CJK punctuation and fullwidth forms** count **~1 unit per rune**; all other runes keep the legacy **`(runeCount + 3) / 4`** style **~4 runes per unit**. The same units gate **`postOrderMergeSplit`** merge decisions and align **`splitOversizedRaw`** window width (`maxTokens * 4` runes) with that heuristic.

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
- **`MarkdownChaptersFromSplit`** (`markdown_chapters.go`) builds **`types.Chapter`** for REST/detail flows using the same splitter + token budget as ingest; human-facing section titles use **`pkg/markdown.ChapterDisplayTitle`**. **`IColdIndex.MarkdownChapters`** delegates to it.

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
| `internal/storage/coldindex/testdata/chapters/*.md` (22 files) | Markdown fixtures covering: API docs, technical guides, tutorials, Setext/ATX mix, Chinese content, Chinese-numbered headings, numbered outlines, code blocks (fenced/indented), blockquotes, YAML frontmatter, emoji, HTML tags, links/images, long titles, comments, empty lines |
| `internal/storage/coldindex/testdata/goldens/*.golden.json` | Golden **boundary** cases (no parent / parent / nested + sliding); run `TestChapterSplitBoundaryGolden`; regenerate with `UPDATE_CHAPTER_SPLIT_GOLDEN=1` |
| `chapter_split_integration_test.go` | All integration tests: fixture loading, boundary golden, sliding window, Chinese headings |
| `chapter_split_markdown_test.go` | Markdown-specific unit tests (Setext, code fences, lists, blockquotes) |
| `chapter_split_unit_test.go` | Helper function tests (token estimate, path sanitize, setext underline, etc.) |
| `index_chapter_test.go` | Index add / merge key / stub splitter |

---

## 8. REST surface

- **`GET /api/v1/cold/chapter_hits`**: comma-separated **`q`** terms → **`IColdIndex.Search`** → JSON hits with **full chapter** `context`. See [../core-api-flows.md §5](../core-api-flows.md).

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
