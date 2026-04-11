package telemetry

import "context"

type forceSampleKeyType struct{}

// ContextWithForceSample marks the context so the HTTP root span sampler records this trace.
func ContextWithForceSample(ctx context.Context, force bool) context.Context {
	return context.WithValue(ctx, forceSampleKeyType{}, force)
}

// ForceSampleFromContext returns whether the context was marked for forced sampling.
func ForceSampleFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(forceSampleKeyType{}).(bool)
	return v
}
