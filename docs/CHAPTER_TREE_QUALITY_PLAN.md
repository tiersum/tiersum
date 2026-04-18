# 章节树质量改进计划（Chapter Tree Quality Plan）

**创建时间**: 2026-04-18  
**目标**: 提升 `parseSplitTree` 的标题识别准确率，减少误判与漏判  
**优先级**: 在保持高召回的同时，显著降低假阳性（false positive）章节边界  

---

## 1. 现状诊断

### 1.1 核心痛点（按影响排序）

| # | 痛点 | 误判场景示例 | 当前行为 | 影响 |
|---|------|-------------|---------|------|
| **1** | **编号列表误判为标题** | `1. 下载安装包` | 被识别为 `level 2` 大纲标题 | 列表被拆成多个伪章节，向量索引碎片化 |
| **2** | **缩进代码块内标题误判** | `// 步骤1：初始化` | 缩进代码块未被识别，内部含 `#` 或数字的行可能误判 | 代码被切成多个伪章节 |
| **3** | **Setext 与水平线混淆** | `---` 分隔线 | 被误判为 `level 2` Setext 标题 | 生成空标题节点，干扰合并 |
| **4** | **中文编号标题不支持** | `一、系统架构` | 不被识别，当作正文 | 中文文档章节结构丢失 |
| **5** | **标题层级推断过于简单** | `1.1` → `level 3` | 固定映射 `1 + dot_count` | 可能与 ATX `#` 层级冲突 |

### 1.2 根本原因

当前 `parseSplitTree`（`markdown_chapter_splitter_impl.go:243`）采用**单行正则匹配**策略：

```go
// 当前：逐行独立判断，无上下文
if headingLine.Match(trim) { ... }                 // ATX
if parseSetextUnderline(trim) { ... }              // Setext  
if parseNumberedOutlineHeading(trim) { ... }       // 编号大纲
```

**缺失**：
- ❌ 无上下文信息（前后行内容、列表嵌套深度）
- ❌ 无文档类型感知（API文档 vs 教程 vs 论文）
- ❌ 无中文格式支持（中文数字、中文标点）
- ❌ 无缩进代码块识别（仅支持围栏 ` ``` `）

---

## 2. 目标与约束

### 2.1 核心目标

**在"高召回 + 低误判"之间取得平衡**：
- 宁可漏判（少识别一个真标题），也不要误判（把一个列表项识别为标题）
- 减少伪章节数量，提升向量索引质量

### 2.2 约束条件

| 约束 | 说明 |
|------|------|
| **向后兼容** | 现有正确识别的标题不应被破坏 |
| **性能** | 解析速度不应显著下降（当前 O(n)） |
| **零配置优先** | 默认自动识别，配置为可选增强 |
| **错误可观测** | 提供调试接口，方便排查误判 |

---

## 3. 改进方案（分阶段实施）

### Phase 1: 上下文感知的编号列表识别（P0 - 最高优先级）

**目标**: 解决"编号列表误判为标题"这一最严重痛点  
**影响**: 显著减少伪章节，提升索引质量

#### 3.1.1 上下文结构

```go
// LineContext 提供单行判断所需的上下文信息
type LineContext struct {
    LineNum      int      // 行号（用于调试）
    Content      string   // 当前行内容（去首尾空格前）
    Trimmed      string   // 当前行内容（去首尾空格后）
    PrevLine     string   // 上一行（去首尾空格后）
    NextLine     string   // 下一行（去首尾空格后，需预读）
    
    // 状态机
    InFence      bool     // 是否在代码围栏 ``` 内
    InIndented   bool     // 是否在缩进代码块内
    ListDepth    int      // 当前列表嵌套深度
    
    // 统计信息
    HeadingCount int      // 已识别的标题数量
    LastHeadingLevel int  // 上一个标题的层级
}
```

#### 3.1.2 编号行判断规则（核心改进）

```go
// isNumberedListItem 判断是否为编号列表项（而非标题）
func isNumberedListItem(ctx LineContext) bool {
    trimmed := ctx.Trimmed
    
    // 规则1: 单行 N. / N.N. 格式
    if !numberedHeadingSingle.MatchString(trimmed) && 
       !numberedHeadingMulti.MatchString(trimmed) {
        return false
    }
    
    // 规则2: 上下文判断 - 连续编号 = 列表
    if hasSequentialNumbers(ctx.PrevLine, trimmed, ctx.NextLine) {
        return true
    }
    
    // 规则3: 内容特征 - 操作指令 = 列表
    if isOperationalStep(trimmed) {
        return true
    }
    
    // 规则4: 下一行缩进 = 列表
    if isNextLineIndented(ctx.NextLine) {
        return true
    }
    
    return false
}
```

**具体判断逻辑**:

| 模式 | 示例 | 上下文 | 判定 |
|------|------|--------|------|
| 连续编号 | `1. 步骤A`<br>`2. 步骤B` | 上一行/下一行也是 `N. 内容` | **列表** |
| 操作指令 | `1. 下载安装包` | 含动作动词（下载/运行/配置） | **列表** |
| 描述性标题 | `1. 系统概述` | 后接大段正文，无连续编号 | **标题** |
| 嵌套列表 | `1. 第一项` | 下一行缩进 `2. 子项` | **列表** |

#### 3.1.3 辅助判断函数

```go
// hasSequentialNumbers 检查是否存在连续编号
func hasSequentialNumbers(prev, curr, next string) bool {
    currNum := extractLeadingNumber(curr)
    if currNum <= 0 {
        return false
    }
    
    // 检查连续性（容忍空白行）
    if prevNum := extractLeadingNumber(prev); prevNum > 0 && prevNum+1 == currNum {
        return true
    }
    if nextNum := extractLeadingNumber(next); nextNum > 0 && currNum+1 == nextNum {
        return true
    }
    
    return false
}

// isOperationalStep 检查是否为操作步骤
func isOperationalStep(text string) bool {
    // 去除编号前缀
    content := removeNumberPrefix(text)
    
    // 检查是否以动作动词开头
    actionVerbs := []string{"下载", "安装", "运行", "配置", "设置", "创建", "删除", "更新", "启动", "停止"}
    for _, verb := range actionVerbs {
        if strings.HasPrefix(content, verb) {
            return true
        }
    }
    
    return false
}
```

#### 3.1.4 与其他标题类型的协同

```go
func shouldTreatAsHeading(ctx LineContext) bool {
    // ATX 标题（## 标题）优先，不受上下文影响
    if isATXHeading(ctx.Trimmed) {
        return true
    }
    
    // Setext 标题
    if isSetextHeading(ctx) {
        return true
    }
    
    // 编号大纲 - 先检查是否为列表
    if isNumberedListItem(ctx) {
        return false
    }
    
    // 剩余情况按现有规则判断
    return isNumberedOutlineHeading(ctx.Trimmed)
}
```

---

### Phase 2: 缩进代码块识别（P1）

**目标**: 解决"代码块内标题误判"  
**当前限制**: 仅识别围栏代码块（```），忽略缩进代码块（4+空格）

#### 3.2.1 缩进代码块检测

```go
func isIndentedCodeBlockStart(prevLine, currLine string) bool {
    // 规则: 4+空格开头 + 前一行空行
    if !strings.HasPrefix(currLine, "    ") {
        return false
    }
    
    // 前一行必须是空行（或文档开头）
    if prevLine != "" && strings.TrimSpace(prevLine) != "" {
        return false
    }
    
    return true
}

func isIndentedCodeBlockLine(line string, inBlock bool) bool {
    if !inBlock {
        return false
    }
    
    // 空行允许在代码块内（作为分隔）
    if strings.TrimSpace(line) == "" {
        return true
    }
    
    // 必须保持缩进或空行
    return strings.HasPrefix(line, "    ") || line == ""
}
```

#### 3.2.2 与围栏代码块的统一处理

```go
type CodeBlockState struct {
    Active      bool
    Type        string  // "fence" | "indented"
    FenceChar   string  // "`" | "~" (仅 fence 类型)
}

func updateCodeBlockState(state *CodeBlockState, prevLine, currLine string) {
    if state.Active {
        // 检查是否退出代码块
        if state.Type == "fence" && isFenceEnd(currLine, state.FenceChar) {
            state.Active = false
        } else if state.Type == "indented" && !isIndentedCodeBlockLine(currLine, true) {
            state.Active = false
        }
    } else {
        // 检查是否进入代码块
        if isFenceStart(currLine) {
            state.Active = true
            state.Type = "fence"
            state.FenceChar = extractFenceChar(currLine)
        } else if isIndentedCodeBlockStart(prevLine, currLine) {
            state.Active = true
            state.Type = "indented"
        }
    }
}
```

---

### Phase 3: Setext 与水平线区分（P2）

**目标**: 减少空标题节点，提升合并质量  
**问题**: `---` 既可能是水平线，也可能是 Setext 标题下划线

#### 3.3.1 改进的 Setext 判断

```go
func isSetextHeading(ctx LineContext) (title string, level int, ok bool) {
    // 严格规则:
    // 1. 当前行必须是下划线（=== 或 ---）
    // 2. 上一行必须是非空正文（且不是列表项、不是标题）
    // 3. 上一行下方不能是列表（否则可能是列表分隔线）
    
    underline := ctx.Trimmed
    prev := ctx.PrevLine
    
    // 检查下划线格式
    if !isSetextUnderline(underline) {
        return "", 0, false
    }
    
    // 检查上一行是否适合作为标题
    if prev == "" || isListItem(prev) || isHeading(prev) {
        return "", 0, false
    }
    
    // 提取层级
    level = 2 // 默认 ---
    if strings.HasPrefix(underline, "=") {
        level = 1
    }
    
    return strings.TrimSpace(prev), level, true
}
```

#### 3.3.2 水平线识别

```go
func isHorizontalRule(line string) bool {
    // 水平线特征:
    // 1. 仅由 - / * / _ 组成（可有空格）
    // 2. 至少 3 个字符
    // 3. 上一行是空行或标题（即不作为 Setext）
    
    trimmed := strings.TrimSpace(line)
    if len(trimmed) < 3 {
        return false
    }
    
    firstChar := trimmed[0]
    if firstChar != '-' && firstChar != '*' && firstChar != '_' {
        return false
    }
    
    // 检查是否全是同一种字符（允许空格）
    for _, ch := range trimmed {
        if ch != rune(firstChar) && ch != ' ' {
            return false
        }
    }
    
    return true
}
```

---

### Phase 4: 中文编号标题支持（P3）

**目标**: 支持中文技术文档常见格式

#### 3.4.1 中文标题模式

```markdown
一、系统架构          ← 一级标题（level 2）
（一）模块设计        ← 二级标题（level 3）
1. 子模块概述        ← 三级标题（level 4）
1.1 详细设计        ← 四级标题（level 5）

vs

第一步：安装环境      ← 操作步骤（正文，非标题）
（1）检查配置        ← 列表项（正文，非标题）
```

#### 3.4.2 识别规则

```go
var (
    chineseNumberedHeading = regexp.MustCompile(`^([一二三四五六七八九十]+)、(.+)$`)
    chineseParenHeading    = regexp.MustCompile(`^（([一二三四五六七八九十]+)）(.+)$`)
)

func parseChineseHeading(trimmed string) (title string, level int, ok bool) {
    // 规则1: 一、标题
    if m := chineseNumberedHeading.FindStringSubmatch(trimmed); m != nil {
        return trimmed, 2, true
    }
    
    // 规则2: （一）标题
    if m := chineseParenHeading.FindStringSubmatch(trimmed); m != nil {
        return trimmed, 3, true
    }
    
    return "", 0, false
}

// 区分标题与步骤
func isChineseStep(text string) bool {
    // 步骤特征:
    // - 以"第X步"或"步骤X"开头
    // - 后接冒号
    return strings.Contains(text, "第") && 
           strings.Contains(text, "步") &&
           strings.Contains(text, "：")
}
```

---

### Phase 5: 可配置的识别策略（P4）

**目标**: 适应不同文档类型（API文档 vs 教程）

#### 3.5.1 配置结构

```yaml
# config.yaml
cold_index:
  chapter_split:
    heading_recognition:
      atx: true                    # ## 标题（始终启用）
      setext: true                 # 标题\n===（默认启用）
      numbered_outline: true       # 1.1 标题（默认启用）
      chinese_numbered: false      # 一、标题（默认关闭，中文文档手动开启）
      strict_mode: false           # true = 更严格的判断，减少误判
      
      # 容错级别
      tolerance: "balanced"        # "strict" | "balanced" | "permissive"
      # strict: 宁可漏判，不要误判（推荐用于已知格式文档）
      # balanced: 平衡（默认）
      # permissive: 宁可误判，不要漏判（不推荐）
```

#### 3.5.2 运行时配置

```go
type HeadingRecognitionConfig struct {
    ATX                bool
    Setext             bool
    NumberedOutline    bool
    ChineseNumbered    bool
    StrictMode         bool
    Tolerance          string
}

func parseHeadingWithConfig(ctx LineContext, config HeadingRecognitionConfig) bool {
    if config.StrictMode {
        // 严格模式: 只认最明确的标题
        return isATXHeading(ctx.Trimmed) || 
               (config.Setext && isStrictSetext(ctx))
    }
    
    // 平衡模式: 默认逻辑
    if config.ATX && isATXHeading(ctx.Trimmed) {
        return true
    }
    
    if config.Setext && isSetextHeading(ctx) {
        return true
    }
    
    if config.NumberedOutline && !isNumberedListItem(ctx) {
        if _, _, ok := parseNumberedOutlineHeading(ctx.Trimmed); ok {
            return true
        }
    }
    
    if config.ChineseNumbered {
        if _, _, ok := parseChineseHeading(ctx.Trimmed); ok {
            return true
        }
    }
    
    return false
}
```

---

### Phase 6: 质量验证与调试机制（P5）

**目标**: 错误可观测，方便排查

#### 3.6.1 调试接口

```go
// ParseSplitTreeDebug 返回解析结果及调试信息
type ParseDebugInfo struct {
    TotalLines       int            // 总行数
    RecognizedHeadings []HeadingInfo // 识别出的标题
    SkippedLines     []LineInfo     // 被跳过的可疑行
    Warnings         []string       // 警告信息
}

type HeadingInfo struct {
    LineNum     int
    Level       int
    Title       string
    Type        string  // "atx" | "setext" | "numbered" | "chinese"
    Confidence  float64 // 置信度 (0.0-1.0)
}

func ParseSplitTreeDebug(markdown string) (*splitNode, *ParseDebugInfo, error) {
    // ... 解析逻辑 ...
    // 同时收集调试信息
}
```

#### 3.6.2 结构合理性检查

```go
func validateHeadingTree(root *splitNode) []string {
    var warnings []string
    
    // 检查1: 层级跳跃
    checkLevelJumps(root, &warnings)
    
    // 检查2: 空节点
    checkEmptyNodes(root, &warnings)
    
    // 检查3: 同级数量异常
    checkSiblingCount(root, &warnings)
    
    return warnings
}

func checkLevelJumps(node *splitNode, warnings *[]string) {
    // 层级不应跳跃超过1级
    // level 1 -> level 4 是异常
}
```

#### 3.6.3 日志输出

```go
// 解析时输出统计
logger.Info("chapter tree parsed",
    zap.Int("total_lines", debug.TotalLines),
    zap.Int("headings_found", len(debug.RecognizedHeadings)),
    zap.Int("warnings", len(debug.Warnings)),
)

// 警告示例
"Line 45: '1. 下载安装包' recognized as heading but appears to be list item (confidence: 0.3)"
"Line 67: Setext heading with empty title skipped"
"Line 120: Level jump detected (2 -> 5)"
```

---

## 4. 实施路线图

### 4.1 优先级矩阵

| 阶段 | 改进 | 影响 | 成本 | 优先级 | 状态 |
|------|------|------|------|--------|------|
| **P0** | 上下文感知编号识别 | 🔴 高（减少伪章节） | 中 | **最高** | 待实施 |
| **P1** | 缩进代码块识别 | 🟡 中（保护代码） | 低 | **高** | 待实施 |
| **P2** | Setext/水平线优化 | 🟡 中（减少空标题） | 低 | **中** | 待实施 |
| **P3** | 中文标题支持 | 🟡 中（支持中文文档） | 中 | **中** | 待实施 |
| **P4** | 可配置策略 | 🟢 低（适配不同场景） | 中 | **低** | 待实施 |
| **P5** | 质量验证机制 | 🟢 低（调试保障） | 低 | **低** | 待实施 |

### 4.2 验收标准

对于每个阶段，定义明确的验收标准：

**P0 验收标准**:
- [ ] 测试用例：`1. 下载安装包` + `2. 运行程序` 被识别为列表，不生成伪章节
- [ ] 测试用例：`1. 系统概述` + 大段正文 被正确识别为标题
- [ ] 现有正确识别的标题（如 `## 标题`）不受影响
- [ ] 性能：解析速度下降不超过 20%

**P1 验收标准**:
- [ ] 测试用例：4空格缩进的代码块内的 `#` 或数字不被误判为标题
- [ ] 测试用例：缩进代码块与围栏代码块等价处理

---

## 5. 关键决策记录

### 5.1 决策1: 容错策略

**选择**: "宁可漏判，不要误判"（strict-first）

**理由**:
- 漏判 = 少一个章节，正文合并到父章节 → 向量仍包含该内容
- 误判 = 生成伪章节 → 向量碎片化，检索质量下降

**实现**: 默认 `tolerance: "balanced"`，可配置为 `strict`

### 5.2 决策2: 上下文预读

**选择**: 单行预读（lookahead = 1）

**理由**:
- 足够判断连续性（编号列表）
- 不显著增加内存（只缓存下一行）
- 保持 O(n) 时间复杂度

**不选择**: 全文预分析（ lex-&-parse ）
- 过度设计，增加复杂度
- 性能开销大

### 5.3 决策3: 中文支持方式

**选择**: 通过配置启用（默认关闭）

**理由**:
- 避免误伤英文文档中的中文内容
- 中文格式多样性高，需要显式声明

---

## 6. 测试策略

### 6.1 测试文档集合

准备以下测试文档，覆盖常见场景：

```
testdata/chapter_tree/
├── api_document.md          # API文档（ATX为主）
├── tutorial_steps.md        # 教程（编号列表多）
├── chinese_technical.md     # 中文技术文档
├── mixed_content.md         # 混合内容（代码+文字）
├── edge_cases.md            # 边界情况（空标题、层级跳跃）
└── false_positive.md        # 误判场景（列表、代码注释）
```

### 6.2 回归测试

每次改进后运行：
1. 解析测试文档
2. 对比改进前后的标题识别结果
3. 确保：
   - 新发现的真标题数 > 0（召回提升）
   - 误判数 = 0 或显著减少（精确提升）
   - 原有正确识别不受影响

---

## 7. 附录：参考实现片段

### 7.1 改进后的 parseSplitTree 框架

```go
func parseSplitTree(markdown string) *splitNode {
    root := &splitNode{level: 0, title: ""}
    stack := []*splitNode{root}
    
    state := &ParserState{
        inFence:    false,
        inIndented: false,
        listDepth:  0,
    }
    
    lines := strings.Split(markdown, "\n")
    
    for i := 0; i < len(lines); i++ {
        ctx := LineContext{
            LineNum:  i,
            Content:  lines[i],
            Trimmed:  strings.TrimSpace(lines[i]),
            PrevLine: getPrevLine(lines, i),
            NextLine: getNextLine(lines, i),
        }
        
        // 更新代码块状态
        updateCodeBlockState(state, ctx)
        
        if state.inFence || state.inIndented {
            // 在代码块内，不识别标题
            appendToBody(stack, lines[i])
            continue
        }
        
        // 使用上下文感知判断
        if shouldTreatAsHeading(ctx) {
            // ... 现有逻辑 ...
        } else {
            appendToBody(stack, lines[i])
        }
    }
    
    return root
}
```

---

## 8. 版本记录

| 版本 | 日期 | 变更 |
|------|------|------|
| v1.0 | 2026-04-18 | 初始版本，基于代码分析制定 |

---

**下一步**: 待确认 P0 的详细设计后，开始实施 Phase 1。
