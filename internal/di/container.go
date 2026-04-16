// Package di provides dependency injection and application initialization.
// This is the composition root where all concrete implementations are wired together.
package di

import (
	"database/sql"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/api"
	"github.com/tiersum/tiersum/internal/client/llm"
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/service/impl/adminconfig"
	authimpl "github.com/tiersum/tiersum/internal/service/impl/auth"
	"github.com/tiersum/tiersum/internal/service/impl/catalog"
	"github.com/tiersum/tiersum/internal/service/impl/document"
	"github.com/tiersum/tiersum/internal/service/impl/observability"
	queryimpl "github.com/tiersum/tiersum/internal/service/impl/query"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/cache"
	"github.com/tiersum/tiersum/internal/storage/coldindex"
	"github.com/tiersum/tiersum/internal/storage/db"
)

// Dependencies holds all application dependencies.
type Dependencies struct {
	// OtelSpans persists OpenTelemetry spans (progressive debug + optional HTTP tracing).
	OtelSpans storage.IOtelSpanRepository

	// Service Layer interfaces
	DocumentService    service.IDocumentService
	HotIngestProcessor service.IHotIngestProcessor
	QueryService       service.IQueryService

	// Topic regrouping + listing
	TopicService service.ITopicService

	// API Layer
	RESTHandler *api.Handler   // REST API
	MCPServer   *api.MCPServer // MCP protocol

	// Auth (dual-track: program + browser)
	AuthService service.IAuthService

	// AdminConfigView serves redacted viper snapshots for browser admins.
	AdminConfigView service.IAdminConfigViewService

	// TraceService exposes persisted OpenTelemetry traces to the BFF/API.
	TraceService service.ITraceService

	// Job Layer
	JobScheduler        *job.Scheduler
	DocumentMaintenance service.IDocumentMaintenanceService

	// ColdIndex is the concrete cold-document index (coldindex.Index).
	ColdIndex *coldindex.Index

	// Logger
	Logger *zap.Logger
}

// NewDependencies creates application dependencies (composition root for service, API, and job wiring).
func NewDependencies(sqlDB *sql.DB, driver string, coldIndex *coldindex.Index, logger *zap.Logger, serverVersion string) (*Dependencies, error) {
	// Storage: cache + repositories (UnitOfWork)
	cacheTTL := viper.GetDuration("storage.cache.ttl")
	if cacheTTL <= 0 {
		cacheTTL = 10 * time.Minute
	}
	cacheMax := viper.GetInt("storage.cache.max_size")
	cacheStore := cache.NewCache(cacheTTL, cacheMax)
	uow := db.NewUnitOfWork(sqlDB, driver, cacheStore)

	// Service: auth (program + browser/admin)
	programAuth := authimpl.NewProgramAuth(uow.SystemAuth, uow.APIKeys, uow.APIKeyAudit)
	authService := authimpl.NewAuthService(
		programAuth,
		uow.SystemAuth,
		uow.AuthUsers,
		uow.BrowserSessions,
		uow.DeviceTokens,
		uow.Passkeys,
		uow.PasskeyVerifs,
		uow.APIKeys,
		uow.APIKeyAudit,
		logger,
	)

	// Service: traces (observability)
	traceSvc := observability.NewTraceService(uow.OtelSpans)
	obsSvc := observability.NewObservabilityService(coldIndex)

	// Service: admin config view (redacted viper snapshot)
	adminCfgView := adminconfig.NewAdminConfigViewService()

	// Service: topics + tags (Tag Browser UI)
	topicSvc := catalog.NewTopicService(uow.Tags, uow.Topics)
	tagSvc := catalog.NewTagService(uow.Tags)

	quotaMgr := document.NewHotIngestQuota()

	llmProv, llmErr := llm.NewProviderFactory(logger).CreateProvider()
	if llmErr != nil {
		resolved := strings.ToLower(strings.TrimSpace(viper.GetString("llm.provider")))
		if resolved == "" {
			resolved = "openai"
		}
		// Help operators correlate "I set a key" with the actual viper keys CreateProvider reads (no secret values).
		logger.Warn("LLM provider creation failed; IQueryService and IDocumentMaintenanceService stay nil until fixed",
			zap.Error(llmErr),
			zap.String("llm.provider", resolved),
			zap.Bool("llm.openai.api_key_non_empty", strings.TrimSpace(viper.GetString("llm.openai.api_key")) != ""),
			zap.Bool("llm.anthropic.api_key_non_empty", strings.TrimSpace(viper.GetString("llm.anthropic.api_key")) != ""),
			zap.Bool("llm.local.base_url_non_empty", strings.TrimSpace(viper.GetString("llm.local.base_url")) != ""),
		)
	}

	// Document analysis generator accepts nil ILLMProvider: GenerateAnalysis uses markdown-only structuring
	// so deferred hot ingest can still persist summary + chapter rows without an API key.
	analyzer := document.NewDocumentAnalysisGenerator(llmProv, logger)
	persister := document.NewDocumentAnalysisPersister(uow.Chapters, uow.Documents, logger)
	hotIngestProc := document.NewHotIngestProcessor(uow.Documents, analyzer, persister, uow.Tags, logger)
	hotIngestSink := NewHotIngestQueueSink(logger)

	// Cold→hot promotion and other maintenance steps still require a working LLM provider.
	var maintenance service.IDocumentMaintenanceService
	if llmProv != nil {
		maintenance = document.NewDocumentMaintenanceService(uow.Documents, persister, analyzer, logger)
	} else {
		logger.Warn("LLM provider unavailable; document maintenance (cold→hot promotion, hot-score refresh) disabled")
	}

	// Service: documents (list/detail/create ingest)
	docSvc := document.NewDocumentService(uow.Documents, coldIndex, uow.Tags, uow.Chapters, quotaMgr, hotIngestSink, logger)

	// Service: chapters (detail UI + cold probe + hot chapter search)
	chapterSvc := catalog.NewChapterService(uow.Chapters, uow.Documents, uow.Tags, uow.Topics, coldIndex, llmProv, logger)

	var querySvc service.IQueryService
	if llmProv == nil {
		logger.Warn("LLM provider unavailable; progressive query disabled")
	} else {
		querySvc = queryimpl.NewQueryService(
			uow.Documents,
			chapterSvc,
			llmProv,
			logger,
		)
	}

	// Job scheduler + jobs (job layer depends on service facades only).
	sched := job.NewScheduler(logger)
	if maintenance != nil {
		sched.Register(job.NewPromoteJob(maintenance))
		sched.Register(job.NewHotScoreJob(maintenance))
	}
	sched.Register(job.NewTopicRegroupJob(topicSvc, logger))

	// API: REST + MCP servers
	restHandler := api.NewHandler(
		docSvc,        // docService
		querySvc,      // queryService
		topicSvc,      // topicService
		tagSvc,        // tagService
		chapterSvc,    // chapterService
		obsSvc,        // observabilityService (monitoring stats)
		traceSvc,      // traceService
		quotaMgr,      // quota (hot-ingest)
		logger,        // logger
		serverVersion, // serverVersion
	)
	mcpServer := api.NewMCPServer(restHandler, authService, logger)

	return &Dependencies{
		// Storage
		OtelSpans: uow.OtelSpans,

		// Auth
		AuthService:        authService,
		TraceService:       traceSvc,
		AdminConfigView:    adminCfgView,
		TopicService:       topicSvc,
		DocumentService:    docSvc,
		HotIngestProcessor: hotIngestProc,
		QueryService:       querySvc,

		JobScheduler:        sched,
		DocumentMaintenance: maintenance,

		// Cold index is owned by cmd/main.go and passed in here.
		ColdIndex: coldIndex,

		RESTHandler: restHandler,
		MCPServer:   mcpServer,

		Logger: logger,
	}, nil
}
