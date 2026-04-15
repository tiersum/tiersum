// Package service defines internal service-layer contracts.
//
// These interfaces are used to compose service implementations (svcimpl) and are not intended
// to be consumed by upper layers (API/Job). Keep facade interfaces in interface.go.
package service

import (
	"context"

	"github.com/tiersum/tiersum/pkg/types"
)

// IChapterMaterializer persists document analysis outputs (document summary + chapter rows).
// It is not a search indexer; cold search lives in storage.IColdIndex.
type IChapterMaterializer interface {
	// Materialize persists analysis-derived fields onto the document row and its chapters table.
	Materialize(ctx context.Context, doc *types.Document, analysis *types.DocumentAnalysisResult) error
}

// IDocumentAnalyzer performs LLM-backed analysis to produce summaries/tags/chapters.
type IDocumentAnalyzer interface {
	AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error)
}

// IRelevanceFilter performs LLM-backed relevance filtering for progressive query.
type IRelevanceFilter interface {
	FilterDocuments(ctx context.Context, query string, docs []types.Document) ([]types.LLMFilterResult, error)
	FilterChapters(ctx context.Context, query string, chapters []types.Chapter) ([]types.LLMFilterResult, error)
}

