// Package embedding provides text vectorization for cold-document hybrid search.
package embedding

import "context"

// TextEmbedder turns text into a dense vector for HNSW (dimension must match memory.VectorDimension).
type TextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Close() error
}
