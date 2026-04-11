// Package coldvec provides deterministic cold-document text embeddings used by the cold index (HNSW branch fallback).
// (hash projection fallback when no neural embedder is configured).
package coldvec

import "math"

// SimpleHashEmbedding builds a deterministic L2-normalized bag-of-n-gram vector of length dimension.
// Used as the cold-index fallback when no neural embedder is available.
func SimpleHashEmbedding(content string, dimension int) []float32 {
	if dimension <= 0 {
		return nil
	}
	embedding := make([]float32, dimension)
	for i := 0; i < len(content)-3 && i < 10000; i++ {
		hash := 0
		for j := 0; j < 4 && i+j < len(content); j++ {
			hash = hash*31 + int(content[i+j])
		}
		idx := int(math.Abs(float64(hash))) % dimension
		embedding[idx] += 1.0
	}
	norm := float32(0)
	for _, v := range embedding {
		norm += v * v
	}
	if norm > 0 {
		norm = float32(math.Sqrt(float64(norm)))
		for i := range embedding {
			embedding[i] /= norm
		}
	}
	return embedding
}
