// Package metrics provides Prometheus metrics collection for TierSum
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// LLMMetrics tracks LLM calls and token counts
	LLMCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tiersum_llm_calls_total",
			Help: "Total number of LLM calls",
		},
		[]string{"path"},
	)

	LLMInputTokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tiersum_llm_input_tokens_total",
			Help: "Total number of input tokens sent to LLM",
		},
		[]string{"path"},
	)

	LLMOutputTokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tiersum_llm_output_tokens_total",
			Help: "Total number of output tokens received from LLM",
		},
		[]string{"path"},
	)

	// QueryMetrics tracks query latency
	QueryDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "tiersum_query_duration_seconds",
			Help: "Query latency in seconds",
			Buckets: []float64{
				0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5,
				1, 2.5, 5, 10, 30, 60, 120, 300,
			},
		},
		[]string{"path"},
	)

	QueryResultsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tiersum_query_results_total",
			Help: "Total number of results returned by queries",
		},
		[]string{"path"},
	)

	// DocumentTierMetrics tracks document status
	DocumentsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tiersum_documents_total",
			Help: "Total number of documents by status",
		},
		[]string{"status"},
	)

	DocumentPromotionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tiersum_document_promotions_total",
			Help: "Total number of document promotions",
		},
		[]string{"from", "to"},
	)

	// SystemMetrics tracks system health
	JobExecutionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tiersum_job_executions_total",
			Help: "Total number of job executions",
		},
		[]string{"job_name", "status"},
	)

	JobDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "tiersum_job_duration_seconds",
			Help: "Job execution duration in seconds",
			Buckets: []float64{
				0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5,
				1, 2.5, 5, 10, 30, 60, 120, 300,
			},
		},
		[]string{"job_name"},
	)
)

func init() {
	// Register all metrics
	prometheus.MustRegister(LLMCallsTotal)
	prometheus.MustRegister(LLMInputTokensTotal)
	prometheus.MustRegister(LLMOutputTokensTotal)
	prometheus.MustRegister(QueryDurationSeconds)
	prometheus.MustRegister(QueryResultsTotal)
	prometheus.MustRegister(DocumentsTotal)
	prometheus.MustRegister(DocumentPromotionsTotal)
	prometheus.MustRegister(JobExecutionsTotal)
	prometheus.MustRegister(JobDurationSeconds)
}

// Path constants for LLM calls
const (
	PathTagFilter     = "tag_filter"
	PathTopicFilter   = "topic_filter"
	PathDocFilter     = "doc_filter"
	PathChapterFilter = "chapter_filter"
	PathDocAnalyze    = "doc_analyze"
	PathTopicRegroup  = "topic_regroup"
	PathUnknown       = "unknown"
)

// Path constants for queries
const (
	QueryPathHot     = "hot"
	QueryPathCold    = "cold"
	PathAnswerGen    = "answer_gen"
)

// RecordLLMTokens records an LLM call with input and output token counts
func RecordLLMTokens(path string, inputTokens, outputTokens int) {
	LLMCallsTotal.WithLabelValues(path).Inc()
	LLMInputTokensTotal.WithLabelValues(path).Add(float64(inputTokens))
	LLMOutputTokensTotal.WithLabelValues(path).Add(float64(outputTokens))
}

// RecordQueryLatency records query latency
func RecordQueryLatency(path string, seconds float64, results int) {
	QueryDurationSeconds.WithLabelValues(path).Observe(seconds)
	if results > 0 {
		QueryResultsTotal.WithLabelValues(path).Add(float64(results))
	}
}

// UpdateDocumentCount updates document count gauge
func UpdateDocumentCount(status string, count int) {
	DocumentsTotal.WithLabelValues(status).Set(float64(count))
}

// RecordDocumentPromotion records a document promotion
func RecordDocumentPromotion(from, to string) {
	DocumentPromotionsTotal.WithLabelValues(from, to).Inc()
}

// RecordJobExecution records job execution
func RecordJobExecution(jobName string, success bool, durationSeconds float64) {
	status := "success"
	if !success {
		status = "failure"
	}
	JobExecutionsTotal.WithLabelValues(jobName, status).Inc()
	JobDurationSeconds.WithLabelValues(jobName).Observe(durationSeconds)
}
