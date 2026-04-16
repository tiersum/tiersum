package catalog

import (
	"context"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewTagService constructs the service.ITagService implementation.
func NewTagService(tagRepo storage.ITagRepository) service.ITagService {
	return &tagService{tagRepo: tagRepo}
}

type tagService struct {
	tagRepo storage.ITagRepository
}

func (s *tagService) ListTags(ctx context.Context, topicIDs []string, byTopicLimit int, listAllCap int) ([]types.Tag, error) {
	if len(topicIDs) > 0 {
		if byTopicLimit <= 0 {
			byTopicLimit = 100
		}
		return s.tagRepo.ListByTopicIDs(ctx, topicIDs, byTopicLimit)
	}
	tags, err := s.tagRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	if listAllCap > 0 && len(tags) > listAllCap {
		tags = tags[:listAllCap]
	}
	return tags, nil
}

var _ service.ITagService = (*tagService)(nil)

