package coldindex

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/pkg/types"
)

func TestSimple_Embed_dimensionAndNormalized(t *testing.T) {
	s := NewSimple()
	vec, err := s.Embed(context.Background(), "hello world 测试")
	require.NoError(t, err)
	require.Len(t, vec, types.ColdEmbeddingVectorDimension)
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	assert.InDelta(t, 1.0, sum, 1e-5, "simple embedding should be L2-normalized")
	require.NoError(t, s.Close())
}
