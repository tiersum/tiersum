// Package app provides dependency injection and application initialization
// This is the only place where concrete implementations are wired together
package app

import (
	"database/sql"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/adapters/llm"
	"github.com/tiersum/tiersum/internal/adapters/repository"
	"github.com/tiersum/tiersum/internal/api"
	"github.com/tiersum/tiersum/internal/domain/core"
	"github.com/tiersum/tiersum/internal/domain/service"
	"github.com/tiersum/tiersum/internal/ports"
	"github.com/tiersum/tiersum/internal/storage"
)

// Dependencies holds all application dependencies
type Dependencies struct {
	// Services (interfaces)
	DocumentService ports.DocumentService
	QueryService    ports.QueryService

	// API Handler
	Handler *api.Handler

	// Infrastructure
	Storage *storage.Storage
	Logger  *zap.Logger
}

// NewDependencies creates all dependencies with proper wiring
// This is the composition root where all concrete implementations are bound to interfaces
func NewDependencies(db *sql.DB, driver string, logger *zap.Logger) (*Dependencies, error) {
	// 1. Infrastructure Layer
	cache := storage.NewCache(0) // 0 = use default TTL from config
	store := &storage.Storage{
		// DB and Cache fields should be accessed through interfaces
	}

	// 2. Repository Layer (implements ports interfaces)
	uow := repository.NewUnitOfWork(db, driver, cache)

	// 3. Infrastructure Adapters (implements ports interfaces)
	llmProvider := llm.NewOpenAIProvider()

	// 4. Core Domain Services (implements ports interfaces)
	parser := core.NewParserSvc()
	summarizer := core.NewSummarizerSvc(llmProvider, logger)
	indexer := core.NewIndexerSvc(parser, summarizer, uow.Summaries, logger)

	// 5. Application Services (implements ports interfaces)
	docService := service.NewDocumentSvc(uow.Documents, indexer, logger)
	queryService := service.NewQuerySvc(uow.Documents, uow.Summaries, logger)

	// 6. API Layer (depends on service interfaces)
	handler := api.NewHandler(docService, queryService, logger)

	return &Dependencies{
		DocumentService: docService,
		QueryService:    queryService,
		Handler:         handler,
		Storage:         store,
		Logger:          logger,
	}, nil
}

// Interface compliance checks
// Ensure all implementations satisfy their interface contracts
var (
	// Repositories implement ports interfaces
	_ ports.DocumentRepository = (*repository.DocumentRepo)(nil)
	_ ports.SummaryRepository  = (*repository.SummaryRepo)(nil)

	// Services implement ports interfaces
	_ ports.DocumentService = (*service.DocumentSvc)(nil)
	_ ports.QueryService    = (*service.QuerySvc)(nil)

	// Core services implement ports interfaces
	_ ports.Parser     = (*core.ParserSvc)(nil)
	_ ports.Summarizer = (*core.SummarizerSvc)(nil)
	_ ports.Indexer    = (*core.IndexerSvc)(nil)

	// Infrastructure implements ports interfaces
	_ ports.LLMProvider = (*llm.OpenAIProvider)(nil)
)
