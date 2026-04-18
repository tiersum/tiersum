package types

// ColdIndexInvertedStats summarizes the Bleve BM25 / inverted-text side of the cold chapter index (monitoring).
type ColdIndexInvertedStats struct {
	// BleveDocCount is the number of documents (cold chapters) in the Bleve index.
	BleveDocCount uint64 `json:"bleve_doc_count"`
	// StorageBackend describes how the Bleve index is held (e.g. in-process memory only).
	StorageBackend string `json:"storage_backend"`
	// TextAnalyzer is a short label for the primary text analyzer pipeline.
	TextAnalyzer string `json:"text_analyzer"`
}

// ColdIndexVectorStats summarizes the in-process HNSW vector side of the cold chapter index (monitoring).
type ColdIndexVectorStats struct {
	// HNSWNodes is the number of vectors in the HNSW graph (normally matches indexed cold chapters).
	HNSWNodes int `json:"hnsw_nodes"`
	VectorDim int `json:"vector_dim"`
	HNSWM     int `json:"hnsw_m"`
	// HNSWEfSearch is the HNSW efSearch setting used for vector recall.
	HNSWEfSearch int `json:"hnsw_ef_search"`
	// TextEmbedderConfigured is true when a text embedder was wired at startup (e.g. MiniLM or simple cold embeddings).
	TextEmbedderConfigured bool `json:"text_embedder_configured"`
}
