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
	JobScheduler        *job.Scheduler
	DocumentMaintenance service.IDocumentMaintenanceService

	// ColdIndex is the concrete cold-document index (memory.Index).
	ColdIndex *memory.Index

	// Logger
	Logger *zap.Logger
}

// NewDependencies creates all dependencies with proper wiring
func NewDependencies(sqlDB *sql.DB, driver string, coldIndex *memory.Index, logger *zap.Logger) (*Dependencies, error) {
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
		coldIndex,
		llmProvider,
		logger,
	)
	docService := svcimpl.NewDocumentSvc(
		uow.Documents,
		indexer,
		summarizer,
		uow.Tags,
		coldIndex,
		quotaManager,
		logger,
	)

	// 8. API Layer — retrieval facade so HTTP handlers do not import storage interfaces
	retrievalSvc := svcimpl.NewRetrievalSvc(uow.Tags, uow.Summaries, uow.Documents, coldIndex)
	restHandler := api.NewHandler(docService, queryService, tagGroupSvc, retrievalSvc, quotaManager, logger)
	mcpServer := api.NewMCPServer(restHandler, logger)

	// 9. Job Layer — jobs depend only on service.* contracts
	docMaintenance := svcimpl.NewDocumentMaintenanceSvc(uow.Documents, indexer, summarizer, logger)
	jobScheduler := job.NewScheduler(logger)
	jobScheduler.Register(job.NewTagGroupJob(tagGroupSvc, logger))
	jobScheduler.Register(job.NewPromoteJob(docMaintenance))
	jobScheduler.Register(job.NewHotScoreJob(docMaintenance))

	return &Dependencies{
		DocumentService:     docService,
		QueryService:        queryService,
		TagGroupService:     tagGroupSvc,
		RESTHandler:         restHandler,
		MCPServer:           mcpServer,
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
	_ storage.ISummaryRepository  = (*db.SummaryRepo)(nil)
	_ storage.ITagRepository      = (*db.TagRepo)(nil)
	_ storage.ITagGroupRepository = (*db.TagGroupRepo)(nil)
	_ storage.ICache              = (*cache.Cache)(nil)
	_ storage.IColdIndex          = (*memory.Index)(nil)

	// Service Layer
	_ service.IDocumentService = (*svcimpl.DocumentSvc)(nil)
	_ service.IQueryService    = (*svcimpl.QuerySvc)(nil)
	_ service.ITagGroupService = (*svcimpl.TagGroupSvc)(nil)
	_ service.IIndexer         = (*svcimpl.IndexerSvc)(nil)
	_ service.ISummarizer       = (*svcimpl.SummarizerSvc)(nil)
	_ service.IRetrievalService           = (*svcimpl.RetrievalSvc)(nil)
	_ service.IDocumentMaintenanceService = (*svcimpl.DocumentMaintenanceSvc)(nil)

	// Client Layer
	_ client.ILLMProvider = (*llm.OpenAIProvider)(nil)
	_ client.ILLMProvider = (*llm.AnthropicProvider)(nil)
	_ client.ILLMProvider = (*llm.OllamaProvider)(nil)
)
