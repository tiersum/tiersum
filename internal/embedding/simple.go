package embedding

import (
	"context"

	"github.com/tiersum/tiersum/internal/storage/memory"
)

// Simple uses the legacy hash/n-gram projection (no external runtime).
type Simple struct{}

// NewSimple returns an embedder compatible with existing cold-index behavior.
func NewSimple() *Simple {
	return &Simple{}
}

// Embed implements TextEmbedder.
func (Simple) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	return memory.GenerateSimpleEmbedding(text), nil
}

// Close implements TextEmbedder.
func (Simple) Close() error {
	return nil
}
