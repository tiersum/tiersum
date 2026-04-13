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

func TestTopicSvc_RegroupTags(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
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
	err := svc.RegroupTags(ctx)
	require.NoError(t, err)

	// Verify groups were created
	groups, err := topicRepo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, groups, 2)
}

func TestTopicSvc_RegroupTags_NoTags(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
		provider,
		testLogger(),
	)

	// No tags in the system
	err := svc.RegroupTags(ctx)
	require.NoError(t, err)

	// Verify no groups were created
	groups, err := topicRepo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestTopicSvc_RegroupTags_TagRepoError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
		provider,
		testLogger(),
	)

	// Set error on tag repo
	tagRepo.SetError(errors.New("database error"))

	err := svc.RegroupTags(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestTopicSvc_RegroupTags_LLMError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
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

	err := svc.RegroupTags(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm error")
}

func TestTopicSvc_ShouldRefresh_FirstTime(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
		provider,
		testLogger(),
	)

	// First time - zero time, should refresh
	shouldRefresh, err := svc.ShouldRefresh(ctx)
	require.NoError(t, err)
	assert.True(t, shouldRefresh)
}

func TestTopicSvc_ShouldRefresh_TagCountChanged(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
		provider,
		testLogger(),
	)

	// Create initial tags
	tagRepo.Create(ctx, &types.Tag{ID: "tag1", Name: "golang"})

	// Perform initial grouping to set state
	provider.SetResponse(`[{"name": "Languages", "description": "Programming", "tags": ["golang"]}]`)
	svc.RegroupTags(ctx)

	// Add more tags
	tagRepo.Create(ctx, &types.Tag{ID: "tag2", Name: "python"})

	// Should refresh due to tag count change
	shouldRefresh, err := svc.ShouldRefresh(ctx)
	require.NoError(t, err)
	assert.True(t, shouldRefresh)
}

func TestTopicSvc_ShouldRefresh_TimeElapsed(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := &TopicSvc{
		tagRepo:         tagRepo,
		topicRepo:       topicRepo,
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

func TestTopicSvc_ShouldRefresh_NoRefreshNeeded(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := &TopicSvc{
		tagRepo:         tagRepo,
		topicRepo:       topicRepo,
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

func TestTopicSvc_ListTopics(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
		provider,
		testLogger(),
	)

	// Create groups
	topicRepo.Create(ctx, &types.Topic{
		ID:   "group1",
		Name: "Languages",
	})
	topicRepo.Create(ctx, &types.Topic{
		ID:   "group2",
		Name: "Databases",
	})

	groups, err := svc.ListTopics(ctx)
	require.NoError(t, err)
	assert.Len(t, groups, 2)
}

func TestTopicSvc_performGrouping(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
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
	assert.Equal(t, []string{"golang", "python", "javascript"}, groups[0].TagNames)
}

func TestTopicSvc_performGrouping_EmptyTags(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
		provider,
		testLogger(),
	)

	groups, err := svc.performGrouping(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, groups)
}

func TestTopicSvc_performGrouping_LLMError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
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

func TestTopicSvc_parseGroupResponse(t *testing.T) {
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
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

func TestTopicSvc_RegroupTags_GroupRepoError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
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
	topicRepo.SetError(errors.New("delete error"))

	err := svc.RegroupTags(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete error")
}

func TestTopicSvc_ShouldRefresh_TagRepoError(t *testing.T) {
	ctx := context.Background()
	tagRepo := NewMockTagRepository()
	topicRepo := NewMockTopicRepository()
	provider := NewMockLLMProvider()

	svc := NewTopicSvc(
		tagRepo,
		topicRepo,
		provider,
		testLogger(),
	)

	// Set error on tag repo
	tagRepo.SetError(errors.New("count error"))

	_, err := svc.ShouldRefresh(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count error")
}
