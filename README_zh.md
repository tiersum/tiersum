# TierSum

> **分层摘要知识库** —— 基于多层抽象与热/冷文档分层的文档检索系统；**不采用**将全文任意切碎后再做向量检索的典型 RAG 流程，而是通过分层摘要与标签导航组织知识。

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-1.0-6E49CB)](https://modelcontextprotocol.io)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) | [简体中文](README_zh.md)

---

## 为何选择 TierSum？

传统 RAG 往往把文档切成任意片段，**层级语境与结构语义容易丢失**。**TierSum 通过分层摘要 + 标签导航保留知识结构**：

```
┌─────────────────────────────────────────────────────────────┐
│  主题 Topics（LLM 归类）                                      │
│  ├── Cloud Native                                           │
│  │      └── 目录标签: kubernetes, docker, helm                 │
│  └── Programming Languages                                  │
│         └── 目录标签: golang, python, rust                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────┐
│  Document Summary                   │  ← 全局（文档级）视角
├─────────────────────────────────────┤
│  Chapter Summary                    │  ← 结构（章节级）视角
├─────────────────────────────────────┤
│  Source Text                        │  ← 原文
└─────────────────────────────────────┘
```

**用语：** **目录标签（catalog tags）** 指库内去重后的标签名（`tags` 表：文档计数、可选 `topic_id`）。**主题（topics）** 是 LLM 将目录标签归入的「主题/门类」，用于浏览与在目录标签很多时辅助渐进式查询收窄。热文档上的 **tags** 字段会并入同一套目录。

**查询沿层级逐步收窄**：**标签 → 文档 → 章节**（目录标签很多时，服务可先按 **主题** 收窄候选标签），每步 LLM 相关性评分；**不是**仅靠向量相似度「猜」片段 —— 而是 **可解释的层级导航**。

---

## 核心特性

| 特性 | 说明 |
|:--------|:------------|
| **热/冷分层** | 热文档：完整 LLM 分析；冷文档：BM25 + 向量混合检索 |
| **三级摘要** | 文档 → 章节 → 原文，由 LLM 生成 |
| **主题 + 目录标签** | LLM **主题重归类** 将目录标签划入 `topics`；`GET /tags` 可用 `topic_ids` 限定范围 |
| **渐进式查询** | **标签 → 文档 → 章节**；目录标签很多时可先经 **主题** 收窄 |
| **主题自动重归类** | 定时或手动 `POST /api/v1/topics/regroup` 按目录标签刷新主题 |
| **BM25 + 向量混合** | 对冷文档按 **章节** 建索引，关键词 + 语义混合检索，命中返回 **整章正文** |
| **RAG 替代思路** | 避免无结构切碎，尽量保留上下文 |
| **双接口** | REST API + MCP，便于智能体集成 |
| **Web 界面** | Vue 3 CDN + Tailwind + DaisyUI 暗色主题 |
| **Markdown 优先** | 针对 `.md` 优化；可扩展其它格式（规划中） |
| **增量更新** | 智能 diff、仅对变更部分重摘要（**规划中**） |

---

## 热 / 冷文档分层

TierSum 用两层策略平衡 **LLM 成本** 与 **检索效果**：

### 热文档（完整分析）
- ✅ 完整 LLM 分析：文档级 + 章节级摘要  
- ✅ 最多约 10 个自动标签（以实际服务逻辑为准）  
- ✅ 查询时可用 LLM 参与过滤  
- ✅ 持久化在数据库，带分层摘要  
- ⚡ 受 **配额** 限制（默认约 100 次/小时，见配置）  

**入库时热/冷**（`ingest_mode`）：`hot` 强制热路径；`cold` 强制冷路径；`auto`（默认）为已有预置 summary+chapters 则热，否则在配额允许且正文长度大于阈值（如 5000 字符）时为热，否则冷。旧字段 `force_hot=true` 等价于 `ingest_mode: hot`。

### 冷文档（轻量存储）
- ✅ 最小化处理，不做完整 LLM 分析  
- ✅ Bleve（BM25）+ HNSW 混合检索  
- ✅ 按标题树切分为 **章节** 后建索引；检索结果带 **完整章节正文**  
- ✅ 查询次数达到阈值（如 3 次）后可自动晋升为热文档  
- ⚡ **不消耗** 热文档配额  

**存储**：冷文档索引（进程内 Bleve + HNSW）+ 固定维度嵌入（实现为 384 维；代码在 `internal/storage/coldindex`）。

```
┌─────────────────────────────────────────────────────────────┐
│                    Hot Documents                            │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Full LLM Analysis → Tags + Summaries + Chapters     │  │
│  │  Progressive Query（标签→文档→章节；主题辅助）        │  │
│  └───────────────────────────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                    Cold Documents                           │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  BM25 + Vector Hybrid Search                          │  │
│  │  按章节建索引，命中返回整章正文                         │  │
│  │  Auto-promote after 3 queries → Hot                   │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 快速开始

### 前置要求

- Go 1.23+（SQLite 需 **开启 CGO**）  
- 数据库：SQLite（默认）或 PostgreSQL（可选）  
- LLM：OpenAI 或 Anthropic 等（见 `configs/config.example.yaml`）  

### 安装

```bash
# 克隆仓库
git clone https://github.com/tiersum/tiersum.git
cd tiersum

# 安装 Go 依赖
make deps

# 复制并编辑配置
cp configs/config.example.yaml configs/config.yaml

# 设置环境变量
export OPENAI_API_KEY="your-api-key"
# 或
export ANTHROPIC_API_KEY="your-api-key"

# 构建（内含嵌入的前端静态资源）
make build

# 或使用 Docker Compose
cd deployments/docker && docker-compose up -d
```

### 配置示例

**SQLite（默认，零额外服务）：**

```yaml
# configs/config.yaml
server:
  port: 8080

llm:
  provider: openai
  openai:
    api_key: ${OPENAI_API_KEY}
    model: gpt-4o-mini

storage:
  database:
    driver: sqlite3
    dsn: ./data/tiersum.db

quota:
  per_hour: 100  # 每小时允许「热处理」的文档次数上限（见实现）

documents:
  tiering:
    hot_content_threshold: 5000   # 超过该字数倾向走热路径（仍受配额等约束）
    cold_promotion_threshold: 3   # 冷文档查询次数达到该值可触发晋升（见后台任务）
```

**PostgreSQL（可选，高并发场景）：**

```yaml
storage:
  database:
    driver: postgres
    dsn: postgres://user:password@localhost:5432/tiersum?sslmode=disable
```

### 冷文档向量（MiniLM + ONNX Runtime）

冷文档内存索引的语义向量使用磁盘上的 **all-MiniLM-L6-v2** ONNX 与 **ONNX Runtime** 动态库（**不**通过 `go:embed` 打包神经网络权重）。复制 `config.example.yaml` 后默认路径指向 `third_party/...`，需先在仓库根执行：

```bash
make fetch-onnxruntime   # 按本机平台下载 .so / .dylib
make fetch-minilm        # 从 Hugging Face 拉取 model.onnx 与 tokenizer.json
```

大文件 **不进 Git**；本地或 CI 需自行拉取。**Docker 镜像**构建时在镜像内执行与本地相同的 **`make fetch-onnxruntime`** / **`make fetch-minilm`**（同源脚本与版本），并把 `onnx_runtime_path` 写成对应 `linux_amd64` 或 `linux_arm64` 的 `third_party/...` 路径。若 MiniLM 加载失败且 `provider` 为 `auto`，会退回简单哈希向量。

详见 [third_party/onnxruntime/README.md](third_party/onnxruntime/README.md) 与 [third_party/minilm/README.md](third_party/minilm/README.md)。

### 启动服务

```bash
# 本地运行（后端 + 嵌入前端）
make run

# 或直接运行二进制
./build/tiersum --config configs/config.yaml

# 服务就绪后
# Web UI:   http://localhost:8080/
# REST API: http://localhost:8080/api/v1（初始化后需数据库 API Key，见下文）
# BFF:      http://localhost:8080/bff/v1（浏览器会话 Cookie）
# MCP SSE:  http://localhost:8080/mcp/sse
```

---

## 权限与访问（用户指南）

TierSum 将身份分为 **浏览器（人）** 与 **程序（脚本 / MCP）**：前者走 **`/bff/v1` + HttpOnly 会话 Cookie**，后者走 **`/api/v1` + 存于数据库的 API Key**。完整设计见 **[docs/AUTH_AND_PERMISSIONS.md](docs/AUTH_AND_PERMISSIONS.md)**（英文）；英文 README 中有对应的 **[用户向说明](README.md#access-control-and-permissions-user-guide)**。

### 首次初始化（bootstrap）

1. 浏览器打开站点（如 `http://localhost:8080/`）。若尚未初始化，会进入 **`/init`**。
2. 填写 **管理员用户名** 并执行初始化。响应中 **仅显示一次**：
   - **管理员访问令牌**（`ts_u_…`）— 用于网页 **登录**；
   - **初始 API Key**（如 `tsk_live_…`）— 用于 `curl`、自动化、MCP（请妥善保存，服务器不再明文返回）。

### 浏览器登录

1. 打开 **`/login`**，粘贴 **访问令牌**（管理员或其它由管理员创建的用户令牌）。
2. 登录成功后，浏览器持有 **`tiersum_session`** Cookie；前端对 **`/bff/v1`** 使用带 Cookie 的请求。

**设备数上限：** 每名用户可绑定的不同浏览器数量有上限（新用户默认见配置 **`auth.browser.default_max_devices`**）。超出时需在 **管理 → 设备与会话** 中退出旧设备，或由 **管理员** 在后台移除会话。

### 界面权限一览

| 区域 | 对象 | 说明 |
| ---- | ---- | ---- |
| 搜索、文档、标签 | 任意已登录 **user / admin** | 通过 `/bff/v1` 使用核心产品功能。 |
| **Management（管理）→ 可观测性**（`/observability`） | 任意已登录 **user / admin** | 监控快照、冷索引探测、存储的链路追踪（登录后顶栏 **Management** 下拉）。 |
| **Management（管理）→ 设备与会话**（`/settings`） | 所有人 | 管理别名、单设备退出、**退出我的全部设备**。**admin** 在此页可查看 **所有用户** 的浏览器会话。 |
| **Management（管理）→ 用户与 API 密钥**（`/admin`） | 仅 **`admin` 人角色** | 管理用户与令牌重置、创建/吊销 **API Key**、本页 **Devices** 标签查看全局会话等。 |
| **Management（管理）→ Configuration**（`/admin/config`） | 仅 **`admin` 人角色** | 只读脱敏生效配置（`GET /bff/v1/admin/config/snapshot`）。 |

### 调用 `/api/v1`（程序通道）

初始化后，请求需带头 **`X-API-Key`** 或 **`Authorization: Bearer`**（密钥来自 Admin 界面或初始化响应）：

```bash
export TIERSUM_API_KEY='tsk_live_你的密钥'

curl -sS -H "X-API-Key: $TIERSUM_API_KEY" http://localhost:8080/api/v1/documents
```

**Key 的 scope：** `read`（读与渐进查询）、`write`（含写入文档、触发 **主题重归类** `POST /topics/regroup`）、`admin`（在路由检查上包含 write 的 superset）。注意：这是 **服务密钥的 scope**，与网页里的 **人角色 admin** 不是同一概念。

### MCP

与 REST 使用 **同一套** 数据库 API Key：环境变量 **`TIERSUM_API_KEY`**，或配置 **`mcp.api_key`**。工具调用时的 scope 规则与 `/api/v1` 一致。

### 运维提示

- **重置用户访问令牌：** 管理后台 **Users → Reset token**（该用户所有浏览器会话失效）。
- **吊销程序密钥：** **API keys → Revoke**。
- **`GET /health`、`GET /metrics`：** 根路径，**无需** TierSum API Key。

---

## API 说明

**核心流程**（录入冷热判定、渐进式查询、标签聚类、热/冷检索、冷文档混合检索等）见 [docs/CORE_API_FLOWS.md](docs/CORE_API_FLOWS.md)（英文技术文档）。

**权限模型：** [docs/AUTH_AND_PERMISSIONS.md](docs/AUTH_AND_PERMISSIONS.md)；**使用步骤：** 上文 [权限与访问（用户指南）](#权限与访问用户指南)。

### REST API

初始化后，所有 `/api/v1` 受保护请求需携带 **`X-API-Key`** 或 **`Authorization: Bearer`**（见上文用户指南）。

```bash
# 录入文档（未传 tags 时通常由 LLM 生成）
curl -X POST http://localhost:8080/api/v1/documents \
  -H "X-API-Key: $TIERSUM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Kubernetes Architecture",
    "content": "# Kubernetes Architecture\n\n## Control Plane...",
    "format": "markdown",
    "ingest_mode": "hot"
  }'

# 渐进式查询（推荐）：同时覆盖热/冷路径上的结果
curl -X POST http://localhost:8080/api/v1/query/progressive \
  -H "X-API-Key: $TIERSUM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "question": "How does kube-scheduler work?",
    "max_results": 100
  }'

# 以下示例为节省篇幅未重复加 Header；实际调用请对每个 /api/v1 请求加上 -H "X-API-Key: $TIERSUM_API_KEY"。

# 批量检索（热/冷）
curl "http://localhost:8080/api/v1/hot/doc_summaries?tags=kubernetes,docker&max_results=100"
curl "http://localhost:8080/api/v1/hot/doc_chapters?doc_ids=uuid1,uuid2&max_results=100"
curl "http://localhost:8080/api/v1/hot/doc_source?chapter_paths=docId/chapter-title&max_results=100"
curl "http://localhost:8080/api/v1/cold/doc_source?q=scheduler,pods&max_results=100"

# 列出主题（topics）
curl "http://localhost:8080/api/v1/topics"

# 列出目录标签；可按主题过滤，参数为 topic_ids（逗号分隔），可选 max_results
curl "http://localhost:8080/api/v1/tags?topic_ids=topic-uuid-1,topic-uuid-2&max_results=100"

# 手动触发主题重归类（LLM 根据目录标签刷新 topics）
curl -X POST http://localhost:8080/api/v1/topics/regroup

# 获取单个文档 / 摘要
curl "http://localhost:8080/api/v1/documents/{id}"
curl "http://localhost:8080/api/v1/documents/{id}/summaries"

# 配额状态
curl "http://localhost:8080/api/v1/quota"
```

> **校验说明**：标签列表接口使用 **`topic_ids`**（复数、逗号分隔），与 `internal/api/handler_retrieval.go` 一致。

### MCP 工具（智能体）

MCP 工具名与入参与 **`/api/v1` 下 REST** 语义对齐（实现见 `internal/api/mcp.go`）。需配置 **`TIERSUM_API_KEY`** 或 **`mcp.api_key`**（与 REST 同一套数据库 API Key 与 scope）。成功/失败时返回体为与 REST 相同的 JSON 结构（`metrics` 为 Prometheus 文本）。

| MCP 工具名 | 对应 REST |
|------------|-----------|
| `api_v1_documents_post` | `POST /documents` |
| `api_v1_documents_list` | `GET /documents` |
| `api_v1_documents_get` | `GET /documents/:id`（参数 `id`） |
| `api_v1_documents_chapters_get` | `GET /documents/:id/chapters`（`id`） |
| `api_v1_documents_summaries_get` | `GET /documents/:id/summaries`（`id`） |
| `api_v1_query_progressive_post` | `POST /query/progressive`（`question`，`max_results`） |
| `api_v1_tags_get` | `GET /tags`（可选 `topic_ids`、`max_results`） |
| `api_v1_topics_get` | `GET /topics` |
| `api_v1_topics_regroup_post` | `POST /topics/regroup` |
| `api_v1_hot_doc_summaries_get` | `GET /hot/doc_summaries`（`tags`，`max_results`） |
| `api_v1_hot_doc_chapters_get` | `GET /hot/doc_chapters`（`doc_ids`，`max_results`） |
| `api_v1_hot_doc_source_get` | `GET /hot/doc_source`（`chapter_paths`，`max_results`） |
| `api_v1_cold_doc_source_get` | `GET /cold/doc_source`（`q`，`max_results`） |
| `api_v1_quota_get` | `GET /quota` |
| `api_v1_metrics_get` | `GET /metrics` |

**Claude Desktop 配置示例：**

```json
{
  "mcpServers": {
    "tiersum": {
      "command": "npx",
      "args": ["-y", "@anthropics/mcp-proxy", "http://localhost:8080/mcp/sse"]
    }
  }
}
```

---

## 架构

TierSum 采用 **五层架构** + **接口与实现分离**（`I*` 接口 + `svcimpl` 等实现包）：

```
┌─────────────────────────────────────────────────────────────┐
│  Client Layer                                                │
│  (REST API / MCP / Web UI)                                  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  API Layer        (REST handlers + MCP server)              │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  Service Layer    (Business logic + LLM integration)        │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  Storage Layer    (DB repositories + Cache + Memory Index)  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  Client Layer     (LLM providers)                           │
└─────────────────────────────────────────────────────────────┘
```

📚 **更完整的目录约定、命令与端点说明见 [AGENTS.md](AGENTS.md)。**  
🔐 **权限与认证设计（英文）：** [docs/AUTH_AND_PERMISSIONS.md](docs/AUTH_AND_PERMISSIONS.md)；**中文使用步骤：** [上文「权限与访问（用户指南）」](#权限与访问用户指南)。

---

## Web 界面

基于 **Vue 3 CDN** 的单页应用（与 `cmd/web` 一致）。**界面与 BFF 路由对应关系**见 [cmd/web/FRONTEND.md](cmd/web/FRONTEND.md)。**登录与设备管理**见 [权限与访问（用户指南）](#权限与访问用户指南)。

### 产品介绍（`/#/about`）

- 中英双语（先英文后中文）：适用场景、热/冷路径通俗说明、适合谁使用  
- 不调用接口；系统完成首次初始化后**无需登录**即可访问  

### 查询页（`/#/`）
- 中央搜索框，调用渐进式查询 API  
- 分栏：左侧 AI 回答，右侧参考条目  
- 展示热/冷相关结果及相关性、来源标识  

### 文档页（`/#/docs`）
- 文档列表、检索/筛选、创建文档  
- 元数据：标题、标签、格式、状态、热度与查询统计等  

### 标签页（`/#/tags`）
- 左侧 **主题**（`GET /topics`），右侧当前主题下的 **目录标签**（`GET /tags?topic_ids=…`）及文档计数  
- 「重归类」调用 `POST /topics/regroup`  

### 技术栈
- **框架**：Vue 3（CDN）  
- **路由**：Vue Router 4（CDN）  
- **样式**：Tailwind CSS（CDN）  
- **组件**：DaisyUI（CDN）  
- **Markdown**：Marked.js（CDN）  
- **主题**：Slate 系暗色  
- **发布**：由 `cmd` 包内 `//go:embed web/*` 嵌入二进制（见 `cmd/main.go`）  

---

## 冷文档章节检索（简述）

冷文档 Markdown 按 **标题树 + 自下而上 token 合并**（`cold_index.markdown.chapter_max_tokens`）切成 **章节**。若单个叶子仍超预算，再用 **滑动窗口**（`cold_index.markdown.sliding_stride_tokens`，默认相邻窗起始相距 100 个估算 token；重叠约「预算 − 步长」）。路径为 **父级标题全路径 + 序号**（如 `docId/章节标题/1`）；无标题时增加合成根 **`__root__`**（如 `docId/__root__/1`）。

章节写入 **Bleve（BM25）** 与 **HNSW**（可选文本向量），`GET /api/v1/cold/doc_source` 做混合检索；每条结果的 `context` 为对应 **整章正文**，而不是任意小块「片段」。

### 与传统 RAG 的对比

| 维度 | 常见传统 RAG | TierSum（冷路径） |
|:--------|:------------|:--------------------|
| **检索单元** | 多为固定 char/token 块，与结构弱相关 | **Markdown 语义章节**（标题树）；超大叶子用 **可控滑动窗**，路径可寻址 |
| **结构保留** | 标题、列表、代码易被拦腰切断 | 优先在 **标题边界** 切段；仅超大叶子滑动，仍保留 **path** 便于对齐与排错 |
| **重叠策略** | 相邻块固定 overlap，主要为防断句 | 由 **窗长 − 步长** 决定（均可配置），偏「延续上下文」而非随机叠块 |
| **索引与融合** | 常以向量为主（BM25 可选） | **BM25 + 向量混合**，按章节路径去重融合 |
| **返回内容** | 小块拼进 prompt | 命中为 **整章正文** |
| **成本与可解释性** | 切块 + 向量化；信号多为相似度 | 冷入库无完整 LLM；**path** 与可选 **source**（bm25 / vector / hybrid）便于解释 |

**传统 RAG 仍可能更合适**：无标题长文、强依赖任意位置极细粒度命中、或已深度绑定统一向量切块流水线。

**TierSum 冷方案更合适**：Markdown / 技术文档、希望 **少碎块、保层级**、冷侧 **控成本**，并与 **热路径**（分层摘要 + 标签渐进查询）形成同一套产品叙事。

**算法与实现细节：** [docs/COLD_INDEX_zh.md](docs/COLD_INDEX_zh.md)（中文）· [docs/COLD_INDEX.md](docs/COLD_INDEX.md)（English）

---

## 仓库结构（节选）

```
tiersum/
├── cmd/
│   ├── main.go               # entrypoint; //go:embed web/*
│   └── web/                    # Vue 3 CDN 前端（嵌入二进制）
│       ├── index.html          # importmap + `js/main.js`
│       └── js/                 # ESM：页面与 api_client
├── configs/
├── deployments/
│   └── docker/
├── db/
│   └── migrations/
├── internal/
│   ├── api/
│   ├── service/
│   ├── storage/
│   ├── client/
│   ├── job/
│   └── di/
├── pkg/
│   └── types/
├── go.mod
├── Makefile
├── README.md
├── README_zh.md
└── LICENSE
```

（细粒度文件列表以仓库当前树为准。）

---

## 开发命令

```bash
make test
make lint
make fmt
make dev          # 需安装 air
make build-all
```

---

## 路线图

与英文 README 一致：热/冷分层、混合检索、三级摘要、主题与目录标签、渐进式查询、REST+MCP、Vue CDN 界面等已勾选；OpenClaw 技能包、实时协作、多模态、企业 SSO 等为规划中项。

---

## 贡献

欢迎贡献。请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。

**适合入门的方向**：更多文档格式解析、本地 LLM 适配、Web UI 增强等。

---

## 许可证

[MIT License](LICENSE) © 2026 TierSum Contributors

---

## 致谢

- 思路受 [Anthropic's Contextual Retrieval](https://www.anthropic.com/news/contextual-retrieval) 启发  
- [MCP 协议](https://modelcontextprotocol.io)（Anthropic）  
- 构建使用 [Gin](https://gin-gonic.com)、[Goldmark](https://github.com/yuin/goldmark)、[Bleve](https://blevesearch.com)、[HNSW](https://github.com/coder/hnsw) 等（**具体依赖以 `go.mod` 与源码引用为准**）

---

### 翻译与一致性说明

- 本文与 [README.md](README.md) **同步意图**介绍产品；**行为细节以当前分支代码与 `AGENTS.md` 为准**。  
- 术语对照：**Progressive Query** → 渐进式查询；**Hot/Cold** → 热/冷（文档分层）；冷索引命中单位 → **章节**（整章正文）；**tier** → 层级/档。  
- 英文 README 中的 **URL、JSON 字段名、工具名** 保持英文，便于直接复制调用。
