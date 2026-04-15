package document

import (
	"context"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/service/svcimpl/common"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewDocumentAnalyzer constructs the service.IDocumentAnalyzer implementation.
func NewDocumentAnalyzer(provider client.ILLMProvider, logger *zap.Logger) service.IDocumentAnalyzer {
	return &documentAnalyzer{core: common.NewSummarizerCore(provider, logger)}
}

type documentAnalyzer struct {
	core *common.SummarizerCore
}

func (s *documentAnalyzer) AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error) {
	return s.core.AnalyzeDocument(ctx, title, content)
}

var _ service.IDocumentAnalyzer = (*documentAnalyzer)(nil)
