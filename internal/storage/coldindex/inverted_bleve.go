package coldindex

import (
	"github.com/blevesearch/bleve/v2"
)

// invertedBleve is the BM25 / full-text side of the cold chapter index.
type invertedBleve struct {
	idx bleve.Index
}

func newInvertedBleve(idx bleve.Index) *invertedBleve {
	return &invertedBleve{idx: idx}
}

func (b *invertedBleve) indexChapter(path string, doc *DocumentIndex) error {
	payload := *doc
	payload.Embedding = nil
	return b.idx.Index(path, &payload)
}

func (b *invertedBleve) deleteChapter(path string) error {
	return b.idx.Delete(path)
}

func (b *invertedBleve) search(queryText string, topK int) (*bleve.SearchResult, error) {
	q := bleve.NewQueryStringQuery(queryText)
	req := bleve.NewSearchRequest(q)
	req.Size = topK
	req.Fields = []string{"*"}
	return b.idx.Search(req)
}

func (b *invertedBleve) close() error {
	return b.idx.Close()
}
