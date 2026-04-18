package document

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

type listHotMetaRepo struct {
	storage.IDocumentRepository

	lastTags     []string
	lastStatuses []types.DocumentStatus
	lastLimit    int

	ret []types.Document
}

func (r *listHotMetaRepo) Create(ctx context.Context, doc *types.Document) error { return nil }
func (r *listHotMetaRepo) GetByID(ctx context.Context, id string) (*types.Document, error) {
	return nil, nil
}
func (r *listHotMetaRepo) GetRecent(ctx context.Context, limit int) ([]*types.Document, error) {
	return nil, nil
}
func (r *listHotMetaRepo) ListByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error) {
	return nil, nil
}
func (r *listHotMetaRepo) ListMetaByTagsAndStatuses(ctx context.Context, tags []string, statuses []types.DocumentStatus, limit int) ([]types.Document, error) {
	r.lastTags = append([]string(nil), tags...)
	r.lastStatuses = append([]types.DocumentStatus(nil), statuses...)
	r.lastLimit = limit
	return r.ret, nil
}
func (r *listHotMetaRepo) ListByStatus(ctx context.Context, status types.DocumentStatus, limit int) ([]types.Document, error) {
	return nil, nil
}
func (r *listHotMetaRepo) IncrementQueryCount(ctx context.Context, docID string) error { return nil }
func (r *listHotMetaRepo) UpdateStatus(ctx context.Context, docID string, status types.DocumentStatus) error {
	return nil
}
func (r *listHotMetaRepo) UpdateHotScore(ctx context.Context, docID string, score float64) error { return nil }
func (r *listHotMetaRepo) UpdateTags(ctx context.Context, docID string, tags []string) error { return nil }
func (r *listHotMetaRepo) UpdateSummary(ctx context.Context, docID string, summary string) error { return nil }
func (r *listHotMetaRepo) ListAll(ctx context.Context, limit int) ([]types.Document, error) { return nil, nil }
func (r *listHotMetaRepo) CountDocumentsByStatus(ctx context.Context) (types.DocumentStatusCounts, error) {
	return types.DocumentStatusCounts{}, nil
}

var _ storage.IDocumentRepository = (*listHotMetaRepo)(nil)

func TestListHotDocumentsWithSummariesByTags_DedupesTagsAndStatuses(t *testing.T) {
	repo := &listHotMetaRepo{
		ret: []types.Document{
			{ID: "a", Title: "T", Summary: "S", Status: types.DocStatusHot},
		},
	}
	svc := NewDocumentService(repo, nil, nil, nil, nil, nil, zap.NewNop())

	out, err := svc.ListHotDocumentsWithSummariesByTags(context.Background(), []string{" k8s ", "k8s", "docker"}, 50)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0].ID)

	require.Equal(t, []string{"k8s", "docker"}, repo.lastTags)
	require.Equal(t, []types.DocumentStatus{types.DocStatusHot, types.DocStatusWarming}, repo.lastStatuses)
	require.Equal(t, 50, repo.lastLimit)
}

func TestListHotDocumentsWithSummariesByTags_EmptyAfterDedupe(t *testing.T) {
	repo := &listHotMetaRepo{}
	svc := NewDocumentService(repo, nil, nil, nil, nil, nil, zap.NewNop())

	out, err := svc.ListHotDocumentsWithSummariesByTags(context.Background(), []string{"  ", ""}, 10)
	require.NoError(t, err)
	require.Empty(t, out)
	require.Nil(t, repo.lastTags)
}
