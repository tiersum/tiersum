package telemetry

import (
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type forceRootSampler struct {
	ratio sdktrace.Sampler
}

func newForceRootSampler(ratio float64) sdktrace.Sampler {
	switch {
	case ratio <= 0:
		return &forceRootSampler{ratio: sdktrace.NeverSample()}
	case ratio >= 1:
		return &forceRootSampler{ratio: sdktrace.AlwaysSample()}
	default:
		return &forceRootSampler{ratio: sdktrace.TraceIDRatioBased(ratio)}
	}
}

func (f *forceRootSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	if ForceSampleFromContext(p.ParentContext) {
		return sdktrace.SamplingResult{Decision: sdktrace.RecordAndSample}
	}
	return f.ratio.ShouldSample(p)
}

func (f *forceRootSampler) Description() string {
	return "TierSumForceSampleOrRatio"
}

// NewHTTPSampler returns a parent-based sampler: respects remote parent flags, and for local
// roots applies trace-id ratio sampling unless ForceSampleFromContext(parent) is true.
func NewHTTPSampler(sampleRatio float64) sdktrace.Sampler {
	return sdktrace.ParentBased(newForceRootSampler(sampleRatio))
}
