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
	TopicService    service.ITopicService

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
	parser := svcimpl.NewParserSvc()
	summarizer := svcimpl.NewSummarizerSvc(llmProvider, logger)
	indexer := svcimpl.NewIndexerSvc(parser, summarizer, uow.Summaries, logger)

	// 5. Service Layer - Business logic
	// Note: TopicService created first as DocumentService depends on it for auto-matching
	queryService := svcimpl.NewQuerySvc(uow.Documents, uow.Summaries, logger)
	topicService := svcimpl.NewTopicSvc(uow.TopicSummaries, uow.Documents, summarizer, logger)
	docService := svcimpl.NewDocumentSvc(uow.Documents, indexer, summarizer, topicService, logger)

	// 6. API Layer
	restHandler := api.NewHandler(docService, queryService, topicService, logger)
	mcpServer := api.NewMCPServer(docService, queryService, topicService, logger)

	// 7. Job Layer
	jobScheduler := job.NewScheduler(logger)
	jobScheduler.Register(job.NewIndexerJob(uow.Documents, uow.Summaries, indexer, logger))
	jobScheduler.Register(job.NewTopicAggregatorJob(topicService, logger))
	jobScheduler.Register(job.NewCacheCleanupJob(cacheStore, logger))

	return &Dependencies{
		DocumentService: docService,
		QueryService:    queryService,
		TopicService:    topicService,
		RESTHandler:     restHandler,
		MCPServer:       mcpServer,
		JobScheduler:    jobScheduler,
		Logger:          logger,
	}, nil
}

// Interface compliance checks
var (
	// Storage Layer
	_ storage.IDocumentRepository     = (*db.DocumentRepo)(nil)
	_ storage.ISummaryRepository      = (*db.SummaryRepo)(nil)
	_ storage.ITopicSummaryRepository = (*db.TopicSummaryRepo)(nil)
	_ storage.ICache                  = (*cache.Cache)(nil)

	// Service Layer
	_ service.IDocumentService = (*svcimpl.DocumentSvc)(nil)
	_ service.IQueryService    = (*svcimpl.QuerySvc)(nil)
	_ service.ITopicService    = (*svcimpl.TopicSvc)(nil)
	_ service.IIndexer         = (*svcimpl.IndexerSvc)(nil)
	_ service.ISummarizer      = (*svcimpl.SummarizerSvc)(nil)
	_ service.IParser          = (*svcimpl.ParserSvc)(nil)

	// Client Layer
	_ client.ILLMProvider = (*llm.OpenAIProvider)(nil)
)
