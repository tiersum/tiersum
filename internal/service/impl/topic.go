package impl

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// TopicSvc implements service.ITopicService
type TopicSvc struct {
	topicRepo  storage.ITopicSummaryRepository
	docRepo    storage.IDocumentRepository
	summarizer service.ISummarizer
	logger     *zap.Logger
}

// NewTopicSvc creates a new topic service
func NewTopicSvc(topicRepo storage.ITopicSummaryRepository, docRepo storage.IDocumentRepository, summarizer service.ISummarizer, logger *zap.Logger) *TopicSvc {
	return &TopicSvc{
		topicRepo:   topicRepo,
		docRepo:     docRepo,
		summarizer:  summarizer,
		logger:      logger,
	}
}

// CreateTopicFromDocuments implements ITopicService.CreateTopicFromDocuments
func (s *TopicSvc) CreateTopicFromDocuments(ctx context.Context, topicName string, docIDs []string) (*types.TopicSummary, error) {
	s.logger.Info("creating topic from documents", zap.String("topic", topicName), zap.Int("doc_count", len(docIDs)))

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

	topic, err := s.summarizer.GenerateTopicSummary(ctx, topicName, docs)
	if err != nil {
		return nil, fmt.Errorf("generate topic summary: %w", err)
	}

	if err := s.topicRepo.Create(ctx, topic); err != nil {
		return nil, fmt.Errorf("save topic summary: %w", err)
	}

	s.logger.Info("topic created successfully", zap.String("topic_id", topic.ID))
	return topic, nil
}

// GetTopic implements ITopicService.GetTopic
func (s *TopicSvc) GetTopic(ctx context.Context, id string) (*types.TopicSummary, error) {
	return s.topicRepo.GetByID(ctx, id)
}

// ListTopics implements ITopicService.ListTopics
func (s *TopicSvc) ListTopics(ctx context.Context) ([]types.TopicSummary, error) {
	return s.topicRepo.List(ctx)
}

// FindTopicsByTags implements ITopicService.FindTopicsByTags
func (s *TopicSvc) FindTopicsByTags(ctx context.Context, tags []string) ([]types.TopicSummary, error) {
	return s.topicRepo.FindByTags(ctx, tags)
}

var _ service.ITopicService = (*TopicSvc)(nil)
