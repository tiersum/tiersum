package observability

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/viper"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewTraceService constructs a service.ITraceService backed by IOtelSpanRepository.
func NewTraceService(otel storage.IOtelSpanRepository) service.ITraceService {
	return &traceService{otel: otel}
}

type traceService struct {
	otel storage.IOtelSpanRepository
}

func (s *traceService) ListTraceSummaries(ctx context.Context, limit, offset int) ([]types.OtelTraceSummary, error) {
	if s.otel == nil {
		return []types.OtelTraceSummary{}, nil
	}
	svc := strings.TrimSpace(viper.GetString("telemetry.service_name"))
	if svc == "" {
		svc = "tiersum"
	}
	rows, err := s.otel.ListTraceSummaries(ctx, svc, limit, offset)
	if rows == nil {
		rows = []types.OtelTraceSummary{}
	}
	return rows, err
}

func (s *traceService) ListSpansByTraceID(ctx context.Context, traceID string) ([]types.OtelSpanDTO, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, errors.New("trace_id required")
	}
	if s.otel == nil {
		return nil, errors.New("trace store not configured")
	}
	return s.otel.ListSpansByTraceID(ctx, traceID)
}

var _ service.ITraceService = (*traceService)(nil)

