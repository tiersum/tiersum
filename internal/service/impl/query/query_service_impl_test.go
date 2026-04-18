package query

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

type fakeDocRepo struct {
	mu        sync.Mutex
	incrCalls []string
	incrCh    chan string
}

func (f *fakeDocRepo) Create(ctx context.Context, doc *types.Document) error { return nil }
func (f *fakeDocRepo) GetByID(ctx context.Context, id string) (*types.Document, error) {
	return nil, nil
}
func (f *fakeDocRepo) GetRecent(ctx context.Context, limit int) ([]*types.Document, error) {
	return nil, nil
}
func (f *fakeDocRepo) ListByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error) {
	return nil, nil
}
func (f *fakeDocRepo) ListMetaByTagsAndStatuses(ctx context.Context, tags []string, statuses []types.DocumentStatus, limit int) ([]types.Document, error) {
	return nil, nil
}
func (f *fakeDocRepo) ListByStatus(ctx context.Context, status types.DocumentStatus, limit int) ([]types.Document, error) {
	return nil, nil
}
func (f *fakeDocRepo) IncrementQueryCount(ctx context.Context, docID string) error {
	f.mu.Lock()
	f.incrCalls = append(f.incrCalls, docID)
	f.mu.Unlock()
	if f.incrCh != nil {
		select {
		case f.incrCh <- docID:
		default:
		}
	}
	return nil
}
func (f *fakeDocRepo) UpdateStatus(ctx context.Context, docID string, status types.DocumentStatus) error {
	return nil
}
func (f *fakeDocRepo) UpdateHotScore(ctx context.Context, docID string, score float64) error {
	return nil
}
func (f *fakeDocRepo) UpdateTags(ctx context.Context, docID string, tags []string) error { return nil }
func (f *fakeDocRepo) UpdateSummary(ctx context.Context, docID string, summary string) error {
	return nil
}
func (f *fakeDocRepo) ListAll(ctx context.Context, limit int) ([]types.Document, error) {
	return nil, nil
}
func (f *fakeDocRepo) CountDocumentsByStatus(ctx context.Context) (types.DocumentStatusCounts, error) {
	return types.DocumentStatusCounts{}, nil
}

var _ storage.IDocumentRepository = (*fakeDocRepo)(nil)

type fakeChapterSvc struct {
	hotHits  []types.HotSearchHit
	coldHits []types.ColdSearchHit
	hotErr   error
	coldErr  error
}

func (f *fakeChapterSvc) ListChaptersByDocumentID(ctx context.Context, documentID string) ([]types.Chapter, error) {
	return nil, nil
}
func (f *fakeChapterSvc) ExtractChaptersFromMarkdown(ctx context.Context, doc *types.Document) ([]types.Chapter, error) {
	return nil, nil
}
func (f *fakeChapterSvc) ListChaptersByDocumentIDs(ctx context.Context, docIDs []string) (map[string][]types.Chapter, error) {
	return map[string][]types.Chapter{}, nil
}
func (f *fakeChapterSvc) SearchColdChapterHits(ctx context.Context, query string, limit int) ([]types.ColdSearchHit, error) {
	return f.coldHits, f.coldErr
}
func (f *fakeChapterSvc) SearchHotChapters(ctx context.Context, query string, limit int) ([]types.HotSearchHit, error) {
	return f.hotHits, f.hotErr
}

var (
	_ service.IChapterHybridSearch = (*fakeChapterSvc)(nil)
	_ service.IChapterService      = (*fakeChapterSvc)(nil)
)

type fakeLLM struct {
	out string
	err error
}

func (f *fakeLLM) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	_ = ctx
	_ = prompt
	_ = maxTokens
	return f.out, f.err
}

var _ client.ILLMProvider = (*fakeLLM)(nil)

func TestProgressiveQuery_MergeAndDedupe(t *testing.T) {
	docRepo := &fakeDocRepo{}
	chapterSvc := &fakeChapterSvc{
		hotHits: []types.HotSearchHit{
			{DocumentID: "d1", Path: "d1/p1", Title: "T1", Content: "H1", Score: 0.60, Status: types.DocStatusHot, Source: "hot", QueryCount: 0},
			{DocumentID: "d1", Path: "d1/p2", Title: "T2", Content: "H2", Score: 0.80, Status: types.DocStatusHot, Source: "hot", QueryCount: 0},
		},
		coldHits: []types.ColdSearchHit{
			{DocumentID: "d1", Path: "d1/p1", Title: "T1", Content: "C1", Score: 0.95, Source: "cold"},
			{DocumentID: "d2", Path: "d2/p9", Title: "T9", Content: "C9", Score: 0.70, Source: "cold"},
		},
	}
	svc := NewQueryService(docRepo, chapterSvc, nil, zap.NewNop())

	resp, err := svc.ProgressiveQuery(context.Background(), types.ProgressiveQueryRequest{Question: "q", MaxResults: 10})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "q", resp.Question)

	// Dedupe by (docID,path): d1/p1 should keep the higher relevance (from cold 0.95, not hot 0.60).
	require.Len(t, resp.Results, 3)
	require.Equal(t, "d1", resp.Results[0].ID)
	require.Equal(t, "d1/p1", resp.Results[0].Path)
	require.InEpsilon(t, 0.95, resp.Results[0].Relevance, 1e-9)

	// Sorted by relevance descending.
	require.GreaterOrEqual(t, resp.Results[0].Relevance, resp.Results[1].Relevance)
	require.GreaterOrEqual(t, resp.Results[1].Relevance, resp.Results[2].Relevance)

	// Steps should include both paths.
	require.NotEmpty(t, resp.Steps)
}

func TestProgressiveQuery_ColdIndexUnavailable_IsNotFatal(t *testing.T) {
	docRepo := &fakeDocRepo{}
	chapterSvc := &fakeChapterSvc{
		hotHits: []types.HotSearchHit{
			{DocumentID: "d1", Path: "d1/p1", Title: "T1", Content: "H1", Score: 0.60, Status: types.DocStatusHot, Source: "hot", QueryCount: 0},
		},
		coldErr: service.ErrColdIndexUnavailable,
	}
	svc := NewQueryService(docRepo, chapterSvc, nil, zap.NewNop())

	resp, err := svc.ProgressiveQuery(context.Background(), types.ProgressiveQueryRequest{Question: "q", MaxResults: 10})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
}

func TestProgressiveQuery_GeneratesAnswerWhenLLMConfigured(t *testing.T) {
	docRepo := &fakeDocRepo{}
	chapterSvc := &fakeChapterSvc{
		hotHits: []types.HotSearchHit{
			{DocumentID: "d1", Path: "d1/p1", Title: "T1", Content: "H1", Score: 0.60, Status: types.DocStatusHot, Source: "hot", QueryCount: 0},
		},
	}
	llm := &fakeLLM{out: "answer"}
	svc := NewQueryService(docRepo, chapterSvc, llm, zap.NewNop())

	resp, err := svc.ProgressiveQuery(context.Background(), types.ProgressiveQueryRequest{Question: "q", MaxResults: 10})
	require.NoError(t, err)
	require.Equal(t, "answer", resp.Answer)
}

func TestProgressiveQuery_TracksDocAccessAndQueuesPromotion(t *testing.T) {
	// Replace global queue for test isolation.
	orig := job.PromoteQueue
	defer func() { job.PromoteQueue = orig }()
	job.PromoteQueue = make(chan string, 10)

	incrCh := make(chan string, 10)
	docRepo := &fakeDocRepo{incrCh: incrCh}
	chapterSvc := &fakeChapterSvc{
		hotHits: []types.HotSearchHit{
			// Even though hot-path normally returns hot/warming docs, the query service treats hits as authoritative.
			{DocumentID: "cold1", Path: "cold1/full", Title: "T", Content: "H", Score: 0.9, Status: types.DocStatusCold, Source: "hot", QueryCount: 2},
		},
	}
	svc := NewQueryService(docRepo, chapterSvc, nil, zap.NewNop())

	_, err := svc.ProgressiveQuery(context.Background(), types.ProgressiveQueryRequest{Question: "q", MaxResults: 10})
	require.NoError(t, err)

	// Wait for IncrementQueryCount call.
	select {
	case got := <-incrCh:
		require.Equal(t, "cold1", got)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for IncrementQueryCount")
	}

	// Promotion should be queued at default threshold (3): queryCount 2 + 1 triggers.
	select {
	case got := <-job.PromoteQueue:
		require.Equal(t, "cold1", got)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for promotion enqueue")
	}
}

func TestProgressiveQuery_AnswerGenerationFailureIsSoft(t *testing.T) {
	docRepo := &fakeDocRepo{}
	chapterSvc := &fakeChapterSvc{
		hotHits: []types.HotSearchHit{
			{DocumentID: "d1", Path: "d1/p1", Title: "T1", Content: "H1", Score: 0.60, Status: types.DocStatusHot, Source: "hot", QueryCount: 0},
		},
	}
	llm := &fakeLLM{out: "", err: errors.New("boom")}
	svc := NewQueryService(docRepo, chapterSvc, llm, zap.NewNop())

	resp, err := svc.ProgressiveQuery(context.Background(), types.ProgressiveQueryRequest{Question: "q", MaxResults: 10})
	require.NoError(t, err)
	require.Equal(t, "", resp.Answer)
}
