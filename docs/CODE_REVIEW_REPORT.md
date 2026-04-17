# TierSum 代码质量复核报告

**复核范围：** 从 REST API / Job 入口开始，逐层分析 API、Job、Service、Storage 四层代码。
**复核目标：** 识别冗余代码、孤儿代码、接口/函数/模块划分不清晰、边界模糊等问题。
**生成时间：** 2026-04-17

---

## 一、已修复的高危问题（本次已完成修改）

### 1.1 MCP 网关重复实现 REST 鉴权逻辑
- **状态：** ✅ 已修复
- **修改文件：**
  - `internal/api/program_auth_middleware.go`
  - `internal/api/mcp_gate.go`
  - `internal/api/mcp.go`
  - `configs/config.example.yaml`
- **问题描述：** `mcp_gate.go` 原本手写了与 `program_auth_middleware.go` 完全相同的系统初始化检查、API Key 校验、Scope 校验、审计记录逻辑（约 35 行重复代码）。任何安全规则变更都需要改两处。
- **修复方式：**
  - 提取公共函数 `checkProgramAuth(...)`，封装 `IsSystemInitialized` → `ValidateAPIKey` → `APIKeyMeetsScope` → `RecordAPIKeyUse`。
  - REST middleware 和 MCP gate 统一调用该函数。
  - MCP 通过 `mcpserver.WithSSEContextFunc` 从 HTTP Header 读取 `X-API-Key` / `Authorization: Bearer ...` 并注入 context。
- **安全模型变更：** 彻底移除了 `TIERSUM_API_KEY` 环境变量和 `mcp.api_key` 配置项的 fallback。MCP 现在**强制使用数据库级 API Key**，与 REST `/api/v1` 完全一致。

### 1.2 监控 DTO 上移到 `pkg/types`（原报告 §2.1）
- **状态：** ✅ 已修复（2026-04-17）
- **修改文件：** [`pkg/types/cold_index_monitoring.go`](../pkg/types/cold_index_monitoring.go)、[`internal/storage/interface.go`](../internal/storage/interface.go)（类型别名）、[`internal/service/interface.go`](../internal/service/interface.go)、[`internal/service/impl/observability/monitoring_service_impl.go`](../internal/service/impl/observability/monitoring_service_impl.go)、[`internal/api/handler_test.go`](../internal/api/handler_test.go)
- **说明：** `ColdIndexVectorStats` / `ColdIndexInvertedStats` 在 `pkg/types` 定义；`storage` 包保留别名以兼容 `IColdIndex`；`IObservabilityService` 与 API 测试仅依赖 `pkg/types`，不再为监控类型 import `internal/storage`。

### 1.3 监控文档计数改为 SQL 聚合（原报告 §2.3）
- **状态：** ✅ 已修复（2026-04-17）
- **修改文件：** [`pkg/types/document.go`](../pkg/types/document.go)（`DocumentStatusCounts`）、[`internal/storage/interface.go`](../internal/storage/interface.go)、[`internal/storage/db/document/document_repository_impl.go`](../internal/storage/db/document/document_repository_impl.go)、[`internal/service/interface.go`](../internal/service/interface.go)、[`internal/service/impl/document/document_service_impl.go`](../internal/service/impl/document/document_service_impl.go)、[`internal/api/handler_execute.go`](../internal/api/handler_execute.go)
- **说明：** `ExecuteGetMonitoringSnapshot` 改为 `CountDocumentsByStatus`（`SELECT status, COUNT(*) … GROUP BY status`），全库计数正确且避免通过 `ListDocuments` 间接统计。

### 1.4 Job 队列消费者继承父 context（原报告 §3.5）
- **状态：** ✅ 已修复（2026-04-17）
- **修改文件：** [`internal/job/promote_consumer.go`](../internal/job/promote_consumer.go)、[`internal/job/hot_ingest_consumer.go`](../internal/job/hot_ingest_consumer.go)
- **说明：** `context.WithTimeout(ctx, 12*time.Minute)`，关闭/取消上层 context 时可传播到单次处理。

### 1.5 BFF Cookie 与登录限流辅助函数（原报告 §3.6、§3.7）
- **状态：** ✅ 已修复（2026-04-17）
- **修改文件：** [`internal/api/bff_cookie_helpers.go`](../internal/api/bff_cookie_helpers.go)（新建）、[`internal/api/auth_bff_handlers.go`](../internal/api/auth_bff_handlers.go)
- **说明：** `setBFFSessionCookie` / `setBFFDeviceCookie` / `clearBFFSessionAndDeviceCookies` / `clearBFFDeviceCookie` 集中 TTL 与 `Secure`/`SameSite`；`checkBFFLoginRateLimits` 统一登录与设备登录的 IP 限流与退避。

### 1.6 标签去重与异步 ingest 对齐（原报告 §3.9）
- **状态：** ✅ 已修复（2026-04-17）
- **修改文件：** [`internal/service/impl/document/document_service_impl.go`](../internal/service/impl/document/document_service_impl.go)、[`internal/service/impl/document/hot_ingest_processor_impl.go`](../internal/service/impl/document/hot_ingest_processor_impl.go)
- **说明：** `mergeOrderedTagLists` 与 `dedupeTagNames` 共用「按小写 key 去重、保留首次出现拼写」策略；`mergeTags` 委托该实现。

### 1.7 Session 行创建抽取（原报告 §3.10）
- **状态：** ✅ 已修复（2026-04-17）
- **修改文件：** [`internal/service/impl/auth/auth_service_impl.go`](../internal/service/impl/auth/auth_service_impl.go)、[`internal/service/impl/auth/auth_device_passkey_impl.go`](../internal/service/impl/auth/auth_device_passkey_impl.go)
- **说明：** `persistNewBrowserSession` 统一 `BrowserSession` 构造与 `Create` 调用。

### 1.8 MCP 移除未使用 logger（原报告 §4.4）
- **状态：** ✅ 已修复（2026-04-17）
- **修改文件：** [`internal/api/mcp.go`](../internal/api/mcp.go)、[`internal/di/container.go`](../internal/di/container.go)
- **说明：** `MCPServer` 不再持有 `*zap.Logger`；`NewMCPServer` 仅接收 REST `Handler` 与 `IProgramAuth`。

---

## 二、高危问题（High Severity）

**未结项：** §2.2、§2.4。**已修复：** §2.1、§2.3 见第一节 §1.2、§1.3。

### 2.1 服务接口泄露 Storage DTO 到 API 层
- **状态：** ✅ 已修复 — 见 **§1.2**。

### 2.2 冷索引混入 UI/展示逻辑
- **文件：** `internal/storage/coldindex/cold_index_impl.go:625-679`
- **问题：** `MarkdownChapters` 和 `coldChapterDisplayTitle` 本质上是“文档详情页 UI”需求，却被放在存储层的索引实现里。
- **修复建议：** 将这两个方法迁移到 service 层（如 `IChapterService.ExtractChaptersFromMarkdown`）或独立的 markdown 工具包中。

### 2.3 监控接口文档状态计数不准确 / 经 ListDocuments 间接统计
- **状态：** ✅ 已修复 — 见 **§1.3**。（原报告写「limit=0 无上限」与当时实现不符：`ListDocuments` 在 `limit<=0` 时默认 200；真正问题是监控计数非全库且易误导。）

### 2.4 nil IQueryService 会导致运行时 panic
- **文件：** `internal/di/container.go:142-152`
- **问题：** LLM 初始化失败时，`querySvc` 为 nil 接口值，直接传给 `api.NewHandler`。调用 `POST /query/progressive` 会触发 nil interface panic。
- **修复建议：** 容器层应 fail-fast（返回错误），或注入一个返回明确错误的 no-op 实现。

---

## 三、中危问题（Medium Severity）

**未结项：** §3.1–§3.4、§3.8 等。**已修复：** §3.5–§3.7、§3.9、§3.10 见第一节 §1.4–§1.7。

### 3.1 接口臃肿与职责混杂

#### `IAuthService` 过于庞大
- **文件：** `internal/service/interface.go:64-111`
- **问题：** 约 30 个方法，覆盖 bootstrap、登录、session、用户管理、API Key、passkey。
- **修复建议：** 按接口隔离原则拆分为 `ISessionService`、`IPasskeyService`、`IUserAdminService`、`IDeviceTokenService` 等，由 `AuthBFFHandler` 按需组合。

#### `IChapterService` 混合四种职责
- **文件：** `internal/service/interface.go:126-143`
- **问题：** 热文档 CRUD、Markdown 提取、冷索引搜索、渐进式热搜索全部堆在一个接口。
- **修复建议：** 将 `SearchColdChapterHits` 和 `SearchHotChapters` 移到 Query 相关接口（如 `IQueryService` 的内部扩展或 `IChapterSearchService`）。

#### `IBrowserSessionRepository` 臃肿
- **文件：** `internal/storage/interface.go:204-220`
- **问题：** 13 个方法混合 session CRUD、设备指纹安全校验、管理员报表。
- **修复建议：** 拆分为 `IBrowserSessionRepository`（核心 CRUD）、`IBrowserSessionSecurityRepository`（指纹校验）、`IBrowserSessionAdminRepository`（报表查询）。

### 3.2 Job 层存在大量重复模板代码

#### 定时任务骨架重复
- **文件：** `internal/job/promote_job.go` / `internal/job/hotscore_job.go`
- **问题：** 两个 struct 的字段、`Name()`、`Interval()`、`Execute()` 骨架几乎完全一致，仅名称和委托方法不同。
- **修复建议：** 提取泛化的 `delegateJob` 包装器，参数为 name、interval、执行函数，消除约 50 行重复代码。

#### 队列消费者重复
- **文件：** `internal/job/promote_consumer.go` / `internal/job/hot_ingest_consumer.go`
- **问题：** goroutine + select 循环 + 12 分钟超时 + 日志模式完全相同，仅 channel 和服务方法名不同。
- **修复建议：** 提取通用 `startQueueConsumer(ctx, service, logger, workChan, processorFn)` 辅助函数。

### 3.3 队列基础设施与调度器混在同一个文件
- **文件：** `internal/job/scheduler.go:14-19`
- **问题：** `PromoteQueue` 和 `HotIngestQueue` 是全局 package-level `var` channel，与 `Scheduler` 类型定义在同一个文件中，但队列消费者并不由 `Scheduler` 注册、启动或停止。
- **修复建议：** 将 queue 定义迁移到独立的 `internal/job/queues.go` 或改为构造函数注入，避免全局可变状态。

### 3.4 Topic Regroup 行为因调用方不同而分叉
- **文件：** `internal/job/jobs.go:37-47`
- **问题：** `TopicRegroupJob.Execute` 先检查 `ShouldRefresh`，但 REST handler 直接调用 `RegroupTags`。业务规则（“该不该刷新”）错误地放在了 Job 层。
- **修复建议：** 在 `ITopicService` 中新增 `RegroupTagsIfNeeded(ctx)` 方法，把 `ShouldRefresh` 判断下沉到 service 层，保证 API 和 Job 行为一致。

### 3.5 队列消费者忽略父 context 取消
- **状态：** ✅ 已修复 — 见 **§1.4**。

### 3.6 Cookie 设置/清除逻辑散落各处
- **状态：** ✅ 已修复 — 见 **§1.5**。

### 3.7 登录限流逻辑重复
- **状态：** ✅ 已修复 — 见 **§1.5**。

### 3.8 SQL 驱动分支语句在 15+ 个文件中重复
- **文件：** `internal/storage/db/**/*_impl.go`
- **问题：** 几乎每个仓库都重复 `if r.driver == "postgres" { q = "...$1..." } else { q = "...?..." }`。
- **修复建议：** 在 `internal/storage/db/shared` 中新增参数化占位符构建器（如 `func Placeholders(driver string, n int) string`），所有仓库统一调用。

### 3.9 标签去重逻辑重复且行为不一致
- **状态：** ✅ 已修复 — 见 **§1.6**。

### 3.10 Session 创建代码重复
- **状态：** ✅ 已修复 — 见 **§1.7**。

---

## 四、低危问题（Low Severity）

**未结项：** §4.1–§4.3、§4.5–§4.7。**已修复：** §4.4 见第一节 §1.8。

### 4.1 单实现接口的抽象 overhead
- **文件：** `internal/service/interface.go` 全部接口
- **问题：** `IDocumentService`、`IQueryService`、`ITagService` 等几乎所有 facade 接口都只有一个生产实现。
- **说明：** 虽然项目强制 Interface+Impl 模式，但过多单实现接口造成了不必要的间接层。可考虑在内部边界保留接口，在纯 CRUD 门面处直接使用具体类型，或仅在真正需要 mock/替换的位置定义接口。

### 4.2 内部接口全局可见
- **文件：** `internal/service/internal_interface.go:13-24`
- **问题：** `IDocumentAnalysisPersister` / `IDocumentAnalysisGenerator` 只在 `internal/service/impl/document/` 包内使用，却 export 到了顶层 `service` 包。
- **修复建议：** 下放到 `internal/service/impl/document/` 包内作为包局部接口。

### 4.3 `ICache` 微接口类型不安全且无处消费
- **文件：** `internal/storage/interface.go:100-104`
- **问题：** `interface{}` 类型的 Get/Set，且没有被任何上层服务依赖，仅在 storage 内部传递。
- **修复建议：** 用泛型替换，或从公共 storage 契约中移除，改为仓库内部实现细节。

### 4.4 无用 logger 字段
- **状态：** ✅ 已修复 — 见 **§1.8**。

### 4.5 `RowScanner` 孤儿包装
- **文件：** `internal/storage/db/shared/scanner.go:1-6`
- **问题：** 对 `database/sql` 标准 `Scan` 方法的无附加值包装。
- **修复建议：** 直接删除，使用 `sql.Scanner` 隐式接口即可。

### 4.6 调度器首次 tick 触发全部任务
- **文件：** `internal/job/scheduler.go:89-94`
- **问题：** `lastRuns` 初始为空，导致包括 1 小时一次的 `HotScoreJob` 在内的所有任务在启动后约 5 分钟就执行一次。
- **修复建议：** 在 `Scheduler.Start()` 中为每个 job 初始化 `lastRuns[job.Name()] = time.Now().UTC()`，或增加显式的 `RunOnStart bool` 字段由 job 自行声明。

### 4.7 硬编码公开路径列表
- **文件：** `internal/api/bff_session_middleware.go:34-41`
- **问题：** `isBFFPublicPath` 硬编码了 5 条路径，与 `auth_bff_handlers.go` 的 `RegisterPublicRoutes` 必须保持同步。
- **修复建议：** 让 `AuthBFFHandler` 暴露 `PublicPaths() []string`， middleware 直接消费，避免两处硬编码。

---

## 五、修复优先级建议

### P0（立即修复）
1. 运行时安全：`nil IQueryService` 的 panic 风险（`internal/di/container.go`）— **复核说明：** `ExecuteProgressiveQuery` 已对 nil 返回 503；若需防御 typed nil 或启动期语义再收紧，可单独排期。
2. ~~Job 队列消费者使用 `context.Background()` 而非父 context~~ — **已修复**（§1.4）。
3. SQLite tag 搜索使用 `LIKE '%tag%'` 导致的错误匹配（`document_repository_impl.go`）。

### P1（本周修复）
4. 提取 `checkProgramAuth` 的公共函数化（§1.1 已完成）。
5. ~~把 monitoring DTO 从 `internal/storage` 移到 `pkg/types`~~ — **已修复**（§1.2）。
6. 将 `MarkdownChapters` 从 cold index 迁出到 service 层。
7. 提取 `delegateJob` 和通用队列消费者，消除 Job 层重复代码。

### P2（后续迭代）
8. 拆分臃肿的 `IAuthService`、`IChapterService`、`IBrowserSessionRepository`。
9. ~~统一 Cookie 辅助函数、登录限流辅助函数、tag 去重辅助函数~~ — **已修复**（§1.5、§1.6）；其余重复模板见 §3.2、§3.8。
10. 集中处理 SQL 占位符分支逻辑。

---

## 六、代码边界清晰化建议

- **API 层** 应只依赖 `internal/service` 接口和 `pkg/types`；监控 DTO 已迁至 `pkg/types` 后，`handler_test.go` 不再为该项 import `internal/storage`。
- **Job 层** 遵守较好，仅依赖 `internal/service` 接口。问题在于全局 `var` channel 和 Job 层持有业务规则（`ShouldRefresh`）。
- **Service 层** 部分实现包之间共享逻辑不足（如 tag 去重、字符串截断），应在 `pkg/` 或 `internal/util/` 中补充。
- **Storage 层** 应避免混入 presentation 逻辑（如 `coldChapterDisplayTitle`）和 UI 细节（如 `MarkdownChapters`）。

---

## 附录：关键文件索引

| 文件 | 作用 | 问题类型 |
|---|---|---|
| `internal/api/program_auth_middleware.go` | REST API Key 鉴权中间件 | 重复代码（已修复） |
| `internal/api/mcp_gate.go` | MCP 工具鉴权入口 | 重复代码、分裂安全模型（已修复） |
| `internal/api/mcp.go` | MCP 服务器和工具注册 | Header 注入；unused logger（已修复 §1.8） |
| `internal/api/bff_cookie_helpers.go` | BFF Cookie 辅助 | 新建（§1.5） |
| `internal/api/auth_bff_handlers.go` | BFF REST handler | Cookie/限流重复（已修复 §1.5） |
| `internal/api/handler_execute.go` | 核心 execute 方法 | 监控文档计数（已修复 §1.3） |
| `internal/service/interface.go` | Service 层 facade 接口 | 接口臃肿；storage DTO 泄露（已修复 §1.2） |
| `pkg/types/cold_index_monitoring.go` | 冷索引监控 DTO | 新建（§1.2） |
| `internal/service/internal_interface.go` | 内部组合契约 | 全局可见的内部接口 |
| `internal/di/container.go` | 依赖注入根 | nil 接口 panic 风险 |
| `internal/job/scheduler.go` | 任务调度器 | 全局 var channel、首次 tick 全触发 |
| `internal/job/jobs.go` | TopicRegroupJob | 业务规则应在 service 层 |
| `internal/job/promote_job.go` / `hotscore_job.go` | 定时任务 | 重复骨架 |
| `internal/job/promote_consumer.go` / `hot_ingest_consumer.go` | 队列消费者 | 重复模板；父 context（已修复 §1.4） |
| `internal/storage/interface.go` | Storage 契约 | `ICache` 类型不安全；冷索引监控 DTO 已别名至 `pkg/types`（§1.2） |
| `internal/storage/coldindex/cold_index_impl.go` | 冷索引实现 | 混入 UI 逻辑 |
| `internal/storage/db/document/document_repository_impl.go` | 文档 SQL 仓库 | `CountDocumentsByStatus`（§1.3） |
| `internal/storage/db/**/*_impl.go` | SQL 仓库实现 | SQL 占位符分支重复 |
