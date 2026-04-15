package query

import (
	"context"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/service/svcimpl/common"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewRelevanceFilter constructs the service.IRelevanceFilter implementation.
func NewRelevanceFilter(provider client.ILLMProvider, logger *zap.Logger) service.IRelevanceFilter {
	return &relevanceFilter{core: common.NewSummarizerCore(provider, logger)}
}

type relevanceFilter struct {
	core *common.SummarizerCore
}

func (s *relevanceFilter) FilterDocuments(ctx context.Context, query string, docs []types.Document) ([]types.LLMFilterResult, error) {
	return s.core.FilterDocuments(ctx, query, docs)
}

func (s *relevanceFilter) FilterChapters(ctx context.Context, query string, chapters []types.Chapter) ([]types.LLMFilterResult, error) {
	return s.core.FilterChapters(ctx, query, chapters)
}

// FilterTopicsByQuery is used by query service via type assertion for adaptive topic narrowing.
func (s *relevanceFilter) FilterTopicsByQuery(ctx context.Context, query string, topics []types.Topic) ([]types.LLMFilterResult, error) {
	return s.core.FilterTopicsByQuery(ctx, query, topics)
}

// FilterTagsByQuery is used by query service via type assertion for adaptive tag filtering.
func (s *relevanceFilter) FilterTagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error) {
	return s.core.FilterTagsByQuery(ctx, query, tags)
}

var _ service.IRelevanceFilter = (*relevanceFilter)(nil)
