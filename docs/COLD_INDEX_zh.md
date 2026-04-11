# 冷文档索引：算法与设计

本文说明冷文档 **章节抽取**、**双路索引**（BM25 + 稠密向量）与 **混合检索** 的核心算法，与偏重接口调用链的 [CORE_API_FLOWS.md](CORE_API_FLOWS.md)（英文）互为补充。若有出入，以 **`internal/storage/coldindex/`** 中的 **源码为准**。

英文版：[COLD_INDEX.md](COLD_INDEX.md)

---

## 1. 职责与契约

- **冷文档**在数据库中 `status = cold`，并写入内存中的 **`storage.IColdIndex`** 实现（`coldindex.Index`）。
- 索引对外只暴露 **文档 + 纯文本检索**；具体排序策略为实现细节。契约见 `internal/storage/interface.go`（`IColdIndex`、`ColdIndexHit`）。
- 冷入库路径 **不做完整 LLM 分析**；可选 **文本向量**（MiniLM 或简单回退）仅服务于检索中的向量分支。

---

## 2. 章节抽取（Markdown → 冷章节）

**主要代码：** `internal/storage/coldindex/chapter_split.go`、`chapter_split_stride.go`，以及 `internal/storage/coldindex/*_test.go` 中的切分相关测试。

### 2.1 解析树

- 按行扫描 Markdown 正文。**围栏代码块**（`` ``` ``）切换状态，使围栏内的 **`#` 不作为标题** 解析。
- 非围栏行若匹配 `^(#{1,6})\s+(.+)$`，则创建 **标题节点**：记录层级、标题文本，`pathTitles` = 父级路径 + 当前标题。
- 标题之间及首个标题前的正文累积到对应节点的 **`localBody`**。

### 2.2 自下而上合并（`postOrderMergeSplit`）

- 对子树 **后序** 处理。若某节点下各子章节文本（可加本节点 `localBody` 前缀）拼接后的 **`EstimateTokens`** ≤ **`maxTokens`**（来自 `Index.coldChapterMaxTokens`，默认 `types.DefaultColdChapterMaxTokens`），则 **合并为一章**。
- 若合并后仍超预算，或叶子节点正文本身过大，则将该段正文交给 **`splitOversizedRaw`** 处理。

### 2.3 Token 估算

- **`EstimateTokens(s)`** ≈ `(utf8.RuneCountInString(s) + 3) / 4`，为低开销启发式，并与下文 **「每 token 约 4 个 rune」** 的窗口换算一致。

### 2.4 滑动窗口（`splitOversizedRaw`）

在树逻辑执行完毕后，**单段逻辑正文** 仍超过 **`maxTokens`** 时使用。

- **窗口宽度（rune 数）：** `maxTokens * 4`。
- **步长（rune 数）：** `strideTokens * 4`，其中 **`strideTokens`** 来自 `SetColdMarkdownSlidingStrideTokens` / 配置项 **`cold_index.markdown.sliding_stride_tokens`**（未配置或非法时默认 **100**），并限制在 **`[1, maxTokens]`**。
- **相邻窗口** 的起始位置在估算 token 空间上相距 **`strideTokens`**；**重叠量** 在 token 空间上约为 **`maxTokens - strideTokens`**。
- **路径规则：**
  - 有父级标题路径：在路径末尾追加 **`"1"`、`"2"`、…** 作为序号段（例：`docId/章节标题/1`）。
  - 无标题路径：使用合成根 **`__root__`** + 序号（例：`docId/__root__/1`）。
- 中间结构为 **`rawSplitChapter`**，再映射为 **`ColdChapter`**：`Path = docID + "/" + strings.Join(pathTitles, "/")`，`Text` 为去首尾空白后的正文。

### 2.5 切分器接口

- **`IColdChapterSplitter`**：`Split(docID, docTitle, markdown, maxTokens) []ColdChapter`。
- 默认实现 **`MarkdownSplitter`**（测试可设 **`SlidingStrideTokens`** 覆盖包级步长；否则走全局步长）。
- **`SplitMarkdown`** 为包级入口；步长经 **`effectiveSlidingStrideTokens`** 解析。

---

## 3. 双路索引（按「章」建库）

**主要代码：** `internal/storage/coldindex/index.go`、`inverted_bleve.go`、`vector_hnsw.go`。

### 3.1 行模型

每个索引单元是一条 **冷章节**（非整篇文档）：

- **`DocumentIndex`**：`id` / `path`（与章节键一致）、`document_id`、`title`（文档标题）、`content`（该章全文）、`embedding`。

### 3.2 Bleve（BM25，词法）

- 内存 Bleve 索引；`title` 与 `content` 使用 **中文 jieba** 分词与分析器。
- 每一章在 Bleve 中的 **外部文档 ID** = **`ch.Path`**（跨文档全局唯一）。
- 存储字段含 `id`、`path`、`document_id`、`title`、`content`，便于从命中结果还原正文。

### 3.3 HNSW（向量）

- **图节点键** = 与 Bleve 相同的章节路径字符串；**向量** = 该章正文的嵌入，维度为 **`types.ColdEmbeddingVectorDimension`**。
- **余弦**距离；**`M`**、**`EfSearch`** 等目前为 `NewIndex` 内默认值（尚未走 viper）。

### 3.4 `AddDocument` 流水线

1. 加锁；**删除**该 `doc.ID` 下已有章节（Bleve 删、HNSW 删、内存 map 清理）。
2. **`splitter.Split(..., maxTok)`** → **`[]ColdChapter`**。
3. 对每个非空章节正文：
   - **向量：**若已设置 `IColdTextEmbedder` 则 `Embed`，否则 **`GenerateSimpleEmbedding`**（确定性回退）。
   - **Bleve：**`indexChapter(path, docIdx)`（写入 Bleve 的负载去掉 embedding，减小存储）。
   - **HNSW：**`vector.add(path, vec)`。
   - 更新 **`documents[path]`** 与 **`docChapterPaths[docID]`**。

### 3.5 重建与生命周期

- **`RebuildFromDocuments`**：关闭旧索引、构造新 `Index`，再恢复 **embedder、章节 token 预算、分支召回参数、切分器**，然后重新加入全部冷文档。

---

## 4. 查询向量（可选）

- 若启动时在 `cmd/main.go` 中调用了 **`SetTextEmbedder`**（经 `NewTextEmbedderFromViper`），则 **`Search`** 会对 **查询字符串** 做嵌入，回退规则与入库一致（`FallbackColdTextEmbedding`）。
- 若向量长度 ≠ `VectorDimension`，则 **跳过向量分支**（混合检索退化为仅 BM25 一侧 + 空向量结果列表）。

---

## 5. 混合检索与合并

**主要代码：** `index.go` 中的 `Search`、`hybridSearch`、`searchWithBleve`、`searchWithVector`、`mergeHybridResults`、`branchRecallSize`。

### 5.1 分支召回（为何大于 topK）

- 对最终 **`topK`**，每个分支实际请求 **`branchRecallSize(topK)`** 条候选，其中：
  - `recall = clamp(topK * multiplier, floor, ceiling)`，参数来自 **`cold_index.search`**（`SetColdSearchRecall` / `Index` 内默认值）。
- 目的：让 **BM25** 与 **向量** 各自给出更大候选池，**`mergeHybridResults`** 在去重、融合后，再截断到 **`topK`**，避免过早丢失双通道重叠信息。

### 5.2 分支得分

- **BM25：**Bleve 原始分数；按本批 **最大 BM25 分数** 归一化后 × **0.5**。
- **向量：**余弦相似度，限制在 **[0, 1]**，再 × **0.5**。
- 同一 **`path`**：BM25 行与向量行 **合并**，分数相加，`source` 记为 **`hybrid`**。

### 5.3 去重键

- **`mergeResultKeyChapter`**：若 **`path`** 非空则以其为键，否则回退 **`document_id`**（兼容旧数据路径）。

### 5.4 输出

- 按 **融合分降序** 排序，截断至 **`topK`**。
- 映射为 **`[]storage.ColdIndexHit`**（`document_id`、`path`、`title`、`content`、`score`、可选 `source`）。

---

## 6. 配置项（速查）

| 配置键 | 作用 |
|--------|------|
| `cold_index.markdown.chapter_max_tokens` | 树合并的章节 token 预算；亦为超大叶滑动窗的窗宽依据 |
| `cold_index.markdown.sliding_stride_tokens` | 滑动窗相邻起点间距（估算 token）；未设或 `<1` 时默认 **100** |
| `cold_index.search.branch_recall_multiplier` | 分支召回：`recall = topK * multiplier`，再受 floor/ceiling 约束 |
| `cold_index.search.branch_recall_floor` | 分支召回下限 |
| `cold_index.search.branch_recall_ceiling` | 分支召回上限 |
| `cold_index.embedding.*` | 冷文本 embedder（MiniLM / simple / auto）及 ONNX 等路径 |
| *（尚无 viper 项）* | HNSW 的 `M` / `EfSearch` 等目前为 `internal/storage/coldindex/index.go` 中的默认值 |

完整示例见 `configs/config.example.yaml`。

---

## 7. 测试与夹具

| 资产 | 用途 |
|------|------|
| `internal/storage/coldindex/testdata/chapter_split_boundaries/*.input.md` 与 `*.golden.json` | **边界** golden：无父标题 / 单级标题 / 嵌套标题 + 滑动切分；运行 `TestChapterSplitBoundaryGolden`；更新 golden：`UPDATE_CHAPTER_SPLIT_GOLDEN=1` |
| `internal/storage/coldindex/chapter_split_sliding_test.go` | 滑动重叠与步长覆盖 |
| `internal/storage/coldindex/index_chapter_test.go` | 索引写入、合并键、桩切分器 |
| `TestSplitMarkdown_KafkaZkEtcdFixtures_IO` | 可选大批量 IO（`testdata/split_io_out/`）；若 `testdata/` 下不足 10 个 `.md` 则 **跳过** |

---

## 8. REST 接口

- **`GET /api/v1/cold/doc_source`**：逗号分隔的 **`q`** → **`IColdIndex.Search`** → JSON 命中项，`context` 为 **整章正文**。流程见 [CORE_API_FLOWS.md §5](CORE_API_FLOWS.md)。

---

## 9. 文件职责一览

| 文件 | 职责 |
|------|------|
| `chapter_split.go` | 解析树、合并、`splitOversizedRaw`、`MarkdownSplitter` |
| `chapter_split_stride.go` | 包级滑动步长配置 |
| `index.go` | `Index`、`AddDocument`、`Search`、混合合并、分支召回 |
| `inverted_bleve.go` | Bleve 增删查 |
| `vector_hnsw.go` | HNSW 增删查 |
| `embed_*.go`、`embed_factory.go` | 冷 embedder 实现与 `NewTextEmbedderFromViper` |
| `internal/storage/interface.go` | `IColdIndex`、`ColdIndexHit` |
| `cmd/main.go` | `SetColdChapterMaxTokens`、`SetColdMarkdownSlidingStrideTokens`、`SetColdSearchRecall`、`SetTextEmbedder`、加载冷文档 |

---

*本文与 `internal/storage/coldindex` 中冷索引行为对齐；若文档滞后，请以代码为准。*
