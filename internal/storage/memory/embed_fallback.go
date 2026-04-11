package memory

import (
	"context"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage/memory/coldvec"
	"github.com/tiersum/tiersum/pkg/types"
)

// FallbackColdTextEmbedding returns a vector of length types.ColdEmbeddingVectorDimension.
// On nil embedder, error, or wrong length, it uses the deterministic hash projection (memory/coldvec).
func FallbackColdTextEmbedding(ctx context.Context, logger *zap.Logger, emb IColdTextEmbedder, text string) []float32 {
	if emb == nil {
		return coldvec.SimpleHashEmbedding(text, types.ColdEmbeddingVectorDimension)
	}
	v, err := emb.Embed(ctx, text)
	if err != nil {
		if logger != nil {
			logger.Warn("text embedding failed, using simple fallback", zap.Error(err))
		}
		return coldvec.SimpleHashEmbedding(text, types.ColdEmbeddingVectorDimension)
	}
	if len(v) != types.ColdEmbeddingVectorDimension {
		if logger != nil {
			logger.Warn("text embedding wrong dimension, using simple fallback",
				zap.Int("got", len(v)), zap.Int("want", types.ColdEmbeddingVectorDimension))
		}
		return coldvec.SimpleHashEmbedding(text, types.ColdEmbeddingVectorDimension)
	}
	return v
}
