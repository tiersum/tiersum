package coldindex

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/pkg/types"
)

type testEmbedder struct{}

func (testEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	vec := make([]float32, types.ColdEmbeddingVectorDimension)
	vec[0] = 1.0
	return vec, nil
}
func (testEmbedder) Close() error { return nil }

func TestIndex_AddDocument_coldChapters(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	idx, err := NewIndex(logger)
	require.NoError(t, err)
	idx.SetTextEmbedder(testEmbedder{})
	idx.SetColdChapterMaxTokens(512)

	doc := &types.Document{
		ID:      "doc-a",
		Title:   "Book",
		Content: "# Chapter One\n\nHello world.\n",
	}
	err = idx.AddDocument(ctx, doc)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, idx.ApproxEntries(), 1)
}

func Test_mergeResultKeyChapter(t *testing.T) {
	assert.Equal(t, "p1", mergeResultKeyChapter(scoredChapter{DocumentID: "d", Path: "p1"}))
	assert.Equal(t, "d", mergeResultKeyChapter(scoredChapter{DocumentID: "d", Path: ""}))
}

type testStubSplitter struct{}

func (testStubSplitter) Split(docID, docTitle, markdown string, maxTokens int) []ColdChapter {
	_ = docTitle
	_ = markdown
	_ = maxTokens
	return []ColdChapter{{Path: docID + "/stub", Text: "stub-body"}}
}

func TestIndex_SetColdChapterSplitter(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	idx, err := NewIndex(logger)
	require.NoError(t, err)
	idx.SetTextEmbedder(testEmbedder{})
	idx.SetColdChapterSplitter(testStubSplitter{})

	doc := &types.Document{ID: "z1", Title: "T", Content: "# ignored\n"}
	err = idx.AddDocument(ctx, doc)
	require.NoError(t, err)
	require.Contains(t, idx.documents, "z1/stub")
	assert.Equal(t, "stub-body", idx.documents["z1/stub"].Content)
}
