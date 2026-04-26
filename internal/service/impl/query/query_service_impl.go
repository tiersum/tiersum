package query

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/config"
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/metrics"
	"github.com/tiersum/tiersum/pkg/types"
)

func init() {
	viper.SetDefault("query.allow_progressive_debug", true)
}

const (
	// TraceMaxReqBytes caps prompt / question bytes stored on spans.
	TraceMaxReqBytes = 4096
	// TraceMaxRespBytes caps completion bytes stored on spans.
	TraceMaxRespBytes = 4096

	// ProgressiveTracerScope is the OpenTelemetry tracer name used for progressive-query debug trees.
	ProgressiveTracerScope = "github.com/tiersum/tiersum/progressive_query"

	// progressiveAnswerExcerptBytesCeiling caps per-reference excerpt size in the answer prompt (bytes).
	progressiveAnswerExcerptBytesCeiling = 6000
	// progressiveAnswerReferencesCeiling caps how many references are passed to the answer LLM.
	progressiveAnswerReferencesCeiling = 30

	defaultProgressiveAnswerMaxReferences = 12
	defaultProgressiveAnswerExcerptBytes  = 3500
	defaultProgressiveAnswerOutTokens     = 4096
)

// Progressive-query span attributes use a consistent prefix:
//   - tier.request.*  inputs (question, limits, prompts)
//   - tier.response.* outputs (counts, hits, model text, flags)

type debugTracerKeyType struct{}

var debugTracerKey = debugTracerKeyType{}

// WithProgressiveDebugTracer attaches a request-local OpenTelemetry tracer for progressive-query debug recording.
func WithProgressiveDebugTracer(ctx context.Context, t trace.Tracer) context.Context {
	if t == nil {
		return ctx
	}
	return context.WithValue(ctx, debugTracerKey, t)
}

// ProgressiveDebugTracerFrom returns the tracer installed by WithProgressiveDebugTracer, or nil.
func ProgressiveDebugTracerFrom(ctx context.Context) trace.Tracer {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(debugTracerKey).(trace.Tracer)
	return v
}

// ProgressiveTraceRequested is true when the server allows detailed progressive spans and the
// request is part of a sampled trace (parent span in ctx is recording), per OpenTelemetry practice.
func ProgressiveTraceRequested(ctx context.Context) bool {
	if !viper.GetBool("query.allow_progressive_debug") {
		return false
	}
	s := trace.SpanFromContext(ctx)
	return s.SpanContext().IsValid() && s.IsRecording()
}

// WithOptionalSpan runs fn with an active child span when ProgressiveDebugTracerFrom is non-nil.
// If sp is non-nil, fn should record errors on sp; the wrapper still ends the span after fn returns.
func WithOptionalSpan(ctx context.Context, name string, fn func(context.Context, trace.Span) error) error {
	tr := ProgressiveDebugTracerFrom(ctx)
	if tr == nil {
		return fn(ctx, nil)
	}
	ctx2, sp := tr.Start(ctx, name)
	defer sp.End()
	err := fn(ctx2, sp)
	if err != nil {
		sp.RecordError(err)
		sp.SetStatus(codes.Error, err.Error())
	} else {
		sp.SetStatus(codes.Ok, "")
	}
	return err
}

// TruncateTraceStr truncates UTF-8 text for span attribute size limits.
func TruncateTraceStr(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return utf8SafePrefix(s, maxBytes) + "…"
}

// utf8SafePrefix returns the longest prefix of s whose UTF-8 encoding is at most maxBytes bytes.
// For maxBytes <= 0, returns s unchanged (callers that need empty output must branch first).
func utf8SafePrefix(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return s
	}
	if len(s) <= maxBytes {
		return s
	}
	b := []byte(s)
	b = b[:maxBytes]
	for len(b) > 0 && b[len(b)-1]&0xc0 == 0x80 {
		b = b[:len(b)-1]
	}
	return string(b)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func progressiveAnswerMaxReferences() int {
	v := viper.GetInt("query.progressive_answer_max_references")
	if v <= 0 {
		v = defaultProgressiveAnswerMaxReferences
	}
	return clampInt(v, 3, progressiveAnswerReferencesCeiling)
}

func progressiveAnswerExcerptMaxBytes() int {
	v := viper.GetInt("query.progressive_answer_excerpt_max_bytes")
	if v <= 0 {
		v = defaultProgressiveAnswerExcerptBytes
	}
	return clampInt(v, 800, progressiveAnswerExcerptBytesCeiling)
}

func progressiveAnswerCompletionMaxTokens() int {
	const minTok, maxTok = 64, 8192
	if v := viper.GetInt("query.progressive_answer_max_tokens"); v > 0 {
		return clampInt(v, minTok, maxTok)
	}
	global := progressiveAnswerProviderMaxTokens()
	if global <= 0 {
		global = 2000
	}
	if global < defaultProgressiveAnswerOutTokens {
		return global
	}
	return defaultProgressiveAnswerOutTokens
}

func progressiveAnswerProviderMaxTokens() int {
	p := strings.ToLower(viper.GetString("llm.provider"))
	var n int
	switch p {
	case "anthropic", "claude":
		n = viper.GetInt("llm.anthropic.max_tokens")
	default:
		n = viper.GetInt("llm.openai.max_tokens")
	}
	if n <= 0 {
		return 2000
	}
	return n
}

func truncateUTF8ForPrompt(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	return utf8SafePrefix(s, maxBytes) + "\n…(truncated)"
}

func buildProgressiveAnswerMessages(answerTemplate, question string, items []types.QueryItem, language string) []client.LLMMessage {
	maxRefs := progressiveAnswerMaxReferences()
	n := len(items)
	if n > maxRefs {
		n = maxRefs
	}
	var refs strings.Builder
	refs.WriteString("--- References ---\n")
	for i := 0; i < n; i++ {
		it := items[i]
		excerpt := truncateUTF8ForPrompt(it.Content, progressiveAnswerExcerptMaxBytes())
		fmt.Fprintf(&refs, "\n### Reference [^%d^]\n", i+1)
		fmt.Fprintf(&refs, "Title: %s\n", it.Title)
		fmt.Fprintf(&refs, "Document ID: %s\n", it.ID)
		fmt.Fprintf(&refs, "Path: %s\n", it.Path)
		if it.Status != "" {
			fmt.Fprintf(&refs, "Document status: %s\n", it.Status)
		}
		fmt.Fprintf(&refs, "Relevance (0-1): %.4f\n", it.Relevance)
		refs.WriteString("Excerpt:\n")
		refs.WriteString(excerpt)
		refs.WriteByte('\n')
	}

	systemContent := answerTemplate
	if language != "" {
		systemContent = answerTemplate + "\n\nPlease answer in " + language + "."
	}

	return []client.LLMMessage{
		{Role: client.LLMMessageRoleSystem, Content: systemContent},
		{Role: client.LLMMessageRoleUser, Content: refs.String()},
		{Role: client.LLMMessageRoleUser, Content: strings.TrimSpace(question)},
	}
}

// NewQueryService constructs the query service implementation.
func NewQueryService(
	docRepo storage.IDocumentRepository,
	chapterSearch service.IChapterHybridSearch,
	llm client.ILLMProvider,
	answerPrompt string,
	logger *zap.Logger,
) service.IQueryService {
	return &queryService{
		docRepo:       docRepo,
		chapterSearch: chapterSearch,
		llm:           llm,
		answerPrompt:  answerPrompt,
		logger:        logger,
	}
}

type queryService struct {
	docRepo       storage.IDocumentRepository
	chapterSearch service.IChapterHybridSearch
	llm           client.ILLMProvider
	answerPrompt  string
	logger        *zap.Logger
}

// ProgressiveQuery implements service.IQueryService.
// Runs IChapterHybridSearch.SearchHotChapters and SearchColdChapterHits in parallel, merges hits, then optional LLM answer.
func (s *queryService) ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error) {
	if req.MaxResults == 0 {
		req.MaxResults = 15
	}

	response := &types.ProgressiveQueryResponse{
		Question: req.Question,
		Steps:    []types.ProgressiveQueryStep{},
	}

	var traceIDStr string
	if sp := trace.SpanFromContext(ctx); sp.SpanContext().IsValid() && sp.IsRecording() {
		traceIDStr = sp.SpanContext().TraceID().String()
	}

	wantTrace := ProgressiveTraceRequested(ctx)

	type hotResult struct {
		results []types.QueryItem
		steps   []types.ProgressiveQueryStep
		err     error
	}
	type coldResult struct {
		results []types.QueryItem
		step    types.ProgressiveQueryStep
		err     error
	}

	hotChan := make(chan hotResult, 1)
	coldChan := make(chan coldResult, 1)

	runCtx := ctx
	var tracer trace.Tracer
	var rootSpan trace.Span
	if wantTrace {
		tracer = otel.Tracer(ProgressiveTracerScope)
		runCtx, rootSpan = tracer.Start(ctx, "progressive_query",
			trace.WithAttributes(attribute.String("tier_request_question", TruncateTraceStr(req.Question, 512))))
		runCtx = WithProgressiveDebugTracer(runCtx, tracer)
		defer rootSpan.End()
	}

	half := max(1, req.MaxResults/2)

	go func() {
		execCtx := runCtx
		var pathSpan trace.Span
		if wantTrace {
			execCtx, pathSpan = tracer.Start(runCtx, "hot_path")
			defer pathSpan.End()
			execCtx = WithProgressiveDebugTracer(execCtx, tracer)
		}
		results, steps, err := s.queryHotPath(execCtx, req.Question, half)
		if pathSpan != nil {
			if err != nil {
				pathSpan.RecordError(err)
				pathSpan.SetStatus(codes.Error, err.Error())
			} else {
				pathSpan.SetStatus(codes.Ok, "")
			}
		}
		hotChan <- hotResult{results: results, steps: steps, err: err}
	}()

	go func() {
		execCtx := runCtx
		var pathSpan trace.Span
		if wantTrace {
			execCtx, pathSpan = tracer.Start(runCtx, "cold_path")
			defer pathSpan.End()
			execCtx = WithProgressiveDebugTracer(execCtx, tracer)
		}
		results, step, err := s.queryColdPath(execCtx, req.Question, half)
		if pathSpan != nil {
			if err != nil {
				pathSpan.RecordError(err)
				pathSpan.SetStatus(codes.Error, err.Error())
			} else {
				pathSpan.SetStatus(codes.Ok, "")
			}
		}
		coldChan <- coldResult{results: results, step: step, err: err}
	}()

	hotRes := <-hotChan
	coldRes := <-coldChan

	if hotRes.err != nil {
		s.logger.Error("hot path query failed", zap.Error(hotRes.err))
	} else {
		response.Steps = append(response.Steps, hotRes.steps...)
	}

	if coldRes.err != nil {
		s.logger.Error("cold path query failed", zap.Error(coldRes.err))
	} else {
		response.Steps = append(response.Steps, coldRes.step)
	}

	mergedResults := mergeHotAndColdQueryItems(hotRes.results, coldRes.results, req.MaxResults)
	response.Results = mergedResults

	if wantTrace {
		_, mergeSpan := tracer.Start(runCtx, "merge_results", trace.WithAttributes(
			attribute.Int("tier_request_merge_inputs_hot_items", len(hotRes.results)),
			attribute.Int("tier_request_merge_inputs_cold_items", len(coldRes.results)),
			attribute.Int("tier_response_merged_items", len(mergedResults)),
		))
		mergeSpan.SetStatus(codes.Ok, "")
		mergeSpan.End()

		ansCtx, ansSpan := tracer.Start(runCtx, "synthesize_answer", trace.WithAttributes(
			attribute.String("tier_request_question", TruncateTraceStr(req.Question, TraceMaxReqBytes)),
			attribute.Int("tier_request_reference_items", len(mergedResults)),
		))
		ansCtx = trace.ContextWithSpan(ansCtx, ansSpan)
		ansCtx = WithProgressiveDebugTracer(ansCtx, tracer)
		response.AnswerFromReferences, response.AnswerFromKnowledge = s.generateProgressiveAnswer(ansCtx, req.Question, mergedResults, req.AnswerLanguage)
		response.Answer = response.AnswerFromReferences // backward compatibility
		if response.AnswerFromReferences != "" {
			ansSpan.SetAttributes(attribute.String("tier_response_answer", TruncateTraceStr(response.AnswerFromReferences, TraceMaxRespBytes)))
		}
		if response.AnswerFromKnowledge != "" {
			ansSpan.SetAttributes(attribute.String("tier_response_knowledge", TruncateTraceStr(response.AnswerFromKnowledge, 200)))
		}
		ansSpan.SetStatus(codes.Ok, "")
		ansSpan.End()
	} else {
		response.AnswerFromReferences, response.AnswerFromKnowledge = s.generateProgressiveAnswer(ctx, req.Question, mergedResults, req.AnswerLanguage)
		response.Answer = response.AnswerFromReferences // backward compatibility
	}

	if traceIDStr != "" {
		response.TraceID = traceIDStr
	}

	s.logger.Info("progressive query completed",
		zap.String("question", req.Question),
		zap.Int("hot_results", len(hotRes.results)),
		zap.Int("cold_results", len(coldRes.results)),
		zap.Int("total_results", len(mergedResults)),
		zap.Bool("has_answer", response.AnswerFromReferences != ""),
		zap.String("otel_trace_id", traceIDStr),
	)

	if wantTrace && rootSpan != nil {
		rootSpan.SetStatus(codes.Ok, "")
	}

	return response, nil
}

func (s *queryService) generateProgressiveAnswer(ctx context.Context, question string, items []types.QueryItem, language string) (refsAnswer, knowledgeAnswer string) {
	if s.llm == nil || len(items) == 0 {
		return "", ""
	}
	msgs := buildProgressiveAnswerMessages(s.answerPrompt, question, items, language)
	maxTok := progressiveAnswerCompletionMaxTokens()
	ans, err := s.llm.Generate(ctx, msgs, maxTok)
	if err != nil {
		s.logger.Warn("progressive query: answer generation failed", zap.Error(err))
		return "AI 应答生成失败，请参考右侧召回的章节。", ""
	}
	ans = strings.TrimSpace(ans)
	var inputText strings.Builder
	for _, m := range msgs {
		inputText.WriteString(m.Content)
	}
	metrics.RecordLLMTokens(metrics.PathAnswerGen, estimateQueryTokens(inputText.String()), estimateQueryTokens(ans))
	return parseDualPartAnswer(ans)
}

// parseDualPartAnswer splits the LLM output into the evidence-based part and the knowledge supplement.
// It expects the separator "---PART:KNOWLEDGE---" between the two sections.
func parseDualPartAnswer(raw string) (refsAnswer, knowledgeAnswer string) {
	const sep = "---PART:KNOWLEDGE---"
	idx := strings.Index(raw, sep)
	if idx == -1 {
		// No separator found: treat everything as the reference-based answer.
		return raw, ""
	}
	refsAnswer = strings.TrimSpace(raw[:idx])
	knowledgeAnswer = strings.TrimSpace(raw[idx+len(sep):])
	return refsAnswer, knowledgeAnswer
}

func (s *queryService) queryHotPath(ctx context.Context, question string, half int) ([]types.QueryItem, []types.ProgressiveQueryStep, error) {
	start := time.Now()

	var hits []types.HotSearchHit
	err := WithOptionalSpan(ctx, "hot_chapter_search", func(c context.Context, sp trace.Span) error {
		var e error
		hits, e = s.chapterSearch.SearchHotChapters(c, question, half)
		if sp != nil && e == nil {
			sp.SetAttributes(
				attribute.String("tier_request_question", TruncateTraceStr(question, TraceMaxReqBytes)),
				attribute.Int("tier_response_hot_chapter_hits", len(hits)),
			)
		}
		return e
	})
	if err != nil {
		return nil, nil, fmt.Errorf("hot chapter search: %w", err)
	}

	results, trackDocs := hotSearchHitsToQueryItemsAndDocs(hits)
	s.trackDocumentAccess(ctx, trackDocs)

	steps := []types.ProgressiveQueryStep{{
		Step:     "hot_chapters",
		Input:    question,
		Output:   len(results),
		Duration: time.Since(start).Milliseconds(),
	}}

	metrics.RecordQueryLatency(metrics.QueryPathHot, time.Since(start).Seconds(), len(results))
	return results, steps, nil
}

func queryItemFromChapterPath(docID, path, titleFallback, content string, relevance float64, status types.DocumentStatus, contentSource string) types.QueryItem {
	p := strings.TrimSpace(path)
	if p == "" {
		p = docID + "/full"
	}
	t := extractTitleFromPath(p)
	if t == "" || t == docID {
		t = strings.TrimSpace(titleFallback)
	}
	return types.QueryItem{
		ID:            docID,
		Title:         t,
		Content:       content,
		Path:          p,
		Relevance:     relevance,
		Status:        status,
		ContentSource: contentSource,
	}
}

func hotSearchHitsToQueryItemsAndDocs(hits []types.HotSearchHit) ([]types.QueryItem, []types.Document) {
	results := make([]types.QueryItem, 0, len(hits))
	seen := make(map[string]struct{}, len(hits))
	docs := make([]types.Document, 0, len(hits))
	for i := range hits {
		h := &hits[i]
		results = append(results, queryItemFromChapterPath(h.DocumentID, h.Path, h.Title, h.Content, h.Score, h.Status, h.Source))
		if _, ok := seen[h.DocumentID]; ok {
			continue
		}
		seen[h.DocumentID] = struct{}{}
		docs = append(docs, types.Document{
			ID:         h.DocumentID,
			Status:     h.Status,
			QueryCount: h.QueryCount,
		})
	}
	return results, docs
}

func coldSearchHitsToQueryItems(hits []types.ColdSearchHit) []types.QueryItem {
	out := make([]types.QueryItem, 0, len(hits))
	for i := range hits {
		sr := &hits[i]
		out = append(out, queryItemFromChapterPath(sr.DocumentID, sr.Path, sr.Title, sr.Content, sr.Score, types.DocStatusCold, sr.Source))
	}
	return out
}

func chapterHitDedupeKey(it types.QueryItem) string {
	return it.ID + "\x1e" + it.Path
}

func upsertChapterHits(byKey map[string]types.QueryItem, items []types.QueryItem) {
	for _, r := range items {
		k := chapterHitDedupeKey(r)
		if prev, ok := byKey[k]; ok && prev.Relevance >= r.Relevance {
			continue
		}
		byKey[k] = r
	}
}

func mergeHotAndColdQueryItems(hot, cold []types.QueryItem, maxResults int) []types.QueryItem {
	n := len(hot) + len(cold)
	if n == 0 {
		return nil
	}
	byKey := make(map[string]types.QueryItem, n)
	upsertChapterHits(byKey, hot)
	upsertChapterHits(byKey, cold)
	out := make([]types.QueryItem, 0, len(byKey))
	for _, r := range byKey {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Relevance > out[j].Relevance
	})
	if len(out) > maxResults {
		out = out[:maxResults]
	}
	return out
}

func (s *queryService) queryColdPath(ctx context.Context, question string, half int) ([]types.QueryItem, types.ProgressiveQueryStep, error) {
	start := time.Now()

	var searchResults []types.ColdSearchHit
	err := WithOptionalSpan(ctx, "cold_index_search", func(c context.Context, sp trace.Span) error {
		if sp != nil {
			sp.SetAttributes(
				attribute.String("tier_request_question", TruncateTraceStr(question, TraceMaxReqBytes)),
				attribute.Int("tier_request_cold_search_max_results", half),
			)
		}
		var e error
		searchResults, e = s.chapterSearch.SearchColdChapterHits(c, question, half)
		if e != nil && errors.Is(e, service.ErrColdIndexUnavailable) {
			if sp != nil {
				sp.SetAttributes(
					attribute.Bool("tier.response.cold_index_skipped", true),
					attribute.String("tier_response_cold_index_skip_reason", "no_index"),
				)
			}
			searchResults = nil
			return nil
		}
		if sp != nil && e == nil {
			sp.SetAttributes(attribute.Int("tier_response_cold_index_hits", len(searchResults)))
		}
		return e
	})
	if err != nil {
		return nil, types.ProgressiveQueryStep{}, fmt.Errorf("cold chapter search failed: %w", err)
	}

	results := coldSearchHitsToQueryItems(searchResults)
	metrics.RecordQueryLatency(metrics.QueryPathCold, time.Since(start).Seconds(), len(results))
	step := types.ProgressiveQueryStep{
		Step:     "cold_docs",
		Input:    question,
		Output:   len(results),
		Duration: time.Since(start).Milliseconds(),
	}
	return results, step, nil
}

// trackDocumentAccess increments query count for accessed documents
// and triggers promotion for cold documents that exceed the threshold.
func (s *queryService) trackDocumentAccess(ctx context.Context, docs []types.Document) {
	for _, doc := range docs {
		go func(docID string, status types.DocumentStatus, queryCount int) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := s.docRepo.IncrementQueryCount(bgCtx, docID); err != nil {
				s.logger.Warn("failed to increment query count",
					zap.String("doc_id", docID),
					zap.Error(err))
				return
			}

			threshold := config.ColdPromotionThreshold()
			if status == types.DocStatusCold && queryCount+1 >= threshold {
				select {
				case job.PromoteQueue <- docID:
					s.logger.Info("queued cold document for promotion",
						zap.String("doc_id", docID),
						zap.Int("query_count", queryCount+1))
				default:
					s.logger.Warn("promotion queue full, document not queued",
						zap.String("doc_id", docID))
				}
			}
		}(doc.ID, doc.Status, doc.QueryCount)
	}
}

func extractTitleFromPath(path string) string {
	var last string
	for _, p := range strings.Split(path, "/") {
		if p == "" {
			continue
		}
		last = p
	}
	if last == "" {
		return path
	}
	return last
}

var _ service.IQueryService = (*queryService)(nil)

func estimateQueryTokens(text string) int {
	if text == "" {
		return 0
	}
	chineseCount := 0
	for _, r := range text {
		if r > 127 {
			chineseCount++
		}
	}
	englishChars := len(text) - chineseCount
	return chineseCount + englishChars/4
}
