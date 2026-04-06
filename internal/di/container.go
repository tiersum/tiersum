// Package di provides dependency injection and application initialization
// This is the composition root where all concrete implementations are wired together
package di

import (
	"database/sql"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/api"
	"github.com/tiersum/tiersum/internal/client/llm"
	"github.com/tiersum/tiersum/internal/core"
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/ports"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage/cache"
	"github.com/tiersum/tiersum/internal/storage/db"
)

// Dependencies holds all application dependencies
type Dependencies struct {
	// Service Layer (business logic)
	DocumentService ports.DocumentService
	QueryService    ports.QueryService
	TopicService    *service.TopicSvc

	// API Layer (thin layer, depends on services)
	RESTHandler *api.Handler     // REST API handler
	MCPServer   *api.MCPServer   // MCP protocol handler

	// Job Layer (background tasks)
	JobScheduler *job.Scheduler

	// Logger
	Logger *zap.Logger
}

// NewDependencies creates all dependencies with proper wiring
// This is the composition root where all concrete implementations are bound to interfaces
func NewDependencies(sqlDB *sql.DB, driver string, logger *zap.Logger) (*Dependencies, error) {
	// 1. Storage Layer - Cache
	cacheStore := cache.NewCache(0) // 0 = use default TTL

	// 2. Storage Layer - DB (Repository implementations)
	uow := db.NewUnitOfWork(sqlDB, driver, cacheStore)

	// 3. Client Layer - LLM provider
	llmProvider := llm.NewOpenAIProvider()

	// 4. Core Layer - Domain logic services
	parser := core.NewParserSvc()
	summarizer := core.NewSummarizerSvc(llmProvider, logger)
	indexer := core.NewIndexerSvc(parser, summarizer, uow.Summaries, logger)

	// 5. Service Layer - Business logic
	docService := service.NewDocumentSvc(uow.Documents, indexer, summarizer, logger)
	queryService := service.NewQuerySvc(uow.Documents, uow.Summaries, logger)
	topicService := service.NewTopicSvc(uow.TopicSummaries, uow.Documents, summarizer, logger)

	// 6. API Layer - REST API
	restHandler := api.NewHandler(docService, queryService, topicService, logger)

	// 7. API Layer - MCP Server
	mcpServer := api.NewMCPServer(docService, queryService, topicService, logger)

	// 8. Job Layer - Background tasks
	jobScheduler := job.NewScheduler(logger)

	// Register jobs
	indexerJob := job.NewIndexerJob(uow.Documents, uow.Summaries, indexer, logger)
	topicAggJob := job.NewTopicAggregatorJob(uow.TopicSummaries, uow.Documents, summarizer, logger)
	cacheCleanupJob := job.NewCacheCleanupJob(cacheStore, logger)

	jobScheduler.Register(indexerJob)
	jobScheduler.Register(topicAggJob)
	jobScheduler.Register(cacheCleanupJob)

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
// Ensure all implementations satisfy their interface contracts
var (
	// Storage Layer - DB implementations
	_ ports.DocumentRepository     = (*db.DocumentRepo)(nil)
	_ ports.SummaryRepository      = (*db.SummaryRepo)(nil)
	_ ports.TopicSummaryRepository = (*db.TopicSummaryRepo)(nil)

	// Storage Layer - Cache implementation
	_ ports.Cache = (*cache.Cache)(nil)

	// Service Layer implementations
	_ ports.DocumentService = (*service.DocumentSvc)(nil)
	_ ports.QueryService    = (*service.QuerySvc)(nil)

	// Core Layer implementations
	_ ports.Parser     = (*core.ParserSvc)(nil)
	_ ports.Summarizer = (*core.SummarizerSvc)(nil)
	_ ports.Indexer    = (*core.IndexerSvc)(nil)

	// Client Layer implementations
	_ ports.LLMProvider = (*llm.OpenAIProvider)(nil)
)