# 文档

使用 TierSum 的完整指南。

---

## 目录

- [快速开始](#快速开始)
- [摄入文档](#摄入文档)
- [查询](#查询)
- [热 / 冷分层](#热--冷分层)
- [认证](#认证)
- [文档 API](#文档-api)
- [查询 API](#查询-api)
- [MCP 协议](#mcp-协议)

---

## 快速开始

几分钟内在本地运行 TierSum。

### 前置条件

- Go 1.23 或更高版本
- Make
- OpenAI API 密钥（或 Anthropic，或本地 Ollama）

### 安装

```bash
# 克隆仓库
git clone https://github.com/tiersum/tiersum.git
cd tiersum

# 复制并编辑配置
cp configs/config.example.yaml configs/config.yaml
# 编辑 configs/config.yaml 并设置您的 LLM API 密钥

# 构建
make build

# 运行
make run
```

### 初始化

在浏览器中打开 http://localhost:8080。完成初始化向导以创建第一位管理员用户。

> 出于安全考虑，初始化端点默认仅可从本地主机访问。

### 第一篇文档

导航到文库页面并点击「添加文档」。粘贴 Markdown 内容并选择摄入模式：

- **自动** — 让 TierSum 根据内容长度和配额决定
- **热** — 强制完整 LLM 分析（查询效果更好，消耗配额）
- **冷** — 仅索引（摄入更快，BM25 + 向量搜索）

---

## 摄入文档

TierSum 摄入 Markdown 文档，并根据所选模式进行处理。

### 支持的格式

目前，TierSum 支持 Markdown（`.md`、`.markdown`）文档。解析器识别 ATX 标题（`#`、`##` 等）来拆分章节。

### 摄入模式

<div class="p-4 rounded-lg bg-amber-500/10 border border-amber-500/20">
<strong class="text-amber-200">热 — 完整 LLM 分析</strong><br>
生成文档摘要、章节摘要和目录标签。最适合频繁查询的文档。计入每小时配额。
</div>

<div class="p-4 rounded-lg bg-sky-500/10 border border-sky-500/20 mt-4">
<strong class="text-sky-200">冷 — 仅索引</strong><br>
拆分为章节，并使用 BM25 + 向量搜索进行索引。入库时无 LLM 调用。最适合大型存档和成本敏感场景。
</div>

<div class="p-4 rounded-lg bg-slate-700/30 border border-slate-600/30 mt-4">
<strong class="text-slate-200">自动 — 智能选择</strong><br>
如果内容长度 > 5000 字符且配额允许，则选择热；否则冷。推荐用于大多数使用场景。
</div>

### API 示例

```http
POST /api/v1/documents
Content-Type: application/json
X-API-Key: tsk_live_xxx

{
  "title": "架构决策记录",
  "content": "# 为何选择 TierSum……",
  "format": "markdown",
  "tags": ["架构", "adr"],
  "ingest_mode": "auto"
}
```

---

## 查询

TierSum 提供渐进式查询用于智能检索，以及直接冷搜索用于原始章节命中。

### 渐进式查询

推荐的查询方法。经历三个阶段：

1. **标签筛选** — 从查询中找到相关的目录标签
2. **文档排序** — 用 LLM 相关性对匹配文档打分
3. **章节选择** — 从排序后的文档中挑选最佳章节

```http
POST /api/v1/query/progressive
{
  "question": "认证是如何工作的？",
  "max_results": 10
}

// 返回：answer、steps、references
```

### 冷搜索

直接在冷章节索引上进行 BM25 + 向量混合搜索。返回原始章节文本，无 LLM 合成。

```http
GET /api/v1/cold/chapter_hits?q=auth,login&max_results=20
```

---

## 热 / 冷分层

TierSum 的核心成本优化机制。文档可以是热（完整分析）或冷（仅索引）。

### 晋升

`query_count >= 3` 的冷文档会自动加入晋升队列。后台作业每 5 分钟运行一次，将队列中的文档晋升为热，运行完整的 LLM 分析。

### 配额管理

热摄入受速率限制以控制 LLM 成本。默认：每小时 100 篇文档。在 `GET /api/v1/quota` 查看当前配额。

---

## 认证

TierSum 使用双轨认证：程序用 API 密钥，人类用浏览器会话。

### API 密钥

在每个请求中通过标头包含：

```http
X-API-Key: tsk_live_xxx
# 或
Authorization: Bearer tsk_live_xxx
```

作用域：`read`（GET + 查询）、`write`（+ 摄入）、`admin`（完全访问）。

### 浏览器会话

用于 Web UI 的 HttpOnly Cookie。支持通行密钥（WebAuthn）进行无密码认证。

---

## 文档 API

### 创建文档

```http
POST /api/v1/documents
Content-Type: application/json
X-API-Key: tsk_live_xxx

{
  "title": "string",
  "content": "string",
  "format": "markdown",
  "tags": ["string"],
  "ingest_mode": "auto"
}
```

### 列出文档

```http
GET /api/v1/documents?max_results=100
```

### 获取文档

```http
GET /api/v1/documents/:id
```

### 获取文档章节

```http
GET /api/v1/documents/:id/chapters
```

---

## 查询 API

### 渐进式查询

```http
POST /api/v1/query/progressive
Content-Type: application/json
X-API-Key: tsk_live_xxx

{
  "question": "string",
  "max_results": 100
}
```

### 冷章节命中

```http
GET /api/v1/cold/chapter_hits?q=keywords&max_results=20
```

---

## MCP 协议

TierSum 实现了模型上下文协议，用于 AI 代理集成。

### 端点

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| GET | `/mcp/sse` | SSE 流 |
| POST | `/mcp/message` | JSON-RPC 消息 |

### 配置

```yaml
mcp:
  enabled: true
  api_key: ${TIERSUM_API_KEY}  # 可选覆盖
```
