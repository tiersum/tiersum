package coldindex

import (
	"fmt"

	"github.com/coder/hnsw"
)

// vectorHNSW is the dense-vector (cosine) side of the cold chapter index.
type vectorHNSW struct {
	g   *hnsw.Graph[string]
	dim int
}

func newVectorHNSW(g *hnsw.Graph[string], dim int) *vectorHNSW {
	return &vectorHNSW{g: g, dim: dim}
}

func (v *vectorHNSW) add(path string, vec []float32) {
	if len(vec) == v.dim {
		v.g.Add(hnsw.MakeNode(path, vec))
	}
}

func (v *vectorHNSW) delete(path string) {
	v.g.Delete(path)
}

func (v *vectorHNSW) search(query []float32, topK int) ([]hnsw.Node[string], error) {
	if len(query) != v.dim {
		return nil, fmt.Errorf("invalid embedding dimension: expected %d, got %d", v.dim, len(query))
	}
	return v.g.Search(query, topK), nil
}
