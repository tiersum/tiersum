# TierSum 代码质量复核 — 未结项 / 争议项 / 未开始项

**生成时间：** 2026-04-17
**基于：** `docs/CODE_REVIEW_REPORT.md` 复核结果
**复核方法：** 逐项 grep / read 源码验证；区分"报告声称已修复"与"代码实际状态"。

---

## 一、未结项（Confirmed Open Issues）

> 经源码验证，以下问题**确实仍存在**，尚未修复。

### 1.1 高危 — 冷索引混入 UI/展示逻辑
- **文件：** `internal/storage/coldindex/cold_index_impl.go:625-679`
- **问题：** `MarkdownChapters` 和 `coldChapterDisplayTitle` 本质上是文档详情页的 UI 需求，却放在存储层索引实现里。
- **当前状态：** 代码仍在原处，`coldChapterDisplayTitle` 还在做 `" · "` 分隔符的展示格式化。
- **修复建议：** 迁移到 `IChapterService.ExtractChaptersFromMarkdown` 或独立 markdown 工具包。

### 1.2 中危 — 接口臃肿与职责混杂

#### `IAuthService` 过于庞大
- **文件：** `internal/service/interface.go:64-111`
- **问题：** 约 30 个方法，覆盖 bootstrap、登录、session、用户管理、API Key、passkey。
- **当前状态：** 接口未拆分。
- **修复建议：** 按接口隔离原则拆分为 `ISessionService`、`IPasskeyService`、`IUserAdminService`、`IDeviceTokenService` 等。

#### `IChapterService` 混合四种职责
- **文件：** `internal/service/interface.go:126-143`
- **问题：** 热文档 CRUD、Markdown 提取、冷索引搜索、渐进式热搜索全部堆在一个接口。
- **当前状态：** 接口未拆分。
- **修复建议：** 将搜索方法移到 Query 相关接口。

#### `IBrowserSessionRepository` 臃肿
- **文件：** `internal/storage/interface.go:204-220`
- **问题：** 13 个方法混合 session CRUD、设备指纹安全校验、管理员报表。
- **当前状态：** 接口未拆分。
- **修复建议：** 拆分为角色聚焦的小接口。

### 1.3 中危 — Job 层重复模板代码

#### 定时任务骨架重复
- **文件：** `internal/job/promote_job.go` / `internal/job/hotscore_job.go`
- **问题：** 两个 struct 的字段、`Name()`、`Interval()`、`Execute()` 骨架完全一致，仅名称和委托方法不同。
- **当前状态：** 仍是独立文件，各自约 33 行重复骨架。
- **修复建议：** 提取泛化 `delegateJob` 包装器。

#### 队列消费者重复
- **文件：** `internal/job/promote_consumer.go` / `internal/job/hot_ingest_consumer.go`
- **问题：** goroutine + select 循环 + 12 分钟超时 + 日志模式完全相同。
- **当前状态：** 仍是独立文件，重复约 22 行。
- **修复建议：** 提取通用 `startQueueConsumer` 辅助函数。

### 1.4 中危 — 队列基础设施与调度器混在同一个文件
- **文件：** `internal/job/scheduler.go:14-19`
- **问题：** `PromoteQueue` 和 `HotIngestQueue` 是全局 `var` channel，与 `Scheduler` 定义在同一个文件，但消费者不由 `Scheduler` 管理。
- **当前状态：** 仍是全局 `var`。
- **修复建议：** 迁移到 `internal/job/queues.go` 或改为构造函数注入。

### 1.5 中危 — Topic Regroup 行为因调用方不同而分叉
- **文件：** `internal/job/jobs.go:37-47`
- **问题：** `TopicRegroupJob.Execute` 先检查 `ShouldRefresh`，但 REST handler 直接调用 `RegroupTags`。业务规则错误地放在了 Job 层。
- **当前状态：** `jobs.go` 中仍有 `ShouldRefresh` 判断。
- **修复建议：** 在 `ITopicService` 中新增 `RegroupTagsIfNeeded(ctx)`，将判断下沉到 service 层。

### 1.6 中危 — SQL 驱动分支语句在 15+ 个文件中重复
- **文件：** `internal/storage/db/**/*_impl.go`
- **问题：** 几乎每个仓库都重复 `if r.driver == "postgres" { "$1" } else { "?" }`。
- **当前状态：** 仍是逐文件硬编码。
- **修复建议：** 在 `internal/storage/db/shared` 中新增 `Placeholders(driver string, n int) string` 统一处理。

### 1.7 低危 — 单实现接口的抽象 overhead
- **文件：** `internal/service/interface.go` 全部接口
- **问题：** `IDocumentService`、`IQueryService`、`ITagService` 等几乎所有 facade 接口都只有一个生产实现。
- **当前状态：** 未变更。
- **说明：** 项目强制 Interface+Impl 模式，但过多单实现接口造成不必要间接层。属于**架构风格问题**，非 bug。

### 1.8 低危 — 内部接口全局可见
- **文件：** `internal/service/internal_interface.go:13-24`
- **问题：** `IDocumentAnalysisPersister` / `IDocumentAnalysisGenerator` 只在 `internal/service/impl/document/` 包内使用，却 export 到顶层 `service` 包。
- **当前状态：** 仍在原处。
- **修复建议：** 下放到 `internal/service/impl/document/` 包内。

### 1.9 低危 — `ICache` 微接口类型不安全且无处消费
- **文件：** `internal/storage/interface.go:100-104`
- **问题：** `interface{}` 类型的 Get/Set，且没有被任何上层服务依赖。
- **当前状态：** 仍在接口中；`TopicRepo` 和 `TagRepo` 的构造函数接受该参数但**从未使用**（死代码）。
- **修复建议：** 从公共 storage 契约中移除，改为仓库内部实现细节；同时清理仓库中的死字段。

### 1.10 低危 — `RowScanner` 孤儿包装
- **文件：** `internal/storage/db/shared/scanner.go:1-6`
- **问题：** 对 `database/sql` 标准 `Scan` 方法的无附加值包装。
- **当前状态：** 仍在原处。
- **修复建议：** 直接删除，使用 `sql.Scanner` 隐式接口。

### 1.11 低危 — 调度器首次 tick 触发全部任务
- **文件：** `internal/job/scheduler.go:89-94`
- **问题：** `lastRuns` 初始为空，`!ok` 导致所有 job（包括 1 小时一次的 `HotScoreJob`）在启动后首个 tick（约 5 分钟）就执行。
- **当前状态：** 仍是此逻辑。
- **修复建议：** 初始化 `lastRuns[job.Name()] = time.Now().UTC()`，或增加 `RunOnStart bool` 字段。

### 1.12 低危 — 硬编码公开路径列表
- **文件：** `internal/api/bff_session_middleware.go:34-41`
- **问题：** `isBFFPublicPath` 硬编码 5 条路径，与 `auth_bff_handlers.go` 的 `RegisterPublicRoutes` 必须保持同步。
- **当前状态：** 仍是硬编码。
- **修复建议：** 让 `AuthBFFHandler` 暴露 `PublicPaths() []string`，middleware 直接消费。

---

## 二、争议项（Disputed / Clarification Needed）

> 以下条目在报告中的描述与代码实际状态存在偏差，或建议本身存在争议，需要重新评估优先级或结论。

### 2.1 §2.4 "nil IQueryService 会导致运行时 panic" — 应降级
- **原报告评级：** 高危
- **复核发现：** `internal/api/handler_execute.go:145` 已有防御：
  ```go
  if h.QueryService == nil {
      return http.StatusServiceUnavailable, gin.H{"error": "query service not available"}
  }
  ```
- **实际风险：** 运行时不会 panic，会返回 503。真正的隐患是容器层**语义不一致**（`maintenance` 为 nil 时 job 层安全跳过，`querySvc` 为 nil 时依赖 handler 防御），而非 crash。
- **建议处理：** 从 P0 高危降级为 **P2 低危/观察项**，或改为"容器层 nil 处理语义不一致"。

### 2.2 §2.3 "监控接口加载全表文档到内存" — 原报告描述不准确
- **原报告描述：** "`ExecuteGetMonitoringSnapshot` 调用 `h.DocService.ListDocuments(ctx, 0)`（limit=0 表示无上限）"
- **复核发现：** 当前代码（以及历史代码）中 `ListDocuments` 在 `limit <= 0` 时使用的是**服务默认值 200**，并非"无上限"。真正的问题是监控计数**非全库**且容易误导，而非 OOM 风险。
- **当前状态：** 已通过 `CountDocumentsByStatus` 修复（SQL 聚合）。
- **建议处理：** 此项已在 §1.3 标记为修复；原报告中的"limit=0 无上限"描述属于**事实错误**，应在后续文档中勘误。

### 2.3 §4.1 "单实现接口的抽象 overhead" — 架构风格争议
- **问题本质：** 这不是 bug 或技术债，而是对项目强制 Interface+Impl 模式的**设计哲学争议**。
- **争议点：**
  - 一方认为：每个层定义接口有助于边界清晰、便于测试 mock。
  - 另一方认为：单实现接口增加无意义间接层，Go 社区更倾向于"需要 mock 时再抽象"。
- **建议处理：** 标记为 **架构决策项（ADR）**，而非技术债。如需调整，应通过架构评审会议决定，不列入常规修复 backlog。

### 2.4 §1.6 "标签去重与异步 ingest 对齐" — 修复范围存疑
- **报告声称：** `mergeOrderedTagLists` 与 `dedupeTagNames` 共用策略。
- **复核状态：** 由于未直接读取 `hot_ingest_processor_impl.go` 的完整实现，无法 100% 确认两处已完全一致。
- **建议处理：** 保留为 **待验证项**，由负责该模块的开发者确认两处的 casing 策略（小写 key 去重、保留首次拼写）确实一致后，再标记为完成。

### 2.5 §1.7 "Session 行创建抽取" — 需确认 DeviceLogin 路径
- **报告声称：** `persistNewBrowserSession` 统一了 `BrowserSession` 构造。
- **复核状态：** 已确认 `LoginWithAccessToken` 调用了抽取函数；但 `DeviceLogin` 的实现分散在 `auth_service_impl.go` 和 `auth_device_passkey_impl.go` 中，需确认后者是否也使用了同一辅助函数。
- **建议处理：** 标记为 **待验证项**。

---

## 三、未开始项（Not Started）

> 以下条目在原报告中虽有建议，但**没有任何代码修改痕迹**，尚未进入实施阶段。

### 3.1 MarkdownChapters 从 cold index 迁出
- **建议来源：** 原报告 §2.2
- **当前状态：** 零进展。`cold_index_impl.go:625-679` 原封不动。
- **预计工作量：** 中（需新建 markdown 工具包或调整 service 接口，涉及调用方迁移）。
- **阻塞因素：** 需确定目标位置（service 层 vs 独立 `pkg/markdown`）。

### 3.2 接口拆分（IAuthService / IChapterService / IBrowserSessionRepository）
- **建议来源：** 原报告 §3.1
- **当前状态：** 零进展。所有接口仍在原处。
- **预计工作量：** 大（涉及面广，需同步调整 handler、DI container、测试 mock）。
- **阻塞因素：** 属于架构重构，需评估对现有测试和 mock 的影响。

### 3.3 Job 通用模板提取（delegateJob / startQueueConsumer）
- **建议来源：** 原报告 §3.2
- **当前状态：** 零进展。
- **预计工作量：** 小（纯代码移动，无行为变更）。
- **阻塞因素：** 无；可随时进行。

### 3.4 SQL 占位符集中化
- **建议来源：** 原报告 §3.8
- **当前状态：** 零进展。约 15 个文件仍各自硬编码 postgres/sqlite 分支。
- **预计工作量：** 中（需逐个文件替换，但改动机械）。
- **阻塞因素：** 无；属于 good-first-issue。

### 3.5 Topic Regroup 业务规则下沉
- **建议来源：** 原报告 §3.4
- **当前状态：** 零进展。`jobs.go` 中仍有 `ShouldRefresh` 判断。
- **预计工作量：** 小（在 service 层新增方法，job 和 handler 统一调用）。
- **阻塞因素：** 需确认 `RegroupTags` 的 REST 行为是否也应改为"按需刷新"。

### 3.6 `ICache` 从 storage 接口中移除
- **建议来源：** 原报告 §4.3
- **当前状态：** 零进展。
- **预计工作量：** 小（删除接口定义 + 清理仓库构造函数中的死参）。
- **阻塞因素：** 需确认是否有其他未读到的代码路径实际使用了 `ICache`。

### 3.7 公开路径列表去硬编码
- **建议来源：** 原报告 §4.7
- **当前状态：** 零进展。
- **预计工作量：** 极小。
- **阻塞因素：** 无。

---

## 四、建议的后续行动

### 立即可以做（无阻塞）
1. **§1.3 提取 Job 通用模板** — 工作量小，零风险，消除 70+ 行重复代码。
2. **§1.6 SQL 占位符集中化** — 机械改动，适合作为 good-first-issue。
3. **§3.7 公开路径列表去硬编码** — 5 分钟改动。
4. **§3.6 移除 `ICache` 死代码** — 清理构造函数噪音。

### 本周排期（需简单决策）
5. **§1.5 Topic Regroup 规则下沉** — 需确认 REST 是否也应改为"按需刷新"。
6. **§1.11 调度器首次 tick** — 需决策：是初始化 timestamp 还是增加 `RunOnStart` 字段。

### 需要架构评审（大改动）
7. **§1.2 接口拆分** — 影响面大，建议单独开 ADR。
8. **§3.2 MarkdownChapters 迁出** — 需确定新归属（service vs pkg）。
9. **§2.1 单实现接口 overhead** — 架构风格决策，非 bug。

### 降级观察
10. **§2.4 nil IQueryService** — 从 P0 降级为 P2，因为 handler 已有 503 防御。

---

## 附录：复核验证日志

| 条目 | 验证方法 | 结论 |
|---|---|---|
| §1.2 monitoring DTO | `ls pkg/types/cold_index_monitoring.go` | 存在，已修复 ✅ |
| §1.3 CountDocumentsByStatus | grep 跨 14 个文件 | 存在，已修复 ✅ |
| §1.4 Job context | grep `context.WithTimeout(ctx, 12*time.Minute)` | 存在，已修复 ✅ |
| §1.5 Cookie helpers | `ls internal/api/bff_cookie_helpers.go` | 存在，已修复 ✅ |
| §1.8 MCP logger | 读取 `mcp.go` | 字段已移除，已修复 ✅ |
| §2.4 nil query | 读取 `handler_execute.go:145` | 已有 nil 防御，应降级 |
| §1.11 首次 tick | 读取 `scheduler.go:89-94` | 仍是 `!ok` 触发，未修复 |
| §1.1 MCP 鉴权 | 读取 `mcp_gate.go` | 已改为 `checkProgramAuth`，已修复 ✅ |
