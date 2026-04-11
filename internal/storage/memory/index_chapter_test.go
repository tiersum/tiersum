package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/pkg/types"
)

func TestIndex_AddDocument_coldChapters(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	idx, err := NewIndex(logger)
	require.NoError(t, err)
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
	idx.SetColdChapterSplitter(testStubSplitter{})

	doc := &types.Document{ID: "z1", Title: "T", Content: "# ignored\n"}
	err = idx.AddDocument(ctx, doc)
	require.NoError(t, err)
	require.Contains(t, idx.documents, "z1/stub")
	assert.Equal(t, "stub-body", idx.documents["z1/stub"].Content)
}
