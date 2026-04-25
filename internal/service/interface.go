// Package service defines service-layer facades exposed to upper layers.
// This file intentionally contains only interfaces consumed by the API and Job layers.
package service

import (
	"context"
	"time"

	"github.com/tiersum/tiersum/pkg/types"
)

// IDocumentService defines document business logic
type IDocumentService interface {
	// CreateDocument persists a new document; ingest_mode (and legacy force_hot) selects hot vs cold handling and persistence paths.
	CreateDocument(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error)
	GetDocument(ctx context.Context, id string) (*types.Document, error)
	// ListDocuments returns recent documents (descending by created_at). limit<=0 uses a service default.
	ListDocuments(ctx context.Context, limit int) ([]types.Document, error)
	// CountDocumentsByStatus returns full-table aggregates by document status (hot/cold/warming).
	CountDocumentsByStatus(ctx context.Context) (types.DocumentStatusCounts, error)

	// ListHotDocumentsWithSummariesByTags returns hot/warming document metadata including persisted document-level summaries.
	// Used by GET /hot/doc_summaries.
	ListHotDocumentsWithSummariesByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error)
}

// IQueryService defines query business logic
type IQueryService interface {
	// ProgressiveQuery runs the tag/topic-guided progressive query (parallel hot DB+LLM path and cold hybrid index path).
	ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error)
}

// IDocumentMaintenanceService covers background document tiering (cold→hot promotion, hot scores, cold index refresh).
// Used by the job layer; implementations compose storage indexing and summarization.
type IDocumentMaintenanceService interface {
	// RunColdPromotionSweep scans cold documents and promotes those meeting the query-count threshold.
	RunColdPromotionSweep(ctx context.Context) error
	// PromoteColdDocumentByID promotes a single cold document when it meets the threshold (no-op otherwise).
	PromoteColdDocumentByID(ctx context.Context, docID string) error
	// RecalculateDocumentHotScores refreshes persisted hot_score for all documents.
	RecalculateDocumentHotScores(ctx context.Context) error
	// RefreshColdIndex reads updated cold chapters from the database and rebuilds the in-memory cold index.
	// This is the mechanism by which multi-instance deployments converge: each instance's periodic job
	// polls the shared chapters table for changes since the last refresh.
	RefreshColdIndex(ctx context.Context) error
}

// IHotIngestProcessor completes deferred LLM analysis and indexing for hot ingests.
// Invoked by the hot-ingest queue consumer (internal/job).
type IHotIngestProcessor interface {
	ProcessHotIngest(ctx context.Context, work types.HotIngestWork) error
}

// IHotIngestWorkSink submits work to the async hot-ingest pipeline (composition root; non-blocking send).
// Implementations are nil when deferred hot ingest is unavailable.
type IHotIngestWorkSink interface {
	SubmitHotIngest(work types.HotIngestWork)
}

// IProgramAuth is the minimal surface for /api/v1 and MCP (service track).
type IProgramAuth interface {
	IsSystemInitialized(ctx context.Context) (bool, error)
	ValidateAPIKey(ctx context.Context, bearerToken string) (*APIKeyPrincipal, error)
	APIKeyMeetsScope(principal *APIKeyPrincipal, requiredScope string) bool
	RecordAPIKeyUse(ctx context.Context, keyID, method, path, clientIP string) error
}

// IAuthBootstrap performs one-time system initialization (first admin, API key, initialized flag).
type IAuthBootstrap interface {
	Bootstrap(ctx context.Context, adminUsername string) (*BootstrapResult, error)
}

// IBrowserSessionValidator resolves the HttpOnly session cookie to a browser principal (BFF session gate).
type IBrowserSessionValidator interface {
	ValidateBrowserSession(ctx context.Context, sessionCookiePlain, remoteIP, userAgent string) (*BrowserPrincipal, error)
}

// IBFFSessionMiddlewareAuth is the dependency surface for BFFSessionMiddleware (initialized check + session cookie).
type IBFFSessionMiddlewareAuth interface {
	IProgramAuth
	IBrowserSessionValidator
}

// IBrowserCredentialAuth handles human login, sliding session updates, and persistent device tokens.
type IBrowserCredentialAuth interface {
	IBrowserSessionValidator
	LoginWithAccessToken(ctx context.Context, accessTokenPlain string, fp FingerprintInput, remoteIP, userAgent string) (sessionCookiePlain string, err error)
	// DeviceLogin issues a new browser session from a previously issued persistent device token.
	DeviceLogin(ctx context.Context, deviceTokenPlain string, fp FingerprintInput, remoteIP, userAgent string) (sessionCookiePlain string, err error)
	LogoutSession(ctx context.Context, sessionCookiePlain string) error
	// CreateDeviceTokenForSession issues a persistent device token bound to the current session.
	CreateDeviceTokenForSession(ctx context.Context, actor *BrowserPrincipal, deviceName, remoteIP, userAgent string) (deviceTokenPlain string, err error)
	ListOwnDeviceTokens(ctx context.Context, actor *BrowserPrincipal) ([]DeviceTokenSummary, error)
	RevokeDeviceToken(ctx context.Context, actor *BrowserPrincipal, tokenID string) error
	RevokeAllOwnDeviceTokens(ctx context.Context, actor *BrowserPrincipal) error
	SlideTouchFromBrowserRequest(ctx context.Context, userID string) error
}

// IAdminAuthDirectory manages users, API keys, browser devices (admin + self-service), and audit reads.
type IAdminAuthDirectory interface {
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

// IPasskeyPolicyReader exposes whether passkeys gate admin routes (BFF admin passkey middleware).
type IPasskeyPolicyReader interface {
	PasskeyStatus(ctx context.Context, actor *BrowserPrincipal) (*PasskeyStatus, error)
}

// IPasskeyAuth manages WebAuthn credentials and ceremonies for browser users.
type IPasskeyAuth interface {
	IPasskeyPolicyReader
	ListPasskeys(ctx context.Context, actor *BrowserPrincipal) ([]PasskeySummary, error)
	RevokePasskey(ctx context.Context, actor *BrowserPrincipal, passkeyID string) error
	BeginPasskeyRegistration(ctx context.Context, actor *BrowserPrincipal, rpID, origin, deviceName string) (any, error)
	FinishPasskeyRegistration(ctx context.Context, actor *BrowserPrincipal, credential any) error
	BeginPasskeyVerification(ctx context.Context, actor *BrowserPrincipal, rpID, origin string) (any, error)
	FinishPasskeyVerification(ctx context.Context, actor *BrowserPrincipal, assertion any) error
}

// IAuthService is the browser/admin auth façade (composed capability-sized interfaces).
type IAuthService interface {
	IProgramAuth
	IAuthBootstrap
	IBrowserCredentialAuth
	IAdminAuthDirectory
	IPasskeyAuth
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

// IChapterDocumentReads loads persisted chapter rows for hot documents (detail UI and hot chapter APIs).
type IChapterDocumentReads interface {
	ListChaptersByDocumentID(ctx context.Context, documentID string) ([]types.Chapter, error)
	// ListChaptersByDocumentIDs returns persisted hot-document chapters grouped by document id (GET /hot/doc_chapters).
	ListChaptersByDocumentIDs(ctx context.Context, docIDs []string) (map[string][]types.Chapter, error)
}

// IChapterMarkdownFallback extracts sections from raw markdown when DB chapter rows are absent (e.g. cold docs in UI).
type IChapterMarkdownFallback interface {
	ExtractChaptersFromMarkdown(ctx context.Context, doc *types.Document) ([]types.Chapter, error)
}

// IChapterHybridSearch runs cold hybrid index search and progressive hot chapter discovery (query + cold probe endpoints).
type IChapterHybridSearch interface {
	SearchColdChapterHits(ctx context.Context, query string, limit int) ([]types.ColdSearchHit, error)
	SearchHotChapters(ctx context.Context, query string, limit int) ([]types.HotSearchHit, error)
}

// IChapterService composes persistence reads, markdown fallback, and hybrid search for chapters.
type IChapterService interface {
	IChapterDocumentReads
	IChapterMarkdownFallback
	IChapterHybridSearch
}

// IObservabilityService exposes monitoring reads for dashboards.
type IObservabilityService interface {
	// ApproxColdIndexEntries returns the cold index size hint (chapter rows), or 0 if unavailable.
	ApproxColdIndexEntries() int
	// ColdIndexVectorStats returns HNSW / embedding monitoring fields for the cold index (zero value if unavailable).
	ColdIndexVectorStats() types.ColdIndexVectorStats
	// ColdIndexInvertedStats returns Bleve / inverted-text monitoring fields for the cold index (zero value if unavailable).
	ColdIndexInvertedStats() types.ColdIndexInvertedStats
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
	// RegroupTagsIfNeeded runs RegroupTags only when ShouldRefresh is true (scheduled/job path).
	RegroupTagsIfNeeded(ctx context.Context) error
	ShouldRefresh(ctx context.Context) (bool, error)
	ListTopics(ctx context.Context) ([]types.Topic, error)
}
