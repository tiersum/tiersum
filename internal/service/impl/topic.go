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

// AddDocumentToTopics adds a document to matching topics based on tag overlap
// A document is added to a topic if they share at least 2 tags (or 1 if doc has only 1 tag)
func (s *TopicSvc) AddDocumentToTopics(ctx context.Context, docID string, docTags []string) (int, error) {
	if len(docTags) == 0 {
		return 0, nil
	}

	// Find topics with matching tags
	matchingTopics, err := s.topicRepo.FindByTags(ctx, docTags)
	if err != nil {
		return 0, fmt.Errorf("find topics by tags: %w", err)
	}

	addedCount := 0
	for _, topic := range matchingTopics {
		// Check tag overlap
		overlap := countOverlap(docTags, topic.Tags)
		minOverlap := 2
		if len(docTags) == 1 {
			minOverlap = 1
		}

		if overlap >= minOverlap {
			// Check if document is already in topic
			alreadyInTopic := false
			for _, existingDocID := range topic.DocumentIDs {
				if existingDocID == docID {
					alreadyInTopic = true
					break
				}
			}

			if !alreadyInTopic {
				if err := s.topicRepo.AddDocument(ctx, topic.ID, docID); err != nil {
					s.logger.Warn("failed to add document to topic",
						zap.String("doc_id", docID),
						zap.String("topic_id", topic.ID),
						zap.Error(err))
				} else {
					addedCount++
					s.logger.Info("added document to topic",
						zap.String("doc_id", docID),
						zap.String("topic_name", topic.Name))
				}
			}
		}
	}

	return addedCount, nil
}

// AutoCreateTopicFromTag creates a new topic from documents sharing a specific tag
// Only creates topic if there are at least minDocs documents
func (s *TopicSvc) AutoCreateTopicFromTag(ctx context.Context, tag string, minDocs int) (*types.TopicSummary, error) {
	// Find topics that already have this tag
	existingTopics, err := s.topicRepo.FindByTags(ctx, []string{tag})
	if err != nil {
		return nil, fmt.Errorf("find existing topics: %w", err)
	}

	// If a topic with this exact name exists, skip
	for _, topic := range existingTopics {
		if topic.Name == tag {
			s.logger.Info("topic with tag name already exists", zap.String("tag", tag))
			return nil, nil
		}
	}

	// TODO: Query all documents with this tag
	// This requires a document repository method to find by tags
	// For now, we return nil as placeholder
	s.logger.Info("auto-create topic from tag",
		zap.String("tag", tag),
		zap.Int("min_docs", minDocs),
		zap.String("status", "requires document query by tags"))

	return nil, nil
}

// countOverlap counts common elements between two slices
func countOverlap(a, b []string) int {
	count := 0
	bMap := make(map[string]bool)
	for _, x := range b {
		bMap[x] = true
	}
	for _, x := range a {
		if bMap[x] {
			count++
		}
	}
	return count
}

var _ service.ITopicService = (*TopicSvc)(nil)
