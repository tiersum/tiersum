package types

// ColdEmbeddingVectorDimension is the fixed vector size for cold-document HNSW and text embedders.
const ColdEmbeddingVectorDimension = 384

// DefaultColdChapterMaxTokens is the default budget per cold markdown chapter (split), in the same
// units as coldindex.EstimateTokens. It targets the cold text embedder's max sequence length (e.g.
// ~512 subword tokens for MiniLM)—vector/HNSW indexing, not LLM prompt sizing. CJK-heavy runes
// count ~1 unit each; Latin-heavy text uses ~4 runes per unit.
const DefaultColdChapterMaxTokens = 512
