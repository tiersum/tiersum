// Package service defines service-layer facades exposed to upper layers.
// This file intentionally contains only interfaces consumed by the API and Job layers.
package service

import (
	"context"
	"time"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// IDocumentService defines document business logic
type IDocumentService interface {
	// CreateDocument persists a new document; ingest_mode (and legacy force_hot) selects hot vs cold handling and persistence paths.
	CreateDocument(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error)
	GetDocument(ctx context.Context, id string) (*types.Document, error)
	// ListDocuments returns recent documents (descending by created_at). limit<=0 uses a service default.
	ListDocuments(ctx context.Context, limit int) ([]types.Document, error)

	// ListHotDocumentsWithSummariesByTags returns hot/warming document metadata including persisted document-level summaries.
	// Used by GET /hot/doc_summaries.
	ListHotDocumentsWithSummariesByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error)
}

// IQueryService defines query business logic
type IQueryService interface {
	// ProgressiveQuery runs the tag/topic-guided progressive query (parallel hot DB+LLM path and cold hybrid index path).
	ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error)
}

// IDocumentMaintenanceService covers background document tiering (cold→hot promotion, hot scores).
// Used by the job layer; implementations compose storage indexing and summarization.
type IDocumentMaintenanceService interface {
	// RunColdPromotionSweep scans cold documents and promotes those meeting the query-count threshold.
	RunColdPromotionSweep(ctx context.Context) error
	// PromoteColdDocumentByID promotes a single cold document when it meets the threshold (no-op otherwise).
	PromoteColdDocumentByID(ctx context.Context, docID string) error
	// RecalculateDocumentHotScores refreshes persisted hot_score for all documents.
	RecalculateDocumentHotScores(ctx context.Context) error
}

// IHotIngestProcessor completes deferred LLM analysis and indexing for hot ingests.
// Invoked by the hot-ingest queue consumer (internal/job).
type IHotIngestProcessor interface {
	ProcessHotIngest(ctx context.Context, work types.HotIngestWork) error
}

// IProgramAuth is the minimal surface for /api/v1 and MCP (service track).
type IProgramAuth interface {
	IsSystemInitialized(ctx context.Context) (bool, error)
	ValidateAPIKey(ctx context.Context, bearerToken string) (*APIKeyPrincipal, error)
	APIKeyMeetsScope(principal *APIKeyPrincipal, requiredScope string) bool
	RecordAPIKeyUse(ctx context.Context, keyID, method, path, clientIP string) error
}

// IAuthService covers program track plus bootstrap, browser session, and admin operations.
type IAuthService interface {
	IProgramAuth

	Bootstrap(ctx context.Context, adminUsername string) (*BootstrapResult, error)

	LoginWithAccessToken(ctx context.Context, accessTokenPlain string, fp FingerprintInput, remoteIP, userAgent string) (sessionCookiePlain string, err error)
	ValidateBrowserSession(ctx context.Context, sessionCookiePlain, remoteIP, userAgent string) (*BrowserPrincipal, error)
	LogoutSession(ctx context.Context, sessionCookiePlain string) error

	SlideTouchFromBrowserRequest(ctx context.Context, userID string) error

	CreateUser(ctx context.Context, actor *BrowserPrincipal, username, role string) (*CreatedSecretOnce, error)
	ResetUserAccessToken(ctx context.Context, actor *BrowserPrincipal, targetUserID string) (*CreatedSecretOnce, error)
	ListUsers(ctx context.Context, actor *BrowserPrincipal) ([]UserSummary, error)

	CreateAPIKey(ctx context.Context, actor *BrowserPrincipal, name, scope string, expiresAt *time.Time) (*CreatedSecretOnce, *APIKeySummary, error)
	RevokeAPIKey(ctx context.Context, actor *BrowserPrincipal, keyID string) error
	ListAPIKeys(ctx context.Context, actor *BrowserPrincipal) ([]APIKeySummary, error)

	ListOwnDevices(ctx context.Context, actor *BrowserPrincipal) ([]BrowserDeviceSummary, error)
	ListUserDevicesAdmin(ctx context.Context, actor *BrowserPrincipal, targetUserID string) ([]BrowserDeviceSummary, error)
	ListAllDevicesAdmin(ctx context.Context, actor *BrowserPrincipal) ([]AdminBrowserDeviceSummary, error)
	UpdateDeviceAlias(ctx context.Context, actor *BrowserPrincipal, sessionID, alias string) error
	RevokeDeviceSession(ctx context.Context, actor *BrowserPrincipal, sessionID string) error
	RevokeAllOwnSessions(ctx context.Context, actor *BrowserPrincipal) error

	APIKeyUsageCountsSince(ctx context.Context, actor *BrowserPrincipal, since time.Time) (map[string]int64, error)
}

// IAdminConfigViewService exposes read-only, redacted configuration for admin troubleshooting.
type IAdminConfigViewService interface {
	// RedactedSnapshot returns the effective merged settings (viper) with sensitive values replaced.
	// actor must be a browser principal with admin role.
	RedactedSnapshot(ctx context.Context, actor *BrowserPrincipal) (map[string]interface{}, error)
}

// ITagService exposes catalog tag reads for the API layer.
type ITagService interface {
	ListTags(ctx context.Context, topicIDs []string, byTopicLimit int, listAllCap int) ([]types.Tag, error)
}

// IChapterService exposes chapter reads for document detail.
type IChapterService interface {
	// ListChaptersForDocument returns persisted hot-document chapters (summary + source content) for detail UI.
	ListChaptersByDocumentID(ctx context.Context, documentID string) ([]types.Chapter, error)
	// ExtractChaptersFromMarkdown returns markdown-extracted sections for detail UI when DB chapter rows are absent.
	ExtractChaptersFromMarkdown(ctx context.Context, doc *types.Document) ([]types.Chapter, error)

	// ListChaptersByDocumentIDs returns persisted hot-document chapters grouped by document id.
	// Used by GET /hot/doc_chapters.
	ListChaptersByDocumentIDs(ctx context.Context, docIDs []string) (map[string][]types.Chapter, error)

	// SearchColdChapterHits runs hybrid cold search and returns chapter-level hits.
	// Used by GET /cold/chapter_hits.
	SearchColdChapterHits(ctx context.Context, query string, limit int) ([]types.ColdSearchHit, error)

	// SearchHotChapters runs the progressive hot path (catalog tags/topics → documents → chapters) with LLM relevance at each hop (cold candidates in the doc set use keyword gating only), then returns ranked chapter hits.
	// Used by progressive query; symmetric with SearchColdChapterHits at the chapter-service boundary.
	SearchHotChapters(ctx context.Context, query string, limit int) ([]types.HotSearchHit, error)
}

// IObservabilityService exposes monitoring reads for dashboards.
type IObservabilityService interface {
	// ApproxColdIndexEntries returns the cold index size hint (chapter rows), or 0 if unavailable.
	ApproxColdIndexEntries() int
	// ColdIndexVectorStats returns HNSW / embedding monitoring fields for the cold index (zero value if unavailable).
	ColdIndexVectorStats() storage.ColdIndexVectorStats
	// ColdIndexInvertedStats returns Bleve / inverted-text monitoring fields for the cold index (zero value if unavailable).
	ColdIndexInvertedStats() storage.ColdIndexInvertedStats
}

// ITraceService exposes persisted OpenTelemetry traces for the browser observability UI.
type ITraceService interface {
	// ListTraceSummaries returns recent trace summaries (root span name, span count, timestamps).
	ListTraceSummaries(ctx context.Context, limit, offset int) ([]types.OtelTraceSummary, error)
	// ListSpansByTraceID returns all spans in one trace, ordered by start time.
	ListSpansByTraceID(ctx context.Context, traceID string) ([]types.OtelSpanDTO, error)
}

// ITopicService runs LLM regrouping of catalog tags into themes (topics) and related reads.
type ITopicService interface {
	RegroupTags(ctx context.Context) error
	ShouldRefresh(ctx context.Context) (bool, error)
	ListTopics(ctx context.Context) ([]types.Topic, error)
}
