// Package di provides dependency injection and application initialization
// This is the composition root where all concrete implementations are wired together
package di

import (
	"database/sql"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/api"
	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/client/llm"
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/service/svcimpl"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/cache"
	"github.com/tiersum/tiersum/internal/storage/db"
	"github.com/tiersum/tiersum/internal/storage/memory"
)

// Dependencies holds all application dependencies
type Dependencies struct {
	// Service Layer interfaces
	DocumentService service.IDocumentService
	QueryService    service.IQueryService

	// Tag Grouping Service
	TagGroupService service.ITagGroupService

	// API Layer
	RESTHandler *api.Handler   // REST API
	MCPServer   *api.MCPServer // MCP protocol

	// Job Layer
	JobScheduler *job.Scheduler
	PromoteJob   *job.PromoteJob

	// Memory Index for cold documents
	MemIndex *memory.Index

	// Logger
	Logger *zap.Logger
}

// NewDependencies creates all dependencies with proper wiring
func NewDependencies(sqlDB *sql.DB, driver string, memIndex *memory.Index, logger *zap.Logger) (*Dependencies, error) {
	// 1. Storage Layer - Cache
	cacheStore := cache.NewCache(0)

	// 2. Storage Layer - DB
	uow := db.NewUnitOfWork(sqlDB, driver, cacheStore)

	// 3. Client Layer - LLM
	factory := llm.NewProviderFactory(logger)
	llmProvider, err := factory.CreateProvider()
	if err != nil {
		return nil, err
	}

	// 4. Quota Manager - for hot/cold document processing control
	quotaPerHour := viper.GetInt("quota.per_hour")
	if quotaPerHour <= 0 {
		quotaPerHour = 100 // default
	}
	quotaManager := svcimpl.NewQuotaManager(quotaPerHour)

	// 5. Service Layer - Core domain logic
	summarizer := svcimpl.NewSummarizerSvc(llmProvider, logger)
	indexer := svcimpl.NewIndexerSvc(summarizer, uow.Summaries, logger)

	// 6. Service Layer - Tag grouping
	tagGroupSvc := svcimpl.NewTagGroupSvc(
		uow.Tags,
		uow.TagGroups,
		llmProvider,
		logger,
	)

	// 7. Service Layer - Business logic
	queryService := svcimpl.NewQuerySvc(
		uow.Documents,
		uow.Summaries,
		uow.Tags,
		uow.TagGroups,
		summarizer,
		memIndex,
		llmProvider,
		logger,
	)
	docService := svcimpl.NewDocumentSvc(
		uow.Documents,
		indexer,
		summarizer,
		uow.Tags,
		memIndex,
		quotaManager,
		logger,
	)

	// 8. API Layer
	restHandler := api.NewHandler(docService, queryService, tagGroupSvc, uow.Tags, uow.Summaries, uow.Documents, memIndex, quotaManager, logger)
	mcpServer := api.NewMCPServer(docService, queryService, tagGroupSvc, logger)

	// 9. Job Layer
	jobScheduler := job.NewScheduler(logger)
	// Register tag grouping job (runs every 30 minutes)
	jobScheduler.Register(job.NewTagGroupJob(tagGroupSvc, logger))
	promoteJob := job.NewPromoteJob(uow.Documents, indexer, summarizer, logger)
	// Register hot/cold document promotion job (runs every 5 minutes)
	jobScheduler.Register(promoteJob)
	// Register hot score update job (runs every hour)
	jobScheduler.Register(job.NewHotScoreJob(uow.Documents, logger))

	return &Dependencies{
		DocumentService: docService,
		QueryService:    queryService,
		TagGroupService: tagGroupSvc,
		RESTHandler:     restHandler,
		MCPServer:       mcpServer,
		JobScheduler:    jobScheduler,
		PromoteJob:      promoteJob,
		MemIndex:        memIndex,
		Logger:          logger,
	}, nil
}

// Interface compliance checks
var (
	// Storage Layer
	_ storage.IDocumentRepository = (*db.DocumentRepo)(nil)
	_ storage.ISummaryRepository  = (*db.SummaryRepo)(nil)
	_ storage.ITagRepository      = (*db.TagRepo)(nil)
	_ storage.ITagGroupRepository = (*db.TagGroupRepo)(nil)
	_ storage.ICache              = (*cache.Cache)(nil)
	_ storage.IInMemoryIndex      = (*memory.Index)(nil)

	// Service Layer
	_ service.IDocumentService = (*svcimpl.DocumentSvc)(nil)
	_ service.IQueryService    = (*svcimpl.QuerySvc)(nil)
	_ service.ITagGroupService = (*svcimpl.TagGroupSvc)(nil)
	_ service.IIndexer         = (*svcimpl.IndexerSvc)(nil)
	_ service.ISummarizer      = (*svcimpl.SummarizerSvc)(nil)
	_ service.ILLMFilter       = (*svcimpl.SummarizerSvc)(nil)

	// Client Layer
	_ client.ILLMProvider = (*llm.OpenAIProvider)(nil)
	_ client.ILLMProvider = (*llm.AnthropicProvider)(nil)
	_ client.ILLMProvider = (*llm.OllamaProvider)(nil)
)
