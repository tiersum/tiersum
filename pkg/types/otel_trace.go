package types

// OtelTraceSummary is one stored trace (aggregated from span rows) for list APIs.
type OtelTraceSummary struct {
	TraceID           string `json:"trace_id"`
	RootSpanName      string `json:"root_span_name,omitempty"`
	StartedAtUnixNano int64  `json:"started_at_unix_nano"`
	EndedAtUnixNano   int64  `json:"ended_at_unix_nano"`
	SpanCount         int    `json:"span_count"`
}

// OtelSpanDTO is one persisted OpenTelemetry span for detail APIs.
type OtelSpanDTO struct {
	TraceID           string `json:"trace_id"`
	SpanID            string `json:"span_id"`
	ParentSpanID      string `json:"parent_span_id,omitempty"`
	Name              string `json:"name"`
	Kind              string `json:"kind"`
	StartTimeUnixNano int64  `json:"start_time_unix_nano"`
	EndTimeUnixNano   int64  `json:"end_time_unix_nano"`
	StatusCode        string `json:"status_code"`
	StatusMessage     string `json:"status_message,omitempty"`
	AttributesJSON    string `json:"attributes_json,omitempty"`
}
