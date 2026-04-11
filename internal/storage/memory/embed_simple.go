package memory

import (
	"context"

	"github.com/tiersum/tiersum/internal/storage/memory/coldvec"
	"github.com/tiersum/tiersum/pkg/types"
)

// Simple uses the legacy hash/n-gram projection (no external runtime).
type Simple struct{}

// NewSimple returns an embedder compatible with existing cold-index behavior.
func NewSimple() *Simple {
	return &Simple{}
}

// Embed implements IColdTextEmbedder.
func (Simple) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	return coldvec.SimpleHashEmbedding(text, types.ColdEmbeddingVectorDimension), nil
}

// Close implements IColdTextEmbedder.
func (Simple) Close() error {
	return nil
}

var _ IColdTextEmbedder = (*Simple)(nil)
