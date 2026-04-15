package observability

import (
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
)

// NewObservabilityService constructs the IObservabilityService implementation.
func NewObservabilityService(coldIndex storage.IColdIndex) service.IObservabilityService {
	return &observabilityService{coldIndex: coldIndex}
}

type observabilityService struct {
	coldIndex storage.IColdIndex
}

func (s *observabilityService) ApproxColdIndexEntries() int {
	if s.coldIndex == nil {
		return 0
	}
	return s.coldIndex.ApproxEntries()
}

func (s *observabilityService) ColdIndexVectorStats() storage.ColdIndexVectorStats {
	if s.coldIndex == nil {
		return storage.ColdIndexVectorStats{}
	}
	return s.coldIndex.VectorStats()
}

func (s *observabilityService) ColdIndexInvertedStats() storage.ColdIndexInvertedStats {
	if s.coldIndex == nil {
		return storage.ColdIndexInvertedStats{}
	}
	return s.coldIndex.InvertedIndexStats()
}

var _ service.IObservabilityService = (*observabilityService)(nil)
