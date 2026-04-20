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

func (b *invertedBleve) indexChapter(path string, ch *ChapterIndex) error {
	payload := *ch
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
	// Fields are not stored in Bleve; metadata is read from idx.documents.
	return b.idx.Search(req)
}

func (b *invertedBleve) close() error {
	return b.idx.Close()
}
