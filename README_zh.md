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
│  Tag Groups (L1)                                            │
│  ├── Cloud Native                                           │
│  │      └── Tags: kubernetes, docker, helm                  │
│  └── Programming Languages                                  │
│         └── Tags: golang, python, rust                      │
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

**查询沿层级逐步收窄**：先经 LLM 筛选标签，再到文档、章节，每一步都有相关性评分；**不是**仅靠向量相似度「猜」片段 —— 而是 **可解释的层级导航**。

---

## 核心特性

| 特性 | 说明 |
|:--------|:------------|
| **热/冷分层** | 热文档：完整 LLM 分析；冷文档：BM25 + 向量混合检索 |
| **三级摘要** | 文档 → 章节 → 原文，由 LLM 生成 |
| **两级标签** | L1 标签组 → L2 标签（自动生成） |
| **渐进式查询** | 每步用 LLM 过滤：标签 → 文档 → 章节 |
| **标签自动聚类** | LLM 将相关标签归入 L1 类别 |
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
│  │  Progressive Query (L1→L2→Docs→Chapters)             │  │
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

# 数据库迁移
make migrate-up

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
# REST API: http://localhost:8080/api/v1
# MCP SSE:  http://localhost:8080/mcp/sse
```

---

## API 说明

**核心流程**（录入冷热判定、渐进式查询、标签聚类、热/冷检索、冷文档混合检索等）见 [docs/CORE_API_FLOWS.md](docs/CORE_API_FLOWS.md)（英文技术文档）。

### REST API

```bash
# 录入文档（未传 tags 时通常由 LLM 生成）
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Kubernetes Architecture",
    "content": "# Kubernetes Architecture\n\n## Control Plane...",
    "format": "markdown",
    "ingest_mode": "hot"
  }'

# 渐进式查询（推荐）：同时覆盖热/冷路径上的结果
curl -X POST http://localhost:8080/api/v1/query/progressive \
  -H "Content-Type: application/json" \
  -d '{
    "question": "How does kube-scheduler work?",
    "max_results": 100
  }'

# 批量检索（热/冷）
curl "http://localhost:8080/api/v1/hot/doc_summaries?tags=kubernetes,docker&max_results=100"
curl "http://localhost:8080/api/v1/hot/doc_chapters?doc_ids=uuid1,uuid2&max_results=100"
curl "http://localhost:8080/api/v1/hot/doc_source?chapter_paths=docId/chapter-title&max_results=100"
curl "http://localhost:8080/api/v1/cold/doc_source?q=scheduler,pods&max_results=100"

# 列出 L1 标签组
curl "http://localhost:8080/api/v1/tags/groups"

# 列出标签：可按 L1 组过滤。查询参数名为 group_ids（逗号分隔），可选 max_results
curl "http://localhost:8080/api/v1/tags?group_ids=group1,group2&max_results=100"

# 手动触发标签聚类任务
curl -X POST http://localhost:8080/api/v1/tags/group

# 获取单个文档 / 摘要
curl "http://localhost:8080/api/v1/documents/{id}"
curl "http://localhost:8080/api/v1/documents/{id}/summaries"

# 配额状态
curl "http://localhost:8080/api/v1/quota"
```

> **校验说明**：标签列表接口使用 **`group_ids`**（复数、逗号分隔），与 `internal/api/handler_retrieval.go` 一致；**不存在** `group_id` 单数查询参数。

### MCP 工具（智能体）

MCP 工具名与入参与 **`/api/v1` 下 REST** 语义对齐（实现见 `internal/api/mcp.go`）。成功/失败时返回体为与 REST 相同的 JSON 结构（`metrics` 为 Prometheus 文本）。

| MCP 工具名 | 对应 REST |
|------------|-----------|
| `api_v1_documents_post` | `POST /documents` |
| `api_v1_documents_list` | `GET /documents` |
| `api_v1_documents_get` | `GET /documents/:id`（参数 `id`） |
| `api_v1_documents_chapters_get` | `GET /documents/:id/chapters`（`id`） |
| `api_v1_documents_summaries_get` | `GET /documents/:id/summaries`（`id`） |
| `api_v1_query_progressive_post` | `POST /query/progressive`（`question`，`max_results`） |
| `api_v1_tags_get` | `GET /tags`（可选 `group_ids`、`max_results`） |
| `api_v1_tags_groups_get` | `GET /tags/groups` |
| `api_v1_tags_group_post` | `POST /tags/group` |
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

---

## Web 界面

基于 **Vue 3 CDN** 的单页应用（与 `cmd/web` 一致）：

### 查询页（`/#/`）
- 中央搜索框，调用渐进式查询 API  
- 分栏：左侧 AI 回答，右侧参考条目  
- 展示热/冷相关结果及相关性、来源标识  

### 文档页（`/#/docs`）
- 文档列表、检索/筛选、创建文档  
- 元数据：标题、标签、格式、状态、热度与查询统计等  

### 标签页（`/#/tags`）
- 两级导航：左侧 L1 组，右侧 L2 标签及文档计数  
- 点击标签筛选文档  

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

与英文 README 一致：热/冷分层、混合检索、三级摘要、两级标签、渐进式查询、REST+MCP、Vue CDN 界面等已勾选；OpenClaw 技能包、实时协作、多模态、企业 SSO 等为规划中项。

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
