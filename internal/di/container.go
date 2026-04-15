// Package di provides dependency injection and application initialization
// This is the composition root where all concrete implementations are wired together
package di

import (
	"database/sql"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/api"
	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/client/llm"
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/service/svcimpl/admin"
	"github.com/tiersum/tiersum/internal/service/svcimpl/auth"
	"github.com/tiersum/tiersum/internal/service/svcimpl/catalog"
	"github.com/tiersum/tiersum/internal/service/svcimpl/common"
	"github.com/tiersum/tiersum/internal/service/svcimpl/document"
	"github.com/tiersum/tiersum/internal/service/svcimpl/observability"
	"github.com/tiersum/tiersum/internal/service/svcimpl/query"
	"github.com/tiersum/tiersum/internal/service/svcimpl/topic"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/cache"
	"github.com/tiersum/tiersum/internal/storage/coldindex"
	"github.com/tiersum/tiersum/internal/storage/db"
)

// Dependencies holds all application dependencies
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

	// Job Layer
	JobScheduler        *job.Scheduler
	DocumentMaintenance service.IDocumentMaintenanceService

	// ColdIndex is the concrete cold-document index (coldindex.Index).
	ColdIndex *coldindex.Index

	// Logger
	Logger *zap.Logger
}

// NewDependencies creates all dependencies with proper wiring.
// serverVersion is the process release label (e.g. main.Version from ldflags); passed to REST monitoring as server.version.
func NewDependencies(sqlDB *sql.DB, driver string, coldIndex *coldindex.Index, logger *zap.Logger, serverVersion string) (*Dependencies, error) {
	// 1. Storage Layer - Cache
	cacheTTL := viper.GetDuration("storage.cache.ttl")
	if cacheTTL <= 0 {
		cacheTTL = 10 * time.Minute
	}
	cacheMax := viper.GetInt("storage.cache.max_size")
	cacheStore := cache.NewCache(cacheTTL, cacheMax)

	// 2. Storage Layer - DB
	uow := db.NewUnitOfWork(sqlDB, driver, cacheStore)

	// 3. Client Layer - LLM
	factory := llm.NewProviderFactory(logger)
	baseLLM, err := factory.CreateProvider()
	if err != nil {
		return nil, err
	}
	llmProvider := common.NewOTelContextLLM(baseLLM)

	// 4. Quota Manager - for hot/cold document processing control
	quotaPerHour := viper.GetInt("quota.per_hour")
	if quotaPerHour <= 0 {
		quotaPerHour = 100 // default
	}
	quotaManager := common.NewQuotaManager(quotaPerHour)

	// 5. Service Layer - Core domain logic
	analyzer := document.NewDocumentAnalyzer(llmProvider, logger)
	filter := query.NewRelevanceFilter(llmProvider, logger)
	materializer := document.NewChapterMaterializer(uow.Chapters, uow.Documents, logger)

	// 6. Service Layer - Topics (catalog tag regrouping)
	topicService := topic.NewTopicService(
		uow.Tags,
		uow.Topics,
		llmProvider,
		logger,
	)

	// 7. Service Layer - Business logic
	queryService := query.NewQueryService(
		uow.Documents,
		uow.Chapters,
		uow.Tags,
		uow.Topics,
		filter,
		coldIndex,
		llmProvider,
		logger,
	)
	docService := document.NewDocumentService(
		uow.Documents,
		materializer,
		analyzer,
		uow.Tags,
		coldIndex,
		quotaManager,
		logger,
		job.HotIngestQueue,
	)
	hotIngestProc := document.NewHotIngestProcessor(uow.Documents, analyzer, materializer, uow.Tags, logger)

	// 8. Auth + API Layer — tag/chapter/observability read facades so HTTP handlers do not import storage interfaces
	programAuth := auth.NewProgramAuth(uow.SystemAuth, uow.APIKeys, uow.APIKeyAudit)
	authService := auth.NewAuthService(
		programAuth,
		uow.SystemAuth,
		uow.AuthUsers,
		uow.BrowserSessions,
		uow.APIKeys,
		uow.APIKeyAudit,
		logger,
	)
	adminConfigView := admin.NewAdminConfigViewService()
	tagService := catalog.NewTagService(uow.Tags)
	chapterService := catalog.NewChapterService(uow.Chapters, coldIndex)
	observabilityService := observability.NewObservabilityService(coldIndex)
	restHandler := api.NewHandler(
		docService,
		queryService,
		topicService,
		tagService,
		chapterService,
		observabilityService,
		quotaManager,
		uow.OtelSpans,
		logger,
		serverVersion,
	)
	mcpServer := api.NewMCPServer(restHandler, authService, logger)

	// 9. Job Layer — jobs depend only on service.* contracts
	docMaintenance := document.NewDocumentMaintenanceService(uow.Documents, materializer, analyzer, logger)
	jobScheduler := job.NewScheduler(logger)
	jobScheduler.Register(job.NewTopicRegroupJob(topicService, logger))
	jobScheduler.Register(job.NewPromoteJob(docMaintenance))
	jobScheduler.Register(job.NewHotScoreJob(docMaintenance))

	return &Dependencies{
		OtelSpans:           uow.OtelSpans,
		DocumentService:     docService,
		HotIngestProcessor:  hotIngestProc,
		QueryService:        queryService,
		TopicService:        topicService,
		RESTHandler:         restHandler,
		MCPServer:           mcpServer,
		AuthService:         authService,
		AdminConfigView:     adminConfigView,
		JobScheduler:        jobScheduler,
		DocumentMaintenance: docMaintenance,
		ColdIndex:           coldIndex,
		Logger:              logger,
	}, nil
}

// Interface compliance checks
var (
	// Storage Layer
	_ storage.IDocumentRepository = (*db.DocumentRepo)(nil)
	_ storage.IChapterRepository  = (*db.ChapterRepo)(nil)
	_ storage.ITagRepository      = (*db.TagRepo)(nil)
	_ storage.ITopicRepository    = (*db.TopicRepo)(nil)
	_ storage.ICache              = (*cache.Cache)(nil)
	_ storage.IColdIndex          = (*coldindex.Index)(nil)

	// Service Layer
	// (Implementations live in internal/service/svcimpl; compile-time checks are on constructors + package-private types.)

	// Client Layer
	_ client.ILLMProvider = (*llm.OpenAIProvider)(nil)
	_ client.ILLMProvider = (*llm.AnthropicProvider)(nil)
	_ client.ILLMProvider = (*llm.OllamaProvider)(nil)
)
