# TierSum

> **分层摘要知识库** —— 基于多层抽象和冷热文档分层的无 RAG 文档检索系统。

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-1.0-6E49CB)](https://modelcontextprotocol.io)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) | [中文](README_zh.md)

---

## 为什么选择 TierSum？

传统 RAG 系统将文档任意切分成碎片，丢失了层级上下文和语义结构。**TierSum 通过分层摘要和智能标签导航保留知识架构**：

```
┌─────────────────────────────────────────────────────────────┐
│  标签组 (L1)                                                 │
│  ├── 云原生                                                  │
│  │      └── 标签: kubernetes, docker, helm                   │
│  └── 编程语言                                                │
│         └── 标签: golang, python, rust                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────┐
│  文档摘要                            │  ← 30,000ft 全局视角
├─────────────────────────────────────┤
│  章节摘要                            │  ← 10,000ft 结构视角  
├─────────────────────────────────────┤
│  原文内容                            │  ← 原始内容
└─────────────────────────────────────┘
```

**查询通过智能过滤流动**：从 LLM 筛选的标签开始，然后是文档，再到章节 —— 每一步都由 LLM 相关性评分优化。无需向量相似度猜测 —— **精准的层级导航**。

---

## 核心特性

| 特性 | 描述 |
|:--------|:------------|
| **冷热分层** | 智能文档存储：热文档（完整 LLM 分析）vs 冷文档（BM25 + 向量检索） |
| **三层摘要** | 文档 → 章节 → 原文，由 LLM 自动生成 |
| **两级标签层级** | L1 标签组（聚类）→ L2 标签（自动生成） |
| **渐进式查询** | LLM 在每个步骤筛选标签 → 文档 → 章节 |
| **自动标签聚类** | LLM 自动将相关标签分组为类别 |
| **BM25 + 向量混合检索** | 关键词 + 语义检索，带关键词片段提取 |
| **RAG 替代方案** | 零碎片切分；完整上下文保留 |
| **双 API 架构** | REST API + MCP 工具，无缝集成智能体 |
| **现代 Web UI** | Next.js 14 前端，Slate 暗色主题 |
| **原生 Markdown** | 针对 `.md` 优化；可扩展技能支持 PDF/HTML/文档转换 |
| **增量更新** | 智能差异比对 —— 仅重新摘要变更部分（规划中） |

---

## 冷热文档分层

TierSum 使用两层系统平衡 LLM 成本和检索性能：

### 热文档（完整分析）
- ✅ 完整 LLM 分析，包含文档 + 章节摘要
- ✅ 最多 10 个自动生成的标签
- ✅ 查询时基于 LLM 的过滤
- ✅ 存储在数据库中，带分层摘要
- ⚡ 需要配额（默认 100/小时）

**标准**：有可用配额 AND（强制热文档 OR 有预构建摘要 OR 内容 > 5000 字符）

### 冷文档（高效存储）
- ✅ 最小化处理，无 LLM 分析
- ✅ BM25 + 向量混合检索（Bleve + HNSW）
- ✅ 基于关键词的片段提取
- ✅ 查询 3+ 次后自动升级
- ⚡ 不消耗配额

**存储**：内存索引，384 维向量

```
┌─────────────────────────────────────────────────────────────┐
│                    热文档                                    │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  完整 LLM 分析 → 标签 + 摘要 + 章节                   │  │
│  │  渐进式查询（L1→L2→文档→章节）                        │  │
│  └───────────────────────────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                    冷文档                                    │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  BM25 + 向量混合检索                                  │  │
│  │  基于关键词的片段提取                                 │  │
│  │  查询 3 次后自动升级 → 热文档                         │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 快速开始

### 前置条件

- Go 1.23+（需要 CGO 支持 SQLite）
- Node.js 18+（前端需要）
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

# 构建后端
make build

# 构建前端
cd web && npm install && npm run build && cd ..

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

quota:
  per_hour: 100  # 每小时热文档数

documents:
  tiering:
    hot_content_threshold: 5000  # 热层最小字符数
    cold_promotion_threshold: 3  # 自动升级查询次数
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
# 本地运行（后端 + 前端）
make run

# 或直接运行二进制文件
./build/tiersum --config configs/config.yaml

# 服务就绪
# Web UI:   http://localhost:8080/
# REST API: http://localhost:8080/api/v1
# MCP SSE:  http://localhost:8080/mcp/sse
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
    "format": "markdown",
    "force_hot": true  # 强制完整 LLM 分析
  }'

# 渐进式查询（推荐）- 同时搜索热文档和冷文档
curl -X POST http://localhost:8080/api/v1/query/progressive \
  -H "Content-Type: application/json" \
  -d '{
    "question": "kube-scheduler 是如何工作的？",
    "max_results": 100
  }'

# 传统分层查询
curl "http://localhost:8080/api/v1/query?question=kube-scheduler 是如何工作的？&depth=chapter"
# depth: document | chapter | source

# 列出标签组（L1）
curl "http://localhost:8080/api/v1/tags/groups"

# 获取组内标签
curl "http://localhost:8080/api/v1/tags?group_id=xxx"

# 手动触发标签分组
curl -X POST http://localhost:8080/api/v1/tags/group

# 获取文档
curl "http://localhost:8080/api/v1/documents/{id}"

# 获取文档摘要
curl "http://localhost:8080/api/v1/documents/{id}/summaries"

# 检查配额
curl "http://localhost:8080/api/v1/quota"
```

### MCP 工具（面向智能体）

```json
{
  "tools": [
    {
      "name": "tiersum_query",
      "description": "查询知识库获取相关内容（传统）",
      "inputSchema": {
        "question": "string",
        "depth": "document|chapter|source"
      }
    },
    {
      "name": "tiersum_progressive_query",
      "description": "使用两级标签层级执行渐进式查询（推荐）",
      "inputSchema": {
        "question": "string",
        "max_results": "number (default: 100)"
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
      "name": "tiersum_list_tag_groups",
      "description": "列出所有标签组（L1 类别）",
      "inputSchema": {}
    },
    {
      "name": "tiersum_get_tags_by_group",
      "description": "获取属于特定组的所有标签（L2）",
      "inputSchema": {
        "group_id": "string"
      }
    },
    {
      "name": "tiersum_ingest_document",
      "description": "录入文档，可携带预构建摘要",
      "inputSchema": {
        "title": "string",
        "content": "string",
        "format": "markdown|md",
        "tags": ["string"],
        "force_hot": "boolean",
        "summary": "string (optional)",
        "chapters": [{"title": "string", "summary": "string", "content": "string"}]
      }
    },
    {
      "name": "tiersum_trigger_tag_grouping",
      "description": "手动触发标签分组（每 30 分钟自动运行）",
      "inputSchema": {}
    }
  ]
}
```

**Claude Desktop 集成**：
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

TierSum 使用**五层架构**配合接口+实现子包模式：

```
┌─────────────────────────────────────────────────────────────┐
│  客户端层                                                    │
│  (REST API / MCP / Web UI)                                  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  API 层           (REST 处理器 + MCP 服务)                  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  服务层           (业务逻辑 + LLM 集成)                     │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  存储层           (数据库仓库 + 缓存 + 内存索引)            │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  客户端层         (LLM 提供商)                              │
└─────────────────────────────────────────────────────────────┘
```

📚 **详细架构文档请参见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。**

---

## Web UI

TierSum 包含现代化的 Next.js 14 前端，具有以下功能：

### 查询页 (`/`)
- 中央搜索框，支持渐进式查询
- 分栏结果：左侧文档列表，右侧章节详情
- 同时显示热文档和冷文档结果
- 显示相关性评分和来源标识

### 文档页 (`/docs/[id]`)
- 文档元数据（标题、标签、格式、状态）
- 热度评分和查询次数统计
- 章节导航，支持层级选择
- 原文内容查看器，带语法高亮

### 标签浏览器 (`/tags`)
- 两级标签导航
- 左侧面板：L1 标签组（分类）
- 右侧面板：L2 标签及文档数量
- 点击标签过滤文档

### 技术栈
- **框架**：Next.js 14（App Router）
- **语言**：TypeScript
- **样式**：Tailwind CSS 4
- **组件**：shadcn/ui
- **主题**：Slate 暗色主题
- **构建**：静态导出到 `web/dist/`

---

## 片段提取算法

对于冷文档，TierSum 提取基于关键词的片段，在无需加载完整文档的情况下提供相关上下文：

```
查询: "kube-scheduler 是如何工作的？"
    │
    ▼
提取关键词: ["kube-scheduler", "work", "schedule", ...]
    │
    ▼
在文档中定位: 查找所有关键词位置
    │
    ▼
上下文窗口: 提取每个匹配前后 200 字符
    │
    ▼
合并重叠: 合并 50 字符内的片段
    │
    ▼
返回前 3 个: 最相关的合并片段
```

**示例输出：**
```
... kube-scheduler 是控制平面组件，负责将
Pod 分配给节点。它通过监听新创建的 Pod
并选择最佳节点来运行它们 ...

...

... 调度决策考虑资源需求、亲和性规则和污
点/容忍。调度器使用评分算法对节点进行排名 ...
```

---

## 项目结构

```
tiersum/
├── cmd/
│   └── main.go                 # API 服务器入口
├── configs/                    # 配置文件
│   ├── config.example.yaml
│   └── config.yaml
deployments/
│   └── docker/                 # Docker 和 docker-compose 文件
db/
│   └── migrations/             # 数据库迁移文件（7 个版本）
├── internal/
│   ├── api/                    # 第 1 层：API（REST + MCP 处理器）
│   ├── service/                # 第 2 层：业务逻辑
│   │   ├── interface.go        # I* 接口
│   │   └── svcimpl/            # 实现
│   │       ├── document.go     # 冷热分层
│   │       ├── query.go        # 渐进式查询
│   │       ├── tag_grouping.go # 自动聚类
│   │       ├── indexer.go      # 摘要索引
│   │       ├── summarizer.go   # LLM 分析
│   │       └── quota.go        # 速率限制
│   ├── storage/                # 第 3 层：数据持久化
│   │   ├── interface.go
│   │   ├── db/
│   │   │   ├── repository.go   # SQL 仓库
│   │   │   ├── schema.go       # 数据库架构
│   │   │   └── migrator.go     # 迁移
│   │   ├── cache/
│   │   │   └── cache.go        # 内存缓存
│   │   └── memory/
│   │       └── index.go        # BM25 + HNSW 索引
│   ├── client/                 # 第 4 层：外部依赖
│   │   ├── interface.go
│   │   └── llm/
│   │       └── openai.go       # OpenAI/Anthropic
│   ├── job/                    # 后台任务
│   │   ├── scheduler.go
│   │   ├── jobs.go             # 索引器、标签分组
│   │   ├── promote_job.go      # 冷→热升级
│   │   └── hotscore_job.go     # 热度计算
│   └── di/                     # 依赖注入
│       └── container.go
├── web/                        # Next.js 14 前端
│   ├── app/                    # App Router 页面
│   ├── components/ui/          # shadcn/ui 组件
│   ├── lib/api.ts              # API 客户端
│   └── dist/                   # 静态导出
├── pkg/
│   └── types/                  # 公共 API 类型
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

# 前端开发
cd web
npm run dev          # 开发服务器
npm run build        # 生产构建
```

---

## 路线图

- [x] 冷热文档分层与自动升级
- [x] BM25 + 向量混合检索与片段提取
- [x] 三层摘要引擎（文档 + 章节 + 原文）
- [x] 两级标签层级与自动分组
- [x] 渐进式查询，每步都有 LLM 过滤
- [x] LLM 自动标签
- [x] REST API + MCP 服务
- [x] SQLite/PostgreSQL + 内存缓存存储
- [x] Next.js 14 前端，Slate 主题
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
- Web UI 功能增强

---

## 许可证

[MIT 许可证](LICENSE) © 2026 TierSum 贡献者

---

## 致谢

- 灵感来自 [Anthropic's Contextual Retrieval](https://www.anthropic.com/news/contextual-retrieval)
- MCP 协议由 [Anthropic](https://modelcontextprotocol.io) 开发
- 基于 [Gin](https://gin-gonic.com)、[Goldmark](https://github.com/yuin/goldmark)、[Bleve](https://blevesearch.com)、[HNSW](https://github.com/coder/hnsw) 构建
