// Package di provides dependency injection and application initialization
// This is the composition root where all concrete implementations are wired together
package di

import (
	"database/sql"

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
)

// Dependencies holds all application dependencies
type Dependencies struct {
	// Service Layer interfaces
	DocumentService service.IDocumentService
	QueryService    service.IQueryService

	// Tag Clustering Service
	TagClusteringService service.ITagClusteringService

	// API Layer
	RESTHandler *api.Handler   // REST API
	MCPServer   *api.MCPServer // MCP protocol

	// Job Layer
	JobScheduler *job.Scheduler

	// Logger
	Logger *zap.Logger
}

// NewDependencies creates all dependencies with proper wiring
func NewDependencies(sqlDB *sql.DB, driver string, logger *zap.Logger) (*Dependencies, error) {
	// 1. Storage Layer - Cache
	cacheStore := cache.NewCache(0)

	// 2. Storage Layer - DB
	uow := db.NewUnitOfWork(sqlDB, driver, cacheStore)

	// 3. Client Layer - LLM
	llmProvider := llm.NewOpenAIProvider()

	// 4. Service Layer - Core domain logic
	summarizer := svcimpl.NewSummarizerSvc(llmProvider, logger)
	indexer := svcimpl.NewIndexerSvc(summarizer, uow.Summaries, logger)

	// 5. Service Layer - Tag clustering
	tagClusteringSvc := svcimpl.NewTagClusteringSvc(
		uow.GlobalTags,
		uow.TagClusters,
		uow.ClusterRefreshLogs,
		llmProvider,
		logger,
	)

	// 6. Service Layer - Business logic
	queryService := svcimpl.NewQuerySvc(
		uow.Documents,
		uow.Summaries,
		uow.GlobalTags,
		uow.TagClusters,
		summarizer,
		logger,
	)
	docService := svcimpl.NewDocumentSvc(
		uow.Documents,
		indexer,
		summarizer,
		uow.GlobalTags,
		logger,
	)

	// 7. API Layer
	restHandler := api.NewHandler(docService, queryService, tagClusteringSvc, logger)
	mcpServer := api.NewMCPServer(docService, queryService, tagClusteringSvc, logger)

	// 8. Job Layer
	jobScheduler := job.NewScheduler(logger)
	// Register tag clustering job (runs every 30 minutes)
	jobScheduler.Register(job.NewTagClusteringJob(tagClusteringSvc, logger))
	jobScheduler.Register(job.NewIndexerJob(uow.Documents, uow.Summaries, indexer, logger))

	return &Dependencies{
		DocumentService:      docService,
		QueryService:         queryService,
		TagClusteringService: tagClusteringSvc,
		RESTHandler:          restHandler,
		MCPServer:            mcpServer,
		JobScheduler:         jobScheduler,
		Logger:               logger,
	}, nil
}

// Interface compliance checks
var (
	// Storage Layer
	_ storage.IDocumentRepository          = (*db.DocumentRepo)(nil)
	_ storage.ISummaryRepository           = (*db.SummaryRepo)(nil)
	_ storage.IGlobalTagRepository         = (*db.GlobalTagRepo)(nil)
	_ storage.ITagClusterRepository        = (*db.TagClusterRepo)(nil)
	_ storage.IClusterRefreshLogRepository = (*db.ClusterRefreshLogRepo)(nil)
	_ storage.ICache                       = (*cache.Cache)(nil)

	// Service Layer
	_ service.IDocumentService      = (*svcimpl.DocumentSvc)(nil)
	_ service.IQueryService         = (*svcimpl.QuerySvc)(nil)
	_ service.ITagClusteringService = (*svcimpl.TagClusteringSvc)(nil)
	_ service.IIndexer              = (*svcimpl.IndexerSvc)(nil)
	_ service.ISummarizer           = (*svcimpl.SummarizerSvc)(nil)
	_ service.ILLMFilter            = (*svcimpl.SummarizerSvc)(nil)

	// Client Layer
	_ client.ILLMProvider = (*llm.OpenAIProvider)(nil)
)
