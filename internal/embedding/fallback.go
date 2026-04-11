package embedding

import (
	"context"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage/memory"
)

// FallbackEmbed returns an embedding of length memory.VectorDimension.
// On nil embedder, error, or wrong length, it falls back to the legacy simple projection.
func FallbackEmbed(ctx context.Context, logger *zap.Logger, emb TextEmbedder, text string) []float32 {
	if emb == nil {
		return memory.GenerateSimpleEmbedding(text)
	}
	v, err := emb.Embed(ctx, text)
	if err != nil {
		if logger != nil {
			logger.Warn("text embedding failed, using simple fallback", zap.Error(err))
		}
		return memory.GenerateSimpleEmbedding(text)
	}
	if len(v) != memory.VectorDimension {
		if logger != nil {
			logger.Warn("text embedding wrong dimension, using simple fallback",
				zap.Int("got", len(v)), zap.Int("want", memory.VectorDimension))
		}
		return memory.GenerateSimpleEmbedding(text)
	}
	return v
}
