# TierSum

> **分层摘要知识库** —— 基于多层抽象的无 RAG 文档检索系统。

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-1.0-6E49CB)](https://modelcontextprotocol.io)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) | [中文](README_zh.md)

---

## 为什么选择 TierSum？

传统 RAG 系统将文档任意切分成碎片，丢失了层级上下文和语义结构。**TierSum 通过五层摘要保留知识架构**：

```
┌─────────────────────────────────────┐
│  Topic Summary (跨文档主题)          │  ← 多文档主题聚合
├─────────────────────────────────────┤
│  Document Summary (鸟瞰视角)         │  ← 30,000ft 全局视角
├─────────────────────────────────────┤
│  Chapter Summary (结构地图)          │  ← 10,000ft 结构视角  
├─────────────────────────────────────┤
│  Paragraph Summary (核心概念)        │  ← 1,000ft 细节视角
├─────────────────────────────────────┤
│  Source Text (原始内容)              │  ← 原文
└─────────────────────────────────────┘
```

**查询自上而下流动**：从高层摘要开始，仅在需要时深入原文。无需向量相似度猜测 —— **精准的层级导航**。

---

## 核心特性

| 特性 | 描述 |
|:--------|:------------|
| **五层摘要** | 主题 → 文档 → 章节 → 段落 → 原文，由 LLM 自动生成 |
| **LLM 自动标签** | 未提供标签时，自动通过 LLM 分析生成标签 |
| **主题合成** | 从多个文档生成跨文档主题摘要 |
| **RAG 替代方案** | 零碎片切分；完整上下文保留 |
| **双 API 架构** | REST API + MCP 工具，无缝集成智能体 |
| **原生 Markdown** | 针对 `.md` 优化；可扩展技能支持 PDF/HTML/文档转换 |
| **增量更新** | 智能差异比对 —— 仅重新摘要变更部分 |

---

## 快速开始

### 前置条件

- Go 1.23+（需要 CGO 支持 SQLite）
- 数据库：SQLite（默认）或 PostgreSQL（可选）
- LLM API 密钥：OpenAI 或 Anthropic

### 安装

```bash
# 克隆仓库
git clone https://github.com/tiersum/tiersum.git
cd tiersum

# 安装 Go 依赖
make deps

# 复制并编辑配置
cp configs/config.example.yaml configs/config.yaml

# 设置必需的环境变量
export OPENAI_API_KEY="your-api-key"
# 或者
export ANTHROPIC_API_KEY="your-api-key"

# 运行数据库迁移
make migrate-up

# 构建二进制文件
make build

# 或使用 Docker Compose（包含所有服务）
cd deployments/docker && docker-compose up -d
```

### 配置

**SQLite（默认 - 零配置）：**
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
```

**PostgreSQL（可选 - 高并发场景）：**
```yaml
storage:
  database:
    driver: postgres
    dsn: postgres://user:password@localhost:5432/tiersum?sslmode=disable
```

### 启动服务

```bash
# 本地运行
make run

# 或直接运行二进制文件
./build/tiersum --config configs/config.yaml

# 服务就绪
# REST API: http://localhost:8080/api/v1
# MCP SSE: http://localhost:8080/mcp/sse
```

---

## API 使用

### REST API

```bash
# 录入文档（未提供标签时自动通过 LLM 生成）
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Kubernetes 架构",
    "content": "# Kubernetes 架构\n\n## 控制平面...",
    "format": "markdown"
  }'

# 从多个文档创建主题
curl -X POST http://localhost:8080/api/v1/topics \
  -H "Content-Type: application/json" \
  -d '{
    "name": "云原生概念",
    "document_ids": ["doc-1", "doc-2", "doc-3"]
  }'

# 查询分层摘要
curl "http://localhost:8080/api/v1/query?question=kube-scheduler 是如何工作的？&depth=chapter"
# depth: topic | document | chapter | paragraph | source

# 列出主题
curl "http://localhost:8080/api/v1/topics"

# 深入钻取
curl "http://localhost:8080/api/v1/documents/{id}/hierarchy?path=1.2.3"
```

### MCP 工具（面向智能体）

```json
{
  "tools": [
    {
      "name": "tiersum_query",
      "description": "分层精准查询知识库",
      "inputSchema": {
        "question": "string",
        "depth": "topic|document|chapter|paragraph|source",
        "filters": {"tags": ["kubernetes"]}
      }
    },
    {
      "name": "tiersum_get_document",
      "description": "通过 ID 获取文档",
      "inputSchema": {
        "document_id": "string"
      }
    },
    {
      "name": "tiersum_list_topics",
      "description": "列出所有主题摘要",
      "inputSchema": {}
    },
    {
      "name": "tiersum_get_topic",
      "description": "通过 ID 获取主题摘要",
      "inputSchema": {
        "topic_id": "string"
      }
    }
  ]
}
```

**OpenClaw 集成**：
```yaml
# openclaw-skill/skill.yaml
mcpServers:
  tiersum:
    type: sse
    url: http://localhost:8080/mcp/sse
    tools:
      - tiersum_query
      - tiersum_get_document
      - tiersum_list_topics
      - tiersum_get_topic
```

---

## 架构

### 五层设计 + 接口+实现子包模式

```
┌─────────────────────────────────────────────────────────────┐
│                        客户端层                               │
│  (OpenClaw / Claude Desktop / 自定义智能体 / REST 客户端)    │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                      API 层 (internal/api)                   │
│  ┌──────────────┐  ┌──────────────┐                         │
│  │   REST API   │  │  MCP Server  │                         │
│  └──────────────┘  └──────────────┘                         │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   服务层 (internal/service)                  │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  interface.go: I* 接口 (IDocumentService, 等)           ││
│  └─────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────┐│
│  │  impl/: 实现 (DocumentSvc, QuerySvc, 等)                ││
│  │  包括：Indexer, Summarizer, Parser                      ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   存储层 (internal/storage)                  │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  interface.go: I* 接口 (IDocumentRepository, 等)        ││
│  └─────────────────────────────────────────────────────────┘│
│  ┌──────────────────┐    ┌──────────────────┐              ││
│  │  db/repository.go│    │  cache/cache.go  │              ││
│  │  (SQLite/PG)     │    │  (内存缓存)       │              ││
│  └──────────────────┘    └──────────────────┘              ││
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                    客户端层 (internal/client)                │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  interface.go: ILLMProvider                             ││
│  └─────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────┐│
│  │  llm/openai.go: OpenAIProvider 实现                     ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### 设计原则

1. **接口+实现子包模式**：每层定义 `interface.go` 包含 I 前缀接口，实现在子包中
2. **层拥有接口**：没有集中的 `ports/` 包 —— 每层管理自己的接口
3. **依赖注入**：所有装配在 `internal/di/container.go`
4. **统一 API**：REST 和 MCP 处理器共存于 `internal/api/`

---

## 文档处理流程

```
输入 (Markdown/PDF/HTML)
    │
    ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   解析器     │───▶│  结构化器    │───▶│  摘要生成器  │
│ (Goldmark)  │    │ (标题层级)   │    │  (LLM API)  │
│             │    │             │    │             │
└─────────────┘    └─────────────┘    └──────┬──────┘
                                             │
                   ┌─────────────────────────┼─────────────────────────────────┐
                   ▼                         ▼                                 ▼
           ┌─────────────┐          ┌─────────────┐          ┌─────────────┐ ┌─────────────┐
           │ 主题摘要     │          │ 文档摘要     │          │ 章节摘要     │ │ 段落摘要     │
           │(跨文档)      │          │(摘要)       │          │(大纲)       │ │(关键点)      │
           └─────────────┘          └─────────────┘          └─────────────┘ └─────────────┘
```

---

## 项目结构

```
tiersum/
├── cmd/
│   ├── server/          # API 服务器入口
│   ├── worker/          # 后台任务处理器
│   └── cli/             # CLI 工具
├── configs/             # 配置文件
│   ├── config.example.yaml
│   └── config.yaml
├── deployments/
│   └── docker/          # Docker 和 docker-compose 文件
├── internal/
│   ├── api/             # 第 1 层：API（REST + MCP 处理器）
│   ├── service/         # 第 2 层：业务逻辑
│   │   ├── interface.go # I* 接口 (IDocumentService, 等)
│   │   └── impl/        # 实现
│   │       ├── document.go
│   │       ├── query.go
│   │       ├── topic.go
│   │       └── indexer.go  # Indexer, Summarizer, Parser
│   ├── storage/         # 第 3 层：数据持久化
│   │   ├── interface.go # I* 接口
│   │   ├── db/
│   │   │   └── repository.go
│   │   └── cache/
│   │       └── cache.go
│   ├── client/          # 第 4 层：外部依赖
│   │   ├── interface.go # ILLMProvider
│   │   └── llm/
│   │       └── openai.go
│   ├── job/             # 后台任务
│   │   ├── scheduler.go
│   │   └── jobs.go
│   └── di/              # 依赖注入
│       └── container.go
├── pkg/
│   └── types/           # 公共 API 类型
├── migrations/          # 数据库迁移
├── go.mod
├── Makefile
├── README.md
└── LICENSE
```

---

## 开发

```bash
# 运行测试
make test

# 运行代码检查
make lint

# 格式化代码
make fmt

# 热重载运行（需要 air）
make dev

# 多平台构建
make build-all
```

---

## 路线图

- [x] 五层摘要引擎（主题 + 文档 + 章节 + 段落 + 原文）
- [x] LLM 自动标签
- [x] 多文档主题合成
- [x] REST API + MCP 服务
- [x] SQLite/PostgreSQL + 内存缓存 存储
- [ ] OpenClaw 技能包（转换 + 更新）
- [ ] 实时协作编辑
- [ ] 多模态支持（图片、图表）
- [ ] 企业 SSO + 审计日志

---

## 贡献

欢迎贡献！查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解指南。

**适合新手的任务**：
- 额外的文档格式解析器（LaTeX、AsciiDoc）
- 本地 LLM 适配器（Ollama、vLLM）
- 文档探索 Web UI

---

## 许可证

[MIT 许可证](LICENSE) © 2026 TierSum 贡献者

---

## 致谢

- 灵感来自 [Anthropic's Contextual Retrieval](https://www.anthropic.com/news/contextual-retrieval)
- MCP 协议由 [Anthropic](https://modelcontextprotocol.io) 开发
- 基于 [Gin](https://gin-gonic.com)、[Goldmark](https://github.com/yuin/goldmark) 构建
