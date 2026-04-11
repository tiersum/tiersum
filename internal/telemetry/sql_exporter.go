// Package telemetry wires OpenTelemetry export to TierSum storage.
package telemetry

import (
	"context"
	"encoding/json"
	"fmt"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/tiersum/tiersum/internal/storage"
)

// SQLSpanExporter persists completed spans to IOtelSpanRepository (OpenTelemetry SDK SpanExporter).
type SQLSpanExporter struct {
	repo storage.IOtelSpanRepository
}

// NewSQLSpanExporter builds an exporter backed by the application database.
func NewSQLSpanExporter(repo storage.IOtelSpanRepository) sdktrace.SpanExporter {
	return &SQLSpanExporter{repo: repo}
}

// ExportSpans implements sdktrace.SpanExporter.
func (e *SQLSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if e.repo == nil {
		return nil
	}
	for i := range spans {
		if err := e.exportOne(ctx, spans[i]); err != nil {
			return err
		}
	}
	return nil
}

func (e *SQLSpanExporter) exportOne(ctx context.Context, ro sdktrace.ReadOnlySpan) error {
	sc := ro.SpanContext()
	if !sc.IsValid() {
		return nil
	}
	traceID := sc.TraceID().String()
	spanID := sc.SpanID().String()
	parent := parentSpanIDFromReadOnly(ro)
	attrs := ro.Attributes()
	mp := make(map[string]any, len(attrs))
	for _, kv := range attrs {
		mp[string(kv.Key)] = kv.Value.AsInterface()
	}
	attrJSON, err := json.Marshal(mp)
	if err != nil {
		return fmt.Errorf("marshal span attributes: %w", err)
	}
	st := ro.Status()
	row := &storage.OtelSpanRow{
		TraceID:        traceID,
		SpanID:         spanID,
		ParentSpanID:   parent,
		Name:           ro.Name(),
		Kind:           ro.SpanKind().String(),
		StartUnixNano:  ro.StartTime().UnixNano(),
		EndUnixNano:    ro.EndTime().UnixNano(),
		StatusCode:     st.Code.String(),
		StatusMessage:  st.Description,
		AttributesJSON: string(attrJSON),
	}
	return e.repo.InsertSpan(ctx, row)
}

func parentSpanIDFromReadOnly(ro sdktrace.ReadOnlySpan) string {
	type withParent interface {
		Parent() trace.SpanContext
	}
	p, ok := interface{}(ro).(withParent)
	if !ok {
		return ""
	}
	pc := p.Parent()
	if !pc.IsValid() || !pc.HasSpanID() {
		return ""
	}
	return pc.SpanID().String()
}

// Shutdown implements sdktrace.SpanExporter.
func (e *SQLSpanExporter) Shutdown(context.Context) error {
	return nil
}

var _ sdktrace.SpanExporter = (*SQLSpanExporter)(nil)
