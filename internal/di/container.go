package di

import (
	"database/sql"
	"fmt"
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

type Dependencies struct {
	OtelSpans storage.IOtelSpanRepository

	DocumentService    service.IDocumentService
	HotIngestProcessor service.IHotIngestProcessor
	QueryService       service.IQueryService

	TopicService service.ITopicService

	RESTHandler *api.Handler
	MCPServer   *api.MCPServer

	AuthService service.IAuthService

	AdminConfigView service.IAdminConfigViewService

	TraceService service.ITraceService

	JobScheduler        *job.Scheduler
	DocumentMaintenance service.IDocumentMaintenanceService

	ColdIndex *coldindex.Index

	Logger *zap.Logger
}

func requirePrompt(key string) (string, error) {
	v := viper.GetString("llm.prompts." + key)
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("config llm.prompts.%s is required (see configs/config.example.yaml)", key)
	}
	return v, nil
}

func NewDependencies(sqlDB *sql.DB, driver string, coldIndex *coldindex.Index, logger *zap.Logger, serverVersion string) (*Dependencies, error) {
	if _, err := requirePrompt("system_message"); err != nil {
		return nil, err
	}
	analyzeDocPrompt, err := requirePrompt("analyze_document")
	if err != nil {
		return nil, err
	}
	answerPrompt, err := requirePrompt("answer_synthesis")
	if err != nil {
		return nil, err
	}
	filterDocsPrompt, err := requirePrompt("filter_documents")
	if err != nil {
		return nil, err
	}
	filterChapsPrompt, err := requirePrompt("filter_chapters")
	if err != nil {
		return nil, err
	}
	filterTopicsPrompt, err := requirePrompt("filter_topics")
	if err != nil {
		return nil, err
	}
	filterTagsPrompt, err := requirePrompt("filter_tags")
	if err != nil {
		return nil, err
	}

	cacheTTL := viper.GetDuration("storage.cache.ttl")
	if cacheTTL <= 0 {
		cacheTTL = 10 * time.Minute
	}
	cacheMax := viper.GetInt("storage.cache.max_size")
	cacheStore := cache.NewCache(cacheTTL, cacheMax)
	uow := db.NewUnitOfWork(sqlDB, driver, cacheStore)

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

	traceSvc := observability.NewTraceService(uow.OtelSpans)
	obsSvc := observability.NewObservabilityService(coldIndex)

	adminCfgView := adminconfig.NewAdminConfigViewService()

	topicSvc := catalog.NewTopicService(uow.Tags, uow.Topics)
	tagSvc := catalog.NewTagService(uow.Tags)

	llmProv, llmErr := llm.NewProviderFactory(logger).CreateProvider()
	if llmErr != nil {
		resolved := strings.ToLower(strings.TrimSpace(viper.GetString("llm.provider")))
		if resolved == "" {
			resolved = "openai"
		}
		logger.Warn("LLM provider creation failed; IQueryService and IDocumentMaintenanceService stay nil until fixed",
			zap.Error(llmErr),
			zap.String("llm.provider", resolved),
			zap.Bool("llm.openai.api_key_non_empty", strings.TrimSpace(viper.GetString("llm.openai.api_key")) != ""),
			zap.Bool("llm.anthropic.api_key_non_empty", strings.TrimSpace(viper.GetString("llm.anthropic.api_key")) != ""),
			zap.Bool("llm.local.base_url_non_empty", strings.TrimSpace(viper.GetString("llm.local.base_url")) != ""),
		)
	}

	analyzer := document.NewDocumentAnalysisGenerator(llmProv, analyzeDocPrompt, logger)
	persister := document.NewDocumentAnalysisPersister(uow.Chapters, uow.Documents, logger)
	hotIngestProc := document.NewHotIngestProcessor(uow.Documents, analyzer, persister, uow.Tags, logger)
	hotIngestSink := NewHotIngestQueueSink(logger)

	maintenance := document.NewDocumentMaintenanceService(uow.Documents, uow.Chapters, coldIndex, uow.DeletedDocuments, persister, analyzer, logger)
	if llmProv == nil {
		logger.Warn("LLM provider unavailable; cold→hot promotion (PromoteJob) will fail, but cold index refresh and hot score update still work")
	}

	docSvc := document.NewDocumentService(uow.Documents, coldIndex, uow.Tags, uow.Chapters, hotIngestSink, logger)

	chapterSvc := catalog.NewChapterService(uow.Chapters, uow.Documents, uow.Tags, uow.Topics, coldIndex, llmProv, filterDocsPrompt, filterChapsPrompt, filterTopicsPrompt, filterTagsPrompt, logger)

	var querySvc service.IQueryService
	if llmProv == nil {
		logger.Warn("LLM provider unavailable; progressive query disabled")
	} else {
		querySvc = queryimpl.NewQueryService(
			uow.Documents,
			chapterSvc,
			llmProv,
			answerPrompt,
			logger,
		)
	}

	sched := job.NewScheduler(logger)
	sched.Register(job.NewPromoteJob(maintenance))
	sched.Register(job.NewHotScoreJob(maintenance))
	sched.Register(job.NewColdIndexRefreshJob(maintenance, logger))
	sched.Register(job.NewTopicRegroupJob(topicSvc, logger))

	restHandler := api.NewHandler(
		docSvc,
		querySvc,
		topicSvc,
		tagSvc,
		chapterSvc,
		obsSvc,
		traceSvc,
		maintenance,
		logger,
		serverVersion,
	)
	mcpServer := api.NewMCPServer(restHandler, authService)

	return &Dependencies{
		OtelSpans: uow.OtelSpans,

		AuthService:        authService,
		TraceService:       traceSvc,
		AdminConfigView:    adminCfgView,
		TopicService:       topicSvc,
		DocumentService:    docSvc,
		HotIngestProcessor: hotIngestProc,
		QueryService:       querySvc,

		JobScheduler:        sched,
		DocumentMaintenance: maintenance,

		ColdIndex: coldIndex,

		RESTHandler: restHandler,
		MCPServer:   mcpServer,

		Logger: logger,
	}, nil
}
