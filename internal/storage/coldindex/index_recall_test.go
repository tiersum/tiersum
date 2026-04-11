package coldindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestIndex_branchRecallSize_respectsSetColdSearchRecall(t *testing.T) {
	idx, err := NewIndex(zap.NewNop())
	require.NoError(t, err)
	idx.SetColdSearchRecall(3, 50, 150)
	assert.Equal(t, 50, idx.branchRecallSize(1))
	assert.Equal(t, 150, idx.branchRecallSize(100))
	assert.Equal(t, 90, idx.branchRecallSize(30))
}
