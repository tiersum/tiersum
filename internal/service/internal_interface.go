// Package service defines internal service-layer contracts.
//
// These interfaces are used to compose service implementations and are not intended
// to be consumed by upper layers (API/Job). Keep facade interfaces in interface.go.
package service

import (
	"context"

	"github.com/tiersum/tiersum/pkg/types"
)

// IDocumentAnalysisPersister persists document analysis outputs (document summary + chapter rows).
// It is not a search indexer; cold search lives in storage.IColdIndex.
type IDocumentAnalysisPersister interface {
	// PersistAnalysis writes analysis-derived fields onto the document row and its chapters table.
	PersistAnalysis(ctx context.Context, doc *types.Document, analysis *types.DocumentAnalysisResult) error
}

// IDocumentAnalysisGenerator performs one LLM-backed analysis call and parses the JSON result (no hidden repair passes;
// parsed summary/tags/chapters are not post-truncated or tag-normalized—constraints belong in the prompt).
type IDocumentAnalysisGenerator interface {
	GenerateAnalysis(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error)
}
