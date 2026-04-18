# MarkdownChapters 算法深度分析与优化报告

## 文档信息

- **分析对象**: `internal/storage/coldindex/markdown_chapter_splitter_impl.go`
- **分析日期**: 2026-04-18
- **分析范围**: `MarkdownChapters` 及相关函数
- **关键澄清（与 LLM 无关）**：冷路径抽取的章节首先是 **向量索引（HNSW）的编码单元**：要在 **嵌入模型序列长度上限**（如 MiniLM 常见 **512 子词**）内 **尽量占满预算**（单章信息量更大、向量更稳），同时 **尽量保持结构完整**（尤其避免在 fenced code 等块中间硬切）。**不**以对话/推理 LLM 的 context 为设计目标；REST 详情展示的章节列表与入库使用同一套切分，但 **优化动机仍面向向量与检索语义**，而非 LLM。

---

## 实施与复核状态（2026-04-18）

以下已在源码落地（以 `internal/storage/coldindex/markdown_chapter_splitter_impl.go` 为准）。

| 标记 | 含义 |
|------|------|
| **[已实施]** | 行为已变更；冷文档需 **重建冷索引** 后，向量/BM25 与旧切分结果才会一致。 |
| **[已部分实施]** | 仅覆盖部分场景；其余仍见下方「问题诊断」。 |
| **[仍开放]** | 未改代码，或仅文档澄清。 |
| **[文档勘误]** | 本报告旧版片段与仓库不一致，以源码与 `docs/COLD_INDEX*.md` 为准。 |

| 原报告 § | 状态 | 说明 |
|-----------|------|------|
| 2.3 Token 估算 | **[已实施]** | `EstimateTokens`：**CJK 相关脚本约 1 rune / 预算单位**，拉丁等沿用 **`(other+3)/4`**，使合并章节更接近 **嵌入模型序列长度**（如 MiniLM ~512 子词），**与 LLM 无关**；`pkg/types/cold_embedding.go` 注释已对齐。 |
| 2.5 标题识别 | **[已部分实施]** | **Setext**（`===` / `---`）已进解析树；**`N.` 单行大纲**在 remainder **以十进制数字开头**时不再判为标题（减轻 `1. 2. …` 类列表误判）。下文旧伪代码 `strings.Contains` 为 **[文档勘误]**。Setext 与「仅 `---`」水平线仍靠「上一行非空作标题」的实用规则区分。 |
| 2.2 滑窗与结构 | **[仍开放]** | 超大叶仍 **rune 滑窗**，未做 fence/段落感知切断。 |
| 2.1 / 2.4 部分合并 | **[仍开放]** | `postOrderMergeSplit` **全合并或拆开**策略未改。 |
| 2.6 stride | **[仍开放]** | 默认 100 token 步长与配置项未改。 |

### 本次修改汇总（功能）

1. **章节树识别**：在 ATX 之后、编号大纲之前增加 **Setext** 识别（下一行全 `=` 为一级、全 `-` 为二级）；**单行编号大纲**收紧。  
2. **向量预算（与 LLM 无关）**：`EstimateTokens` 混合单位，减轻 **中文等 CJK 文本** 在 `maxTokens=512` 下仍按 `runes/4` 导致 **单章真实子词远超嵌入序列上限**、**向量只编码截断前缀** 的问题。  
3. **文档**：`docs/COLD_INDEX.md` / `docs/COLD_INDEX_zh.md` §2.1、§2.3；`docs/CORE_API_FLOWS.md` §0；本文件增加本状态节与附录勘误。

### 新增 / 调整测试

- `internal/storage/coldindex/chapter_split_markdown_test.go`：`TestSplitMarkdown_setextHeadings`、`TestEstimateTokens`（CJK）、`TestParseNumberedOutlineHeading_rejectsRemainderStartingWithDigit`。

---

## 一、背景与目标

### 1.0 核心澄清：向量索引场景

**本章节的抽取目标**：
- ✅ **用于**：HNSW 向量索引（`internal/storage/coldindex/cold_vector_index_hnsw_impl.go`）；每条冷章节文本经 embedder 得到 **固定维度向量**（如 `types.ColdEmbeddingVectorDimension` = **384**，与 **512 子词上限** 是两件事）。
- ✅ **输入**：MiniLM / 其他 **文本嵌入** 模型（ONNX tokenizer 截断至其 **max sequence length**）。
- ✅ **约束**：`cold_index.markdown.chapter_max_tokens`（默认 **512**，与 `EstimateTokens` **预算单位**对齐）— 控制合并与滑窗，使 **送入 embedder 的文本长度**贴近模型上限又尽量少被静默截断。
- ✅ **要求**：**结构尽量完整**（少在 fence/语义块中间下刀）+ **在序列长度限制内尽量大段**（提高向量表征的信息量）。
- ❌ **非设计目标**：对话/推理 **LLM** 的 prompt 预算；冷入库路径本身也 **不做** 全量文档 LLM 分析。

**为什么「完整 + 贴近上限」对向量重要**：
- 嵌入模型把 **整段章节文本** 编成 **384-d**（示例）向量；若滑窗在代码块中间切断 → 向量只覆盖 **语法残片** → 检索命中语义差、命中文本不可用。
- 若合并后 **真实子词数仍远超** embedder 上限 → tokenizer **截断尾部** → 向量 **未编码** 被截掉的内容，检索侧信息丢失。
- 若每段过短 → 向量 **上下文不足** → 主题信号弱，混合检索中向量分支贡献下降。

---

## 二、算法流程概述

### 2.1 三层流水线架构

```
输入: Markdown 文档 (docID, title, markdown)
  │
  ▼
Phase 1: parseSplitTree (解析标题树)
  - 扫描行，识别 ATX 标题 (#) 和编号标题 (1.1, 2.3)
  - 代码围栏感知 (```)
  - 构建父子树结构 (splitNode)
  │
  ▼
Phase 2: postOrderMergeSplit (自底向上合并)
  - 后序遍历树
  - 叶子节点：检查 localBody 是否超过 maxTokens
  - 内部节点：尝试合并所有子节点 + 自身文本
    - 如果 ≤ maxTokens → 合并为一个章节
    - 如果 > maxTokens → "爆炸"：自身成为章节，子节点各自独立
  │
  ▼
Phase 3: splitOversizedRaw (滑动窗口兜底)
  - 对于仍然超大的叶子，按固定 rune 窗口切分
  - window = maxTokens × 4 runes
  - stride = 100 tokens × 4 runes
  │
  ▼
输出: []ColdChapter
```

### 1.2 核心数据结构

```go
// 与源码一致（见 markdown_chapter_splitter_impl.go）
type splitNode struct {
	level      int
	title      string
	pathTitles []string
	localBody  strings.Builder // pseudo-code; type in Go source
	children   []*splitNode
}

type ColdChapter struct {
    Path string  // 章节路径，如 "docID/第一章"
    Text string  // 章节完整文本（含标题）
}
```

---

## 二、问题诊断

### 2.1 问题 1：缺少部分合并策略

**严重等级**: 🔴 **高**

**场景示例**：技术文档，有 3 级标题

```markdown
# Kubernetes 架构
## 控制平面
控制平面是 Kubernetes 的大脑...（50 tokens）

### API Server
详细内容...（500 tokens）
### etcd
详细内容...（500 tokens）
### Scheduler
详细内容...（500 tokens）
```

**当前行为**（maxTokens=800）：
- 父节点 `localBody` = 50 tokens（只有简介）
- 3 个子节点各 500 tokens，合并后 = 1550 > 800
- **触发"爆炸"**：父节点 localBody 单独输出，3 个子节点各自独立
- **输出结果**：
  ```
  1. "Kubernetes 架构/控制平面" (50 tokens，只有简介)
  2. "Kubernetes 架构/控制平面/API Server" (500 tokens)
  3. "Kubernetes 架构/控制平面/etcd" (500 tokens)
  4. "Kubernetes 架构/控制平面/Scheduler" (500 tokens)
  ```

**问题分析**：
1. 父节点 `控制平面` 成了一个**只有简介的孤立章节（50 tokens）**
   - 向量嵌入质量差：50 token 的稀疏向量 vs 500 token 的稠密向量
   - 检索时可能因向量过于稀疏而匹配不到
2. 子节点各 500 tokens，本可以 "API Server + etcd" = 1000 > 800，但 "API Server 单独" 500 < 800，**没有尝试最优组合**
3. 缺少"贪心组合子节点直到接近 maxTokens"的策略

**向量索引场景下的正确行为**：
```
1. "Kubernetes 架构/控制平面" (50 + 500 = 550 tokens，合并简介 + API Server)
   → 向量嵌入编码了完整的"控制平面 + API Server"上下文
2. "Kubernetes 架构/控制平面/etcd" (500 tokens)
   → 独立向量，专门编码 etcd 语义
3. "Kubernetes 架构/控制平面/Scheduler" (500 tokens)
   → 独立向量，专门编码 Scheduler 语义
```

**根本原因**：
```go
// postOrderMergeSplit 内部节点逻辑（第 318-328 行）
if EstimateTokens(combined) <= maxTokens && n.level > 0 {
    return []rawSplitChapter{{pathTitles: n.pathTitles, text: combined}}
}
// ❌ 超过后直接爆炸：父节点 local + 所有子节点独立
// 没有尝试"选择部分子节点合并"
var out []rawSplitChapter
if strings.TrimSpace(local) != "" && n.level > 0 {
    out = append(out, splitOversizedRaw(n.pathTitles, local, maxTokens, strideTokens)...)
}
for _, chs := range nested {
    out = append(out, chs...)  // 所有子节点独立输出
}
```

---

### 2.2 问题 2：滑动窗口无视段落边界

**严重等级**: 🔴 **高**

**场景示例**：Markdown 中有代码块

````markdown
## 示例代码
```go
func main() {
    fmt.Println("Hello")
    // ... 500 行代码 ...
}
```
````

**当前行为**：
- 按 `maxTokens × 4` runes 切分
- **可能在代码块中间切断**
- **输出**：
  ```
  片段 1: "func main() {\n    fmt.Println("  (语法不完整)
  片段 2: "Hello")\n    // ... }"  (语法不完整)
  ```

**影响**：
1. **向量嵌入断裂**：代码块被切成两半 → HNSW 向量分别编码不完整代码 → 检索时匹配到的可能是半截代码
2. **语义不连贯**：向量模型无法理解半个代码块的语义 → 检索相关性降低
3. **检索质量下降**：用户查询 "kubernetes pod 创建代码" → 匹配到 "func main() {\n    fmt.Println(" → 无意义结果

**根本原因**：`splitOversizedRaw` 仍按 **rune 窗口**（`maxTokens*4`）切分，**不识别 fence/段落**；实现已改为 **UTF-8 前向滑窗**（无整篇 `[]rune`），语义风险不变。详见 **§8.2**。

---

### 2.3 问题 3：Token 估算过于粗糙 **[已实施]**

**严重等级**: 🟡 **中**（行为已按 CJK 混合单位调整；本节保留为「历史问题陈述 + 残余风险」）

**历史实现（已替换）**：曾为 `(utf8.RuneCountInString(s)+3)/4`。**当前实现**：`EstimateTokens` 对 **CJK-heavy rune** 计 **1 单位**，其余 rune 仍用 **`(other+3)/4`**；见源码与 **§8.1**。

**准确性对比**：

| 内容类型 | 典型子词/rune（嵌入侧，量级） | 旧估算单位/rune（runes/4） | 误差（旧） | 对向量的影响（旧） |
|---------|----------------|----------------|------|------|
| 中文纯文本 | ~1.0 | 0.25 | **低估 4x** | 合并过大 → **截断** → 向量丢尾部 |
| 英文纯文本 | ~0.25 | 0.25 | 准确 | ✅ |
| 代码（英文+符号） | ~0.3-0.5 | 0.25 | 低估 20-100% | 可能略超截断 |
| 混合内容（中英+代码） | ~0.5-0.8 | 0.25 | 低估 2-3x | 截断 / 粒度失衡 |

**中文文档场景示例**：
- maxTokens = 500
- 算法认为可容纳：500 × 4 = 2000 runes
- 中文实际可容纳：~500 runes（1 rune ≈ 1 token）
- **超发量**：2000 / 500 = **4 倍！**

**影响**（历史问题；**已实施** 混合 `EstimateTokens` 后主要转向 **嵌入侧**）：
- 子词数仍 **超过** embedder 上限时 → **静默截断** → 向量 **未覆盖** 尾部正文。
- 子词数 **远低于** 上限时 → 单向量 **信息密度低**，对稠密检索不利。
- 「检索片段过长」若指 **BM25 文本侧**：过长章节也可能增加关键词噪声；与向量 **序列长度** 是不同维度，需分别看待。

---

### 2.4 问题 4："爆炸"策略过于激进

**严重等级**: 🟡 **中**

**当前"爆炸"逻辑**：
```go
// 内部节点超过 maxTokens 时
if mergedTokens > maxTokens {
    // 父节点自身成为章节
    // 子节点各自独立输出
    // ❌ 没有"部分合并"策略
}
```

**问题场景**：
```markdown
## 第一章
简短介绍...（50 tokens）

### 1.1 小节 A
内容...（100 tokens）

### 1.2 小节 B
内容...（100 tokens）

### 1.3 小节 C
内容...（100 tokens）
```

**maxTokens = 300 时的行为**：
- 总计 = 50 + 100 + 100 + 100 = 350 > 300
- **爆炸**：`第一章`(50t) + `1.1`(100t) + `1.2`(100t) + `1.3`(100t) = 4 个章节
- **向量索引问题**：
  - 4 个短章节 → 4 个稀疏向量（信息密度低）
  - 本来可以合并为 2 个稠密向量（250t + 100t）

**缺失的策略**：
- 没有"最佳子集合并"算法
- 没有"贪心合并直到接近 maxTokens"的逻辑

---

### 2.5 问题 5：标题识别不完整 **[已部分实施]**

**严重等级**: 🟢 **低**（Setext 与部分编号误判已缓解；Setext/HR 边界与中文列表仍见上文）

**未识别的标题类型**：

```markdown
Setext 标题（未识别）
===================

Setext 子标题（未识别）
-----------------------
```

**误判的场景**：
```markdown
1. 第一步操作
2. 第二步操作
3. 第三步操作
```

**源码中的实际规则（节选）**：`parseNumberedOutlineHeading` 使用正则 `numberedHeadingSingle` / `numberedHeadingMulti`，并对 remainder 做 `restIsListLike`（以 `*`/`_` 开头等）拒绝；**2026-04-18** 起对 **remainder 首 rune 为数字** 的单行 `N.` 形式不再判为大纲标题，以减轻 `1. 2. …` 嵌套列表误判。**仍可能**将「`1. 第一步`」等中文步骤判为标题（首字非 ASCII 数字），属 **[仍开放]**。

**[文档勘误]**：旧版此处虚构的 `strings.Contains(line, "*")` 并非仓库实现。

---

### 2.6 问题 6：滑动步幅固定为 100 tokens

**严重等级**: 🟡 **中**

**当前配置**：
```go
const defaultColdMarkdownSlidingStrideTokens = 100
```

**问题**：
- 对于 maxTokens = 200 的场景，stride = 100 意味着 **50% 重叠**
- 对于 maxTokens = 1000 的场景，stride = 100 意味着 **10% 重叠**
- **重叠比例不固定**，影响检索质量

**更好的策略**：固定重叠比例（如 20%），而非固定 token 数

---

## 三、复杂度分析

### 3.1 时间复杂度

| 阶段 | 复杂度 | 说明 |
|------|--------|------|
| `parseSplitTree` | O(n) | 单行扫描，n = rune 数 |
| `postOrderMergeSplit` | O(n) | 每个 rune 被写入 strings.Builder 常数次 |
| `splitOversizedRaw` | O(n) | UTF-8 前向滑窗（无整篇 `[]rune` 分配） |
| **总体** | **O(n)** | 文档大小线性 |

### 3.2 空间复杂度

| 阶段 | 复杂度 | 说明 |
|------|--------|------|
| 标题树 | O(h) | h = 标题节点数 |
| 合并字符串 | O(n) | 最坏情况全量复制 |
| 输出切片 | O(c) | c = 最终章节数 |
| **总体** | **O(n)** | 文档大小线性 |

---

## 四、优化方案

### 4.1 方案 A：段落感知的自顶向下分割（推荐）

**核心思想**：从根开始，尽量保持完整章节；如果太大，再向下分割

```go
func improvedSplit(docID, title, markdown string, maxTokens int) []ColdChapter {
    // Stage 1: 构建标题树
    tree := parseSplitTree(markdown)
    
    // Stage 2: 自顶向下分割（替代自底向上合并）
    return splitTopDown(tree, maxTokens)
}

func splitTopDown(node *splitNode, maxTokens int) []ColdChapter {
    tokens := EstimateTokens(node.FullText())
    
    if tokens <= maxTokens {
        // ✅ 整个节点作为一个章节
        return []ColdChapter{{Path: node.Path, Text: node.FullText()}}
    }
    
    if len(node.Children) > 0 {
        // 有子节点，递归处理
        var chapters []ColdChapter
        for _, child := range node.Children {
            chapters = append(chapters, splitTopDown(child, maxTokens)...)
        }
        return chapters
    }
    
    // 叶子节点且超大 → 段落分割
    return splitByParagraphs(node, maxTokens)
}
```

**改进点**：
1. ✅ 避免父节点成为空章节
2. ✅ 自顶向下更符合直觉
3. ✅ 保持完整章节结构优先

---

### 4.2 方案 B：精确 Token 计数（中文优化）

**混合估算策略**：

```go
func EstimateTokens(s string) int {
    if s == "" { return 0 }
    
    runes := utf8.RuneCountInString(s)
    chineseRunes := countChineseRunes(s)  // CJK 字符
    otherRunes := runes - chineseRunes
    
    // 中文：1 rune ≈ 0.8-1.0 token (GPT-4)
    // 英文/代码：4 runes ≈ 1 token
    return chineseRunes*1 + otherRunes/4
}

func countChineseRunes(s string) int {
    count := 0
    for _, r := range s {
        if unicode.Is(unicode.Han, r) || 
           (r >= '\u2E80' && r <= '\u9FFF') ||
           (r >= '\uF900' && r <= '\uFAFF') ||
           (r >= '\uFF00' && r <= '\uFFEF') {
            count++
        }
    }
    return count
}
```

**预期提升**：
- 中文文档估算准确率：~50% → ~85%
- 避免严重超发

---

### 4.3 方案 C：结构感知的滑动窗口

**保护不可切分的结构**：

```go
type markdownBlock struct {
    Type string  // "paragraph", "code", "table", "list"
    Text string
}

func splitOversizedRaw(text string, maxTokens, stride int) []ColdChapter {
    // 1. 解析为结构块
    blocks := parseMarkdownBlocks(text)
    
    // 2. 贪心组合块
    var chapters []ColdChapter
    current := strings.Builder{}
    currentTokens := 0
    
    for _, block := range blocks {
        blockTokens := EstimateTokens(block.Text)
        
        // 不可切分的块（代码、表格）必须完整保留
        if block.Type == "code" || block.Type == "table" {
            if blockTokens > maxTokens {
                // 单个代码块超过限制 → 只能切断（罕见）
                chapters = append(chapters, splitCodeBlock(block, maxTokens)...)
            } else if currentTokens + blockTokens > maxTokens {
                // 保存当前，开启新章节
                chapters = append(chapters, ColdChapter{Text: current.String()})
                current.Reset()
                currentTokens = 0
                current.WriteString(block.Text)
                currentTokens = blockTokens
            } else {
                current.WriteString(block.Text)
                currentTokens += blockTokens
            }
        } else {
            // 可切分的块（段落、列表项）
            if currentTokens + blockTokens > maxTokens && currentTokens > 0 {
                chapters = append(chapters, ColdChapter{Text: current.String()})
                current.Reset()
                currentTokens = 0
            }
            current.WriteString(block.Text)
            currentTokens += blockTokens
        }
    }
    
    // 保存最后一个章节
    if currentTokens > 0 {
        chapters = append(chapters, ColdChapter{Text: current.String()})
    }
    
    return chapters
}
```

**保护的结构**：
- ✅ 代码块（ fenced code blocks ）
- ✅ 表格（ markdown tables ）
- ✅ 列表（避免单项被切分）

---

### 4.4 方案 D：混合策略（最佳实践）

**多阶段降级策略**：

```
阶段 1: 标题层级分割
  ├── 章节 ≤ maxTokens → 保留完整章节
  └── 章节 > maxTokens → 进入阶段 2

阶段 2: 段落组合分割
  ├── 段落组合 ≤ maxTokens → 合并段落
  └── 单个段落 > maxTokens → 进入阶段 3

阶段 3: 滑动窗口兜底
  └── 按 rune 窗口切分（带重叠）
```

**实现代码**：

```go
func MarkdownChapters(docID, title, markdown string, maxTokens int) []Chapter {
    // Stage 1: 标题树分割
    tree := parseHeadingTree(markdown)
    chapters := splitByTree(tree, maxTokens)
    
    // Stage 2: 段落分割
    for i, ch := range chapters {
        if ch.tokens > maxTokens {
            chapters[i] = splitByParagraphs(ch, maxTokens)
        }
    }
    
    // Stage 3: 滑动窗口兜底
    for i, ch := range chapters {
        if ch.tokens > maxTokens {
            chapters[i] = splitBySlidingWindow(ch, maxTokens, stride)
        }
    }
    
    return chapters
}
```

---

## 五、优化优先级建议

### P0（最高优先级）：段落感知分割

**原因**：
- 代码块被切断是最严重的问题
- 实现成本：中等（需新增 parseMarkdownBlocks）
- 收益：保护代码完整性，提升检索质量

### P1：改进合并策略

**原因**：
- 避免空章节，改善长文档结构
- 实现成本：低（修改 postOrderMergeSplit 逻辑）
- 收益：减少无意义章节

### P2：精确 Token 估算

**原因**：
- 中文文档场景必须修复
- 实现成本：低（混合估算）
- 收益：避免超发/欠发

### P3：结构保护（代码块、表格）

**原因**：
- 特殊结构保护
- 实现成本：中（需解析 Markdown 结构）
- 收益：保护表格、代码块完整性

### P4：Setext 标题支持

**原因**：
- 完整性提升
- 实现成本：低
- 收益：支持更多 Markdown 风格

---

## 六、关键代码路径

### 6.1 主要文件

| 文件 | 作用 |
|------|------|
| `internal/storage/coldindex/markdown_chapter_splitter_impl.go` | **核心算法** |
| `internal/storage/coldindex/markdown_chapters.go` | ColdChapter → types.Chapter 转换 |
| `internal/storage/coldindex/cold_index_impl.go` | IColdIndex 接口实现 |
| `internal/storage/coldindex/chapter_split_stride.go` | 滑动步幅配置 |
| `pkg/markdown/chapter_display.go` | 标题显示格式化 |

### 6.2 关键函数调用链

```
IColdIndex.MarkdownChapters(docID, title, markdown)
  └── cold_index_impl.go: MarkdownChapters()
      └── markdown_chapters.go: MarkdownChaptersFromSplit()
          └── markdown_chapter_splitter_impl.go: MarkdownSplitter.Split()
              ├── Phase 1: parseSplitTree()
              ├── Phase 2: postOrderMergeSplit()
              │   └── buildMergedChapterBody()
              └── Phase 3: splitOversizedRaw()
```

---

## 七、测试建议

### 7.1 需要补充的测试场景

| 场景 | 测试内容 | 期望结果 |
|------|---------|---------|
| 中文长文档 | 纯中文，5000 字 | 按 maxTokens 正确分割，无超发 |
| 代码块文档 | 含 fenced code block | 代码块不被切断 |
| 三级标题文档 | H1 → H2 → H3 | 父节点不成为空章节 |
| Setext 标题 | === 和 --- 标题 | 正确识别为标题 |
| 编号列表 | 1. 2. 3. | 不被误判为标题 |
| 混合内容 | 中文+英文+代码 | Token 估算准确 |
| 超大章节 | 单个章节 > maxTokens × 3 | 段落分割正确 |

---

## 八、附录：当前代码关键片段

### 8.1 Token 估算（当前实现）

```go
// internal/storage/coldindex/markdown_chapter_splitter_impl.go
func isCJKHeavyRune(r rune) bool { /* Han, Kana, Hangul, CJK symbols, fullwidth */ }

func EstimateTokens(s string) int {
    // cjk runes -> +1 unit each; other runes -> +(other+3)/4 units
}
```

### 8.2 滑动窗口（当前实现）

```go
// splitOversizedRaw：winRunes = maxTokens * 4，UTF-8 下用 utf8Pos / advanceUtf8Pos 取子串，无整篇 []rune(text)。
```

### 8.3 "爆炸"策略（逻辑概要）

```go
// postOrderMergeSplit：rawSplitChapter 与 buildMergedChapterBody
if EstimateTokens(combined) <= maxTokens && n.level > 0 {
    return []rawSplitChapter{{pathTitles: n.pathTitles, text: combined}}
}
// 否则：local -> splitOversizedRaw；再并列追加各子树 postOrder 结果
```

---

## 九、结论（含 2026-04-18 复核更新 + 向量索引场景澄清）

### 9.1 关键澄清：向量索引场景

**重申**：MarkdownChapters 的产出用于 **HNSW 向量索引**（`cold_vector_index_hnsw_impl.go`），而非 LLM RAG。因此：
- **核心目标**：在 512 token 预算内，生成高质量的向量嵌入
- **完整性要求**：章节不应在代码块/表格中间切断（向量编码半截语义）
- **密度要求**：章节应尽可能接近 512 token（避免稀疏向量）

### 9.2 问题状态汇总

原列 **6 项** 中：
- **#3 中文 token 低估** → 已通过 **`EstimateTokens` 混合单位** 缓解 **[已实施]**
- **#5 标题识别** → **Setext** 与 **部分编号误判** 已缓解 **[已部分实施]**
- **#1/#4 合并策略** → **[仍开放]**：导致稀疏向量（短章节）和碎片化索引
- **#2 滑窗碎代码** → **[仍开放]**：向量编码半截代码块，检索质量差
- **#6 stride** → **[仍开放]**：固定 100 token 步长，重叠比例不灵活

### 9.3 向量索引场景下的优先级（修订）

| 优先级 | 问题 | 向量索引影响 | 实施成本 |
|-------|------|------------|---------|
| **P0** | #2 滑窗碎代码 | 🔴 向量编码半截语义，检索返回垃圾 | 中 |
| **P1** | #1/#4 合并策略 | 🟡 短章节 → 稀疏向量 → 匹配相关性低 | 低 |
| **P2** | #3 Token 估算 | 🟡 超发 → MiniLM 截断 → 丢失尾部 | 低（已部分实施）|
| **P3** | #6 stride | 🟢 重叠比例影响检索召回 | 低 |
| **P4** | #5 标题识别 | 🟢 结构完整性，间接影响向量 | 低（已部分实施）|

### 9.4 预期收益（向量索引场景）

- **P0（结构感知滑窗）**：代码块/表格完整保留 → 向量编码完整语义 → 检索相关性 **+30-50%**
- **P1（贪心合并）**：减少稀疏短章节 → 向量信息密度提升 → HNSW 检索质量 **+10-20%**
- **P2（精确 token）**：避免 MiniLM 截断 → 尾部信息不丢失 → 长文档召回 **+15-25%**

**注**：以上收益为估算，需通过 A/B 测试验证。
