package coldindex

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeanPoolL2Norm_singleToken(t *testing.T) {
	batchSize, seqLen, hidden := 1, 2, 4
	mask := []int64{1, 0}
	flat := make([]float32, batchSize*seqLen*hidden)
	for h := 0; h < hidden; h++ {
		flat[h] = float32(h + 1)
	}
	out := meanPoolL2Norm(batchSize, seqLen, hidden, flat, mask)
	require.Len(t, out, 1)
	vec := out[0]
	var norm float64
	for _, v := range vec {
		norm += float64(v * v)
	}
	assert.InDelta(t, 1.0, math.Sqrt(norm), 1e-5)
	expected := []float32{1, 2, 3, 4}
	var s float64
	for _, v := range expected {
		s += float64(v * v)
	}
	inv := float32(1 / math.Sqrt(s))
	for i := range expected {
		assert.InDelta(t, expected[i]*inv, vec[i], 1e-5)
	}
}

func TestMeanPoolL2Norm_twoTokensAverage(t *testing.T) {
	batchSize, seqLen, hidden := 1, 2, 2
	mask := []int64{1, 1}
	flat := make([]float32, batchSize*seqLen*hidden)
	flat[0], flat[1] = 2, 4
	flat[2], flat[3] = 4, 8
	out := meanPoolL2Norm(batchSize, seqLen, hidden, flat, mask)
	vec := out[0]
	assert.InDelta(t, 3.0/float32(math.Sqrt(45)), vec[0], 1e-4)
	assert.InDelta(t, 6.0/float32(math.Sqrt(45)), vec[1], 1e-4)
}
