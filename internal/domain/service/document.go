// Package service implements business logic interfaces defined in ports
package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/ports"
	"github.com/tiersum/tiersum/pkg/types"
)

// DocumentSvc implements ports.DocumentService
type DocumentSvc struct {
	repo       ports.DocumentRepository
	indexer    ports.Indexer
	summarizer ports.Summarizer
	logger     *zap.Logger
}

// NewDocumentSvc creates a new document service
func NewDocumentSvc(repo ports.DocumentRepository, indexer ports.Indexer, summarizer ports.Summarizer, logger *zap.Logger) *DocumentSvc {
	return &DocumentSvc{
		repo:       repo,
		indexer:    indexer,
		summarizer: summarizer,
		logger:     logger,
	}
}

// Ingest implements ports.DocumentService.Ingest
// Uses LLM to analyze document and generate tags if not provided
func (s *DocumentSvc) Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.Document, error) {
	doc := &types.Document{
		Title:   req.Title,
		Content: req.Content,
		Format:  req.Format,
		Tags:    req.Tags,
	}

	// Use LLM to generate tags if not provided
	if len(doc.Tags) == 0 && s.summarizer != nil {
		s.logger.Info("generating tags via LLM", zap.String("title", doc.Title))
		analysis, err := s.summarizer.AnalyzeDocument(ctx, doc.Title, doc.Content)
		if err != nil {
			s.logger.Warn("failed to analyze document, continuing without tags", zap.Error(err))
		} else {
			doc.Tags = analysis.Tags
			s.logger.Info("generated tags", zap.Strings("tags", doc.Tags))
		}
	}

	if err := s.repo.Create(ctx, doc); err != nil {
		return nil, err
	}

	// Async indexing
	if s.indexer != nil {
		go func() {
			if err := s.indexer.Index(context.Background(), doc.ID, doc.Content); err != nil {
				s.logger.Error("failed to index document", zap.String("id", doc.ID), zap.Error(err))
			}
		}()
	}

	return doc, nil
}

// Get implements ports.DocumentService.Get
func (s *DocumentSvc) Get(ctx context.Context, id string) (*types.Document, error) {
	return s.repo.GetByID(ctx, id)
}

// QuerySvc implements ports.QueryService
type QuerySvc struct {
	summaryRepo ports.SummaryRepository
	docRepo     ports.DocumentRepository
	logger      *zap.Logger
}

// NewQuerySvc creates a new query service
func NewQuerySvc(docRepo ports.DocumentRepository, summaryRepo ports.SummaryRepository, logger *zap.Logger) *QuerySvc {
	return &QuerySvc{
		docRepo:     docRepo,
		summaryRepo: summaryRepo,
		logger:      logger,
	}
}

// Query implements ports.QueryService.Query
func (s *QuerySvc) Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error) {
	// TODO: Implement hierarchical query logic
	// This is a simplified version
	return nil, nil
}

// TopicSvc provides topic management functionality
type TopicSvc struct {
	topicRepo   ports.TopicSummaryRepository
	docRepo     ports.DocumentRepository
	summarizer  ports.Summarizer
	logger      *zap.Logger
}

// NewTopicSvc creates a new topic service
func NewTopicSvc(topicRepo ports.TopicSummaryRepository, docRepo ports.DocumentRepository, summarizer ports.Summarizer, logger *zap.Logger) *TopicSvc {
	return &TopicSvc{
		topicRepo:   topicRepo,
		docRepo:     docRepo,
		summarizer:  summarizer,
		logger:      logger,
	}
}

// CreateTopicFromDocuments creates a new topic summary from a list of documents
func (s *TopicSvc) CreateTopicFromDocuments(ctx context.Context, topicName string, docIDs []string) (*types.TopicSummary, error) {
	s.logger.Info("creating topic from documents", zap.String("topic", topicName), zap.Int("doc_count", len(docIDs)))

	// Fetch all documents
	docs := make([]*types.Document, 0, len(docIDs))
	for _, docID := range docIDs {
		doc, err := s.docRepo.GetByID(ctx, docID)
		if err != nil {
			return nil, fmt.Errorf("fetch document %s: %w", docID, err)
		}
		if doc != nil {
			docs = append(docs, doc)
		}
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no valid documents found")
	}

	// Generate topic summary using LLM
	topic, err := s.summarizer.GenerateTopicSummary(ctx, topicName, docs)
	if err != nil {
		return nil, fmt.Errorf("generate topic summary: %w", err)
	}

	// Save to repository
	if err := s.topicRepo.Create(ctx, topic); err != nil {
		return nil, fmt.Errorf("save topic summary: %w", err)
	}

	s.logger.Info("topic created successfully", zap.String("topic_id", topic.ID))
	return topic, nil
}

// GetTopic retrieves a topic by ID
func (s *TopicSvc) GetTopic(ctx context.Context, id string) (*types.TopicSummary, error) {
	return s.topicRepo.GetByID(ctx, id)
}

// ListTopics lists all topics
func (s *TopicSvc) ListTopics(ctx context.Context) ([]types.TopicSummary, error) {
	return s.topicRepo.List(ctx)
}

// FindTopicsByTags finds topics matching the given tags
func (s *TopicSvc) FindTopicsByTags(ctx context.Context, tags []string) ([]types.TopicSummary, error) {
	return s.topicRepo.FindByTags(ctx, tags)
}

// Compile-time interface checks
var (
	_ ports.DocumentService = (*DocumentSvc)(nil)
	_ ports.QueryService    = (*QuerySvc)(nil)
)