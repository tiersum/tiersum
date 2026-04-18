package observability

import (
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewObservabilityService exposes read-only monitoring stats for dashboards.
// It is intentionally thin and reads from in-process implementations (e.g. coldindex.Index).
func NewObservabilityService(cold storage.IColdIndex) service.IObservabilityService {
	return &observabilityService{cold: cold}
}

type observabilityService struct {
	cold storage.IColdIndex
}

func (s *observabilityService) ApproxColdIndexEntries() int {
	if s == nil || s.cold == nil {
		return 0
	}
	return s.cold.ApproxEntries()
}

func (s *observabilityService) ColdIndexVectorStats() types.ColdIndexVectorStats {
	if s == nil || s.cold == nil {
		return types.ColdIndexVectorStats{}
	}
	return s.cold.VectorStats()
}

func (s *observabilityService) ColdIndexInvertedStats() types.ColdIndexInvertedStats {
	if s == nil || s.cold == nil {
		return types.ColdIndexInvertedStats{}
	}
	return s.cold.InvertedIndexStats()
}

var _ service.IObservabilityService = (*observabilityService)(nil)

