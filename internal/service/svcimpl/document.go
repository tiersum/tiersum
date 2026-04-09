package svcimpl

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/memory"
	"github.com/tiersum/tiersum/pkg/types"
)

// DocumentSvc implements service.IDocumentService
type DocumentSvc struct {
	docRepo       storage.IDocumentRepository
	indexer       service.IIndexer
	summarizer    service.ISummarizer
	tagRepo       storage.ITagRepository
	memIndex      storage.IInMemoryIndex
	quotaManager  *QuotaManager
	logger        *zap.Logger
}

// NewDocumentSvc creates a new document service
func NewDocumentSvc(
	docRepo storage.IDocumentRepository,
	indexer service.IIndexer,
	summarizer service.ISummarizer,
	tagRepo storage.ITagRepository,
	memIndex storage.IInMemoryIndex,
	quotaManager *QuotaManager,
	logger *zap.Logger,
) *DocumentSvc {
	return &DocumentSvc{
		docRepo:      docRepo,
		indexer:      indexer,
		summarizer:   summarizer,
		tagRepo:      tagRepo,
		memIndex:     memIndex,
		quotaManager: quotaManager,
		logger:       logger,
	}
}

// shouldBeHot determines if a document should be processed as "hot" based on quota and heuristics
func (s *DocumentSvc) shouldBeHot(content string, forceHot bool, hasPrebuiltSummary bool) bool {
	// If force hot, always process as hot
	if forceHot {
		return true
	}

	// If has prebuilt summary from external agent, process as hot (no LLM needed internally)
	if hasPrebuiltSummary {
		return true
	}

	// Check quota - if no quota available, process as cold
	if !s.quotaManager.CheckAndConsume() {
		return false
	}

	// Simple heuristic: documents longer than 5000 characters are considered "hot"
	return len(content) > 5000
}

// Ingest implements IDocumentService.Ingest
func (s *DocumentSvc) Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error) {
	// Check if document has prebuilt summary/tags from external agent
	hasPrebuiltSummary := req.Summary != "" && len(req.Chapters) > 0
	hasPrebuiltTags := len(req.Tags) > 0

	// Determine if document should be hot or cold
	isHot := s.shouldBeHot(req.Content, req.ForceHot, hasPrebuiltSummary)

	// Create document entity
	doc := &types.Document{
		ID:         uuid.New().String(),
		Title:      req.Title,
		Content:    req.Content,
		Format:     req.Format,
		Status:     types.DocStatusCold, // Default to cold
		HotScore:   0,
		QueryCount: 0,
	}

	if isHot {
		doc.Status = types.DocStatusHot

		// Hot document processing
		if hasPrebuiltSummary && hasPrebuiltTags {
			// External agent provided everything - no internal LLM needed
			doc.Tags = req.Tags
			
			// Create analysis result from prebuilt data
			analysis := &types.DocumentAnalysisResult{
				Summary:  req.Summary,
				Tags:     req.Tags,
				Chapters: req.Chapters,
			}

			// Index the document with prebuilt analysis
			if err := s.indexer.Index(ctx, doc, analysis); err != nil {
				s.logger.Error("failed to index document with prebuilt analysis", zap.Error(err))
			}
		} else if hasPrebuiltTags {
			// Has tags but needs summary/chapters from LLM
			analysis, err := s.summarizer.AnalyzeDocument(ctx, doc.Title, doc.Content)
			if err != nil {
				s.logger.Warn("failed to analyze document", zap.Error(err))
				analysis = &types.DocumentAnalysisResult{
					Summary:  truncateContent(doc.Content, 200),
					Tags:     req.Tags,
					Chapters: []types.ChapterInfo{},
				}
			}
			
			// Merge provided tags with generated tags
			tagSet := make(map[string]bool)
			for _, tag := range req.Tags {
				tagSet[tag] = true
			}
			for _, tag := range analysis.Tags {
				tagSet[tag] = true
			}
			mergedTags := make([]string, 0, len(tagSet))
			for tag := range tagSet {
				mergedTags = append(mergedTags, tag)
			}
			doc.Tags = mergedTags
			analysis.Tags = mergedTags

			// Index with analysis
			if err := s.indexer.Index(ctx, doc, analysis); err != nil {
				s.logger.Error("failed to index document", zap.Error(err))
			}
		} else {
			// No prebuilt data - use full LLM analysis flow
			analysis, err := s.summarizer.AnalyzeDocument(ctx, doc.Title, doc.Content)
			if err != nil {
				s.logger.Warn("failed to analyze document", zap.Error(err))
				analysis = &types.DocumentAnalysisResult{
					Summary:  truncateContent(doc.Content, 200),
					Tags:     []string{},
					Chapters: []types.ChapterInfo{},
				}
			}

			doc.Tags = analysis.Tags

			// Index the document with analysis results
			if err := s.indexer.Index(ctx, doc, analysis); err != nil {
				s.logger.Error("failed to index document", zap.Error(err))
			}
		}

		// Update global tags for hot documents
		for _, tag := range doc.Tags {
			tagEntity := &types.Tag{
				ID:      uuid.New().String(),
				Name:    tag,
				GroupID: "", // Will be assigned by clustering service
			}
			if err := s.tagRepo.Create(ctx, tagEntity); err != nil {
				s.logger.Warn("failed to create global tag", zap.String("tag", tag), zap.Error(err))
			} else {
				if err := s.tagRepo.IncrementDocumentCount(ctx, tag); err != nil {
					s.logger.Warn("failed to increment tag count", zap.String("tag", tag), zap.Error(err))
				}
			}
		}
	} else {
		// Cold document: minimal processing, no tags, no LLM
		doc.Status = types.DocStatusCold
		doc.Tags = []string{} // Cold documents have no tags

		// Generate simple embedding for vector index
		embedding := memory.GenerateSimpleEmbedding(doc.Content)

		// Add to memory index (bleve + vector)
		if s.memIndex != nil {
			if err := s.memIndex.AddDocument(doc, embedding); err != nil {
				s.logger.Warn("failed to add cold document to memory index",
					zap.String("doc_id", doc.ID),
					zap.Error(err))
			}
		}

		s.logger.Info("cold document ingested",
			zap.String("id", doc.ID),
			zap.Int("content_length", len(doc.Content)))
	}

	// Save document
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = doc.CreatedAt
	if err := s.docRepo.Create(ctx, doc); err != nil {
		return nil, err
	}

	s.logger.Info("document ingested",
		zap.String("id", doc.ID),
		zap.String("title", doc.Title),
		zap.String("status", string(doc.Status)),
		zap.Int("tags", len(doc.Tags)))

	return &types.CreateDocumentResponse{
		ID:           doc.ID,
		Title:        doc.Title,
		Format:       doc.Format,
		Tags:         doc.Tags,
		Summary:      "", // Could fetch from summary repo
		ChapterCount: 0,  // Could fetch from summary repo
		Status:       doc.Status,
		CreatedAt:    doc.CreatedAt,
	}, nil
}

// Get implements IDocumentService.Get
func (s *DocumentSvc) Get(ctx context.Context, id string) (*types.Document, error) {
	return s.docRepo.GetByID(ctx, id)
}

// GetRecent implements IDocumentService.GetRecent
func (s *DocumentSvc) GetRecent(ctx context.Context, limit int) ([]*types.Document, error) {
	return s.docRepo.GetRecent(ctx, limit)
}

// List implements IDocumentService.List
func (s *DocumentSvc) List(ctx context.Context) ([]types.Document, error) {
	return s.docRepo.ListAll(ctx, 1000)
}

// truncateContent truncates content to max length
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

var _ service.IDocumentService = (*DocumentSvc)(nil)
