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

**主要代码：** `internal/storage/coldindex/markdown_chapter_splitter_impl.go`、`chapter_split_stride.go`，以及 `internal/storage/coldindex/*_test.go` 中的切分相关测试。

### 2.1 解析树（goldmark AST + 中文编号补充）

解析器采用**两阶段标题提取**策略，在最大化召回的同时最小化误判。

#### 第一阶段：goldmark CommonMark AST（主）

- **解析器：** `github.com/yuin/goldmark`（`coldChapterParser`）。文档一次性解析；章节边界仅来自被 `goldmarkHeadingAdopt` 接受的 **`ast.Heading`** 节点。
- **采纳规则（"宁可漏提取，不要误提取"）：** 只有 **直接属于文档** 的标题才被使用（`Heading.Parent().Kind() == Document`）。**引用块、列表、表格** 等内部的标题在树构建时被 **忽略**（其文本保留在父级正文区）。
- **代码块和表格被预先排除：** AST 遍历时，`collectGoldmarkSpans` 会为每个 `CodeBlock`、`FencedCodeBlock` 及表格类区域建立 **禁用的字节范围**。任何 `ast.Heading` 的字节 span 若与禁用范围重叠，则在树构建前 **丢弃**，确保代码块或表格内偶然出现的标题绝不可能成为章节边界。
- **ATX 与 Setext：** 两者均表现为 `ast.Heading`，`Level` 为 1–6；正文切片使用 `Heading.Lines()` 的字节范围，因此分隔符/下划线行不会重复进入 `localBody`。
- **数字编号大纲行**（如 `1. Introduction`、`2.1 Methods`）**不是** CommonMark 标题，保留为 **正文**（通常落在某个 `#` 章节下）。`parseNumberedOutlineHeading` 仅用于 **单元测试**，不参与冷索引树构建。

#### 第二阶段：中文编号标题补充

goldmark/CommonMark 不识别中文编号行（`一、系统架构`、`（一）模块设计`）为标题。第二轮扫描将这些行作为 **补充 span** 收集：

- **模式一：** `^([一二三四五六七八九十百]+)、(.+)$` → **层级 2**（例：`一、系统架构`）
- **模式二：** `^（([一二三四五六七八九十百]+)）(.+)$` → **层级 3**（例：`（一）模块设计`）
- 与 goldmark 标题 span **重叠** 的行被 **跳过**，避免重复。
- 合并后的 span（goldmark + 中文）按 **字节偏移量排序** 后进入树构建。

#### 算法流程

```
输入：markdown 字符串
  │
  ├─ normalizeEOL（\r\n → \n）
  ├─ stripYAMLFrontmatter（移除 --- 元数据块）
  │
  ├─ 第一阶段：goldmark.Parse → 收集 ast.Heading（仅 Document 子节点）
  │     └─ 丢弃与禁用 span（代码块、围栏代码块）重叠的标题
  │
  ├─ 第二阶段：扫描文本 → 收集中文编号行
  │     └─ 若与 goldmark heading span 或禁用 span 重叠则跳过
  │     └─ 若行以 "|" 开头则跳过（表格行防护）
  │
  ├─ mergeAndSortSpans(goldmarkSpans, chineseSpans)
  │
  └─ 构建树：
        按顺序遍历每个 span：
            flushBody(prevEnd, span.start) → 当前节点 localBody
            栈顶层级 ≥ span.level 时出栈
            压入新节点
        flushBody(prevEnd, EOF)
```

#### 示例：混合标题

```markdown
# 产品指南

一、系统架构

架构概述……

（一）总体设计

设计说明……

## API 参考

API 详情……
```

**生成的树：**

```
root (level 0)
├── "产品指南" (level 1)
│   └── "一、系统架构" (level 2)
│       └── "（一）总体设计" (level 3)
└── "API 参考" (level 2)
```

**合并后的路径（预算充足时）：**
- `docId/产品指南`
- `docId/产品指南/一、系统架构`
- `docId/产品指南/一、系统架构/（一）总体设计`
- `docId/产品指南/API 参考`

标题之间及首个标题前的正文累积到对应节点的 **`localBody`**。

### 2.2 自下而上合并（`postOrderMergeSplit`）

树按 **后序** 遍历（先子后父）。对每个节点：

1. 递归合并子节点；每个子节点可能返回一个或多个 `rawSplitChapter` 切片。
2. 将本节点的 `localBody`（标题与第一个子标题之间的正文）与所有子章节合并。
3. 若 `EstimateTokens(合并后)` ≤ `maxTokens` 且该节点为真实标题（`level > 0`），**输出一章** 包含整棵子树的正文。合并后的文本会重新插入标题行（`# 标题`），避免丢失。
4. 若合并后正文超出预算：
   - 若本节点有 `localBody` 正文，将其作为独立章节输出（可能超大 → `splitOversizedRaw`）。
   - 将每个子节点的章节分别输出。
5. 对于 **根节点**（level 0，无标题），若子节点未产生章节但 `localBody` 非空，将整个文档视为一个超大叶子处理。

**示例：**

```markdown
# 第一章

引言……

## 第一节

第一节内容……

## 第二节

第二节内容……
```

预算充足时，整棵树合并为：
- **路径：** `docId/第一章`
- **正文：** `# 第一章\n\n引言……\n\n## 第一节\n\n第一节内容……\n\n## 第二节\n\n第二节内容……`

预算较小时，拆分为：
- `docId/第一章`（仅引言）
- `docId/第一章/第一节`（第一节内容）
- `docId/第一章/第二节`（第二节内容）

若任一片段仍超出 `maxTokens`，则交由 **`splitOversizedRaw`** 处理。

### 2.3 Token 估算

- **用途**：为 **稠密向量索引**（嵌入模型的 **序列长度** 上限，如 MiniLM 约 **512 子词**）控制冷章节体量，**不是**为对话/推理 LLM 的 prompt 预算。目标是：在不超过嵌入序列上限的前提下 **尽量占满预算**，并在 Markdown 切分能力范围内 **保持章节结构尽量完整**。
- **`EstimateTokens(s)`** 采用**混合单位**，使冷章节体量更接近上述子词上限：**汉字、平假名、片假名、韩文音节、常见 CJK 标点与全角区** 按约 **1 单位 / rune**；其余字符仍用 **`(runeCount+3)/4`** 的 **约 4 rune / 单位** 启发式。同一单位用于 **`postOrderMergeSplit`** 的合并判断，并与 **`splitOversizedRaw`** 的窗口宽度 `maxTokens * 4`（rune）换算一致。

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
- **`MarkdownChaptersFromSplit`**（`markdown_chapters.go`）在与入库相同的 splitter、token 预算下生成 **`types.Chapter`** 供 REST/详情使用；人读标题由 **`pkg/markdown.ChapterDisplayTitle`** 生成。**`IColdIndex.MarkdownChapters`** 委托给该函数。

---

## 3. 双路索引（按「章」建库）

**主要代码：** `internal/storage/coldindex/cold_index_impl.go`、`cold_inverted_index_bleve_impl.go`、`cold_vector_index_hnsw_impl.go`。

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

**主要代码：** `cold_index_impl.go` 中的 `Search`、`hybridSearch`、`searchWithBleve`、`searchWithVector`、`mergeHybridResults`、`branchRecallSize`。

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
| *（尚无 viper 项）* | HNSW 的 `M` / `EfSearch` 等目前为 `internal/storage/coldindex/cold_index_impl.go` 中的默认值 |

完整示例见 `configs/config.example.yaml`。

---

## 7. 测试与夹具

| 资产 | 用途 |
|------|------|
| `internal/storage/coldindex/testdata/chapters/*.md`（22 个文件） | Markdown 夹具，覆盖：API 文档、技术指南、教程、Setext/ATX 混合、中文内容、中文编号标题、编号大纲、代码块（围栏/缩进）、引用块、YAML frontmatter、emoji、HTML 标签、链接/图片、长标题、注释、空行 |
| `internal/storage/coldindex/testdata/goldens/*.golden.json` | **边界** golden：无父标题 / 单级标题 / 嵌套标题 + 滑动切分；运行 `TestChapterSplitBoundaryGolden`；更新 golden：`UPDATE_CHAPTER_SPLIT_GOLDEN=1` |
| `chapter_split_integration_test.go` | 全部集成测试：夹具加载、边界 golden、滑动窗口、中文标题 |
| `chapter_split_markdown_test.go` | Markdown 专项单元测试（Setext、代码围栏、列表、引用块） |
| `chapter_split_unit_test.go` | 辅助函数测试（token 估算、路径清理、Setext 下划线等） |
| `index_chapter_test.go` | 索引写入、合并键、桩切分器 |

---

## 8. REST 接口

- **`GET /api/v1/cold/chapter_hits`**：逗号分隔的 **`q`** → **`IColdIndex.Search`** → JSON 命中项，`context` 为 **整章正文**。流程见 [CORE_API_FLOWS.md §5](CORE_API_FLOWS.md)。

---

## 9. 文件职责一览

| 文件 | 职责 |
|------|------|
| `markdown_chapter_splitter_impl.go` | 解析树、合并、`splitOversizedRaw`、`MarkdownSplitter` |
| `chapter_split_stride.go` | 包级滑动步长配置 |
| `cold_index_impl.go` | `Index`、`AddDocument`、`Search`、混合合并、分支召回 |
| `cold_inverted_index_bleve_impl.go` | Bleve 增删查 |
| `cold_vector_index_hnsw_impl.go` | HNSW 增删查 |
| `cold_text_embedder_*_impl.go`、`cold_text_embedder_factory.go`、`cold_text_embedding_fallback.go` | 冷 embedder 实现、工厂与回退嵌入 |
| `internal/storage/interface.go` | `IColdIndex`、`ColdIndexHit` |
| `cmd/main.go` | `SetColdChapterMaxTokens`、`SetColdMarkdownSlidingStrideTokens`、`SetColdSearchRecall`、`SetTextEmbedder`、加载冷文档 |

---

*本文与 `internal/storage/coldindex` 中冷索引行为对齐；若文档滞后，请以代码为准。*
