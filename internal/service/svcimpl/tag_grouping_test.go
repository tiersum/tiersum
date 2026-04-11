package svcimpl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/pkg/types"
)

func TestTagGroupSvc_GroupTags(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	// Create some tags
	tags := []string{"golang", "python", "javascript", "postgres", "mysql", "redis"}
	for _, name := range tags {
		err := tagRepo.Create(ctx, &types.Tag{
			ID:   "tag-" + name,
			Name: name,
		})
		require.NoError(t, err)
	}

	// Set up mock LLM response
	provider.SetResponse(`[
		{"name": "Programming Languages", "description": "Programming languages", "tags": ["golang", "python", "javascript"]},
		{"name": "Databases", "description": "Database systems", "tags": ["postgres", "mysql", "redis"]}
	]`)

	// Test grouping
	err := svc.GroupTags(ctx)
	require.NoError(t, err)

	// Verify groups were created
	groups, err := groupRepo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, groups, 2)
}

func TestTagGroupSvc_GroupTags_NoTags(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	// No tags in the system
	err := svc.GroupTags(ctx)
	require.NoError(t, err)

	// Verify no groups were created
	groups, err := groupRepo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestTagGroupSvc_GroupTags_TagRepoError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	// Set error on tag repo
	tagRepo.SetError(errors.New("database error"))

	err := svc.GroupTags(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestTagGroupSvc_GroupTags_LLMError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	// Create a tag
	tagRepo.Create(ctx, &types.Tag{
		ID:   "tag1",
		Name: "golang",
	})

	// Set error on provider
	provider.SetError(errors.New("llm error"))

	err := svc.GroupTags(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm error")
}

func TestTagGroupSvc_ShouldRefresh_FirstTime(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	// First time - zero time, should refresh
	shouldRefresh, err := svc.ShouldRefresh(ctx)
	require.NoError(t, err)
	assert.True(t, shouldRefresh)
}

func TestTagGroupSvc_ShouldRefresh_TagCountChanged(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	// Create initial tags
	tagRepo.Create(ctx, &types.Tag{ID: "tag1", Name: "golang"})

	// Perform initial grouping to set state
	provider.SetResponse(`[{"name": "Languages", "description": "Programming", "tags": ["golang"]}]`)
	svc.GroupTags(ctx)

	// Add more tags
	tagRepo.Create(ctx, &types.Tag{ID: "tag2", Name: "python"})

	// Should refresh due to tag count change
	shouldRefresh, err := svc.ShouldRefresh(ctx)
	require.NoError(t, err)
	assert.True(t, shouldRefresh)
}

func TestTagGroupSvc_ShouldRefresh_TimeElapsed(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := &TagGroupSvc{
		tagRepo:         tagRepo,
		groupRepo:       groupRepo,
		provider:        provider,
		logger:          testLogger(),
		lastRefreshTime: time.Now().Add(-31 * time.Minute), // 31 minutes ago
		lastTagCount:    0,
	}

	// Should refresh due to time elapsed
	shouldRefresh, err := svc.ShouldRefresh(ctx)
	require.NoError(t, err)
	assert.True(t, shouldRefresh)
}

func TestTagGroupSvc_ShouldRefresh_NoRefreshNeeded(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := &TagGroupSvc{
		tagRepo:         tagRepo,
		groupRepo:       groupRepo,
		provider:        provider,
		logger:          testLogger(),
		lastRefreshTime: time.Now().Add(-5 * time.Minute), // 5 minutes ago
		lastTagCount:    0,
	}

	// Should not refresh - too soon
	shouldRefresh, err := svc.ShouldRefresh(ctx)
	require.NoError(t, err)
	assert.False(t, shouldRefresh)
}

func TestTagGroupSvc_GetL1Groups(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	// Create groups
	groupRepo.Create(ctx, &types.TagGroup{
		ID:   "group1",
		Name: "Languages",
	})
	groupRepo.Create(ctx, &types.TagGroup{
		ID:   "group2",
		Name: "Databases",
	})

	groups, err := svc.GetL1Groups(ctx)
	require.NoError(t, err)
	assert.Len(t, groups, 2)
}

func TestTagGroupSvc_GetL2TagsByGroup(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	// Create tags in a group
	tagRepo.Create(ctx, &types.Tag{
		ID:      "tag1",
		Name:    "golang",
		GroupID: "group1",
	})
	tagRepo.Create(ctx, &types.Tag{
		ID:      "tag2",
		Name:    "python",
		GroupID: "group1",
	})
	tagRepo.Create(ctx, &types.Tag{
		ID:      "tag3",
		Name:    "postgres",
		GroupID: "group2",
	})

	tags, err := svc.GetL2TagsByGroup(ctx, "group1")
	require.NoError(t, err)
	assert.Len(t, tags, 2)
}

func TestTagGroupSvc_FilterL2TagsByQuery(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	tags := []types.Tag{
		{ID: "tag1", Name: "golang", DocumentCount: 10},
		{ID: "tag2", Name: "python", DocumentCount: 5},
	}

	// Set up mock LLM response
	provider.SetResponse(`[
		{"tag": "golang", "relevance": 0.95},
		{"tag": "python", "relevance": 0.7}
	]`)

	results, err := svc.FilterL2TagsByQuery(ctx, "programming languages", tags)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "golang", results[0].Tag)
	assert.Equal(t, 0.95, results[0].Relevance)
}

func TestTagGroupSvc_FilterL2TagsByQuery_EmptyTags(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	results, err := svc.FilterL2TagsByQuery(ctx, "query", nil)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestTagGroupSvc_FilterL2TagsByQuery_LLMError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	tags := []types.Tag{
		{ID: "tag1", Name: "golang"},
	}

	// Set error on provider
	provider.SetError(errors.New("llm error"))

	// Should fallback to equal relevance
	results, err := svc.FilterL2TagsByQuery(ctx, "query", tags)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "golang", results[0].Tag)
	assert.Equal(t, 0.5, results[0].Relevance)
}

func TestTagGroupSvc_performGrouping(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	tags := []string{"golang", "python", "javascript"}

	// Set up mock LLM response
	provider.SetResponse(`[
		{"name": "Languages", "description": "Programming languages", "tags": ["golang", "python", "javascript"]}
	]`)

	groups, err := svc.performGrouping(ctx, tags)
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "Languages", groups[0].Name)
	assert.Equal(t, []string{"golang", "python", "javascript"}, groups[0].Tags)
}

func TestTagGroupSvc_performGrouping_EmptyTags(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	groups, err := svc.performGrouping(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, groups)
}

func TestTagGroupSvc_performGrouping_LLMError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	tags := []string{"golang"}

	// Set error on provider
	provider.SetError(errors.New("llm error"))

	_, err := svc.performGrouping(ctx, tags)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm error")
}

func TestTagGroupSvc_parseGroupResponse(t *testing.T) {
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	tests := []struct {
		name     string
		response string
		wantErr  bool
		wantLen  int
	}{
		{
			name:     "valid response",
			response: `[{"name": "Languages", "description": "Programming", "tags": ["go", "py"]}]`,
			wantErr:  false,
			wantLen:  1,
		},
		{
			name:     "response with markdown",
			response: "```json\n[{\"name\": \"Languages\", \"description\": \"Programming\", \"tags\": [\"go\"]}]\n```",
			wantErr:  false,
			wantLen:  1,
		},
		{
			name:     "invalid json",
			response: "not json",
			wantErr:  true,
			wantLen:  0,
		},
		{
			name:     "no json array",
			response: "{\"name\": \"test\"}",
			wantErr:  true,
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups, err := svc.parseGroupResponse(tt.response)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, groups, tt.wantLen)
			}
		})
	}
}

func TestTagGroupSvc_parseTagFilterResults(t *testing.T) {
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	tests := []struct {
		name     string
		response string
		wantLen  int
		firstTag string
	}{
		{
			name:     "valid response",
			response: `[{"tag": "golang", "relevance": 0.9}, {"tag": "python", "relevance": 0.7}]`,
			wantLen:  2,
			firstTag: "golang",
		},
		{
			name:     "sorted by relevance",
			response: `[{"tag": "python", "relevance": 0.7}, {"tag": "golang", "relevance": 0.9}]`,
			wantLen:  2,
			firstTag: "golang",
		},
		{
			name:     "invalid json",
			response: "not json",
			wantLen:  0,
		},
		{
			name:     "no json array",
			response: "{}",
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := svc.parseTagFilterResults(tt.response)
			assert.Len(t, results, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.firstTag, results[0].Tag)
			}
		})
	}
}

func TestTagGroupSvc_fallbackTagFilter(t *testing.T) {
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	tags := []types.Tag{
		{ID: "tag1", Name: "golang"},
		{ID: "tag2", Name: "python"},
	}

	results := svc.fallbackTagFilter(tags)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, 0.5, r.Relevance)
	}
}

func TestTagGroupSvc_GroupTags_GroupRepoError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	// Create a tag
	tagRepo.Create(ctx, &types.Tag{
		ID:   "tag1",
		Name: "golang",
	})

	// Set up valid LLM response
	provider.SetResponse(`[{"name": "Languages", "description": "Programming", "tags": ["golang"]}]`)

	// Set error on group repo for DeleteAll
	groupRepo.SetError(errors.New("delete error"))

	err := svc.GroupTags(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete error")
}

func TestTagGroupSvc_ShouldRefresh_TagRepoError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	provider := NewMockLLMProvider()

	svc := NewTagGroupSvc(
		tagRepo,
		groupRepo,
		provider,
		testLogger(),
	)

	// Set error on tag repo
	tagRepo.SetError(errors.New("count error"))

	_, err := svc.ShouldRefresh(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count error")
}
