// Package storage defines storage layer interfaces
package storage

import (
	"context"
	"time"

	"github.com/tiersum/tiersum/pkg/types"
)

// IDocumentRepository defines document storage operations
type IDocumentRepository interface {
	Create(ctx context.Context, doc *types.Document) error
	GetByID(ctx context.Context, id string) (*types.Document, error)
	// GetRecent retrieves recent documents up to a limit
	GetRecent(ctx context.Context, limit int) ([]*types.Document, error)
	// ListByTags retrieves documents that match ANY of the given tags (OR logic)
	ListByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error)
	// ListMetaByTagsAndStatuses returns documents matching any tag (OR) and any of the given statuses,
	// without loading body content (content field is empty; summary is loaded).
	ListMetaByTagsAndStatuses(ctx context.Context, tags []string, statuses []types.DocumentStatus, limit int) ([]types.Document, error)
	// ListByStatus retrieves documents by status (hot/cold/warming)
	ListByStatus(ctx context.Context, status types.DocumentStatus, limit int) ([]types.Document, error)
	// IncrementQueryCount increments the query count and updates last_query_at
	IncrementQueryCount(ctx context.Context, docID string) error
	// UpdateStatus updates the document status (hot/cold/warming)
	UpdateStatus(ctx context.Context, docID string, status types.DocumentStatus) error
	// UpdateHotScore updates the hot score for a document
	UpdateHotScore(ctx context.Context, docID string, score float64) error
	// UpdateTags updates document tags (e.g. after async LLM analysis).
	UpdateTags(ctx context.Context, docID string, tags []string) error
	// UpdateSummary updates the persisted document-level summary (hot/warming docs).
	UpdateSummary(ctx context.Context, docID string, summary string) error
	// ListAll returns all documents for hot score calculation
	ListAll(ctx context.Context, limit int) ([]types.Document, error)
	// CountDocumentsByStatus returns counts grouped by status (full table; SQL aggregate).
	CountDocumentsByStatus(ctx context.Context) (types.DocumentStatusCounts, error)
}

// IChapterRepository persists hot-document chapters.
// Schema: chapters(id, document_id, path, title, summary, content, created_at, updated_at)
type IChapterRepository interface {
	// ReplaceByDocument deletes all chapters for document_id and inserts the given rows.
	ReplaceByDocument(ctx context.Context, documentID string, chapters []types.Chapter) error
	// ListByDocument returns chapters for one document (order is implementation-defined).
	ListByDocument(ctx context.Context, documentID string) ([]types.Chapter, error)
	// ListByDocumentIDs returns chapters for multiple documents.
	ListByDocumentIDs(ctx context.Context, documentIDs []string) ([]types.Chapter, error)
	// ListByIDs returns chapters for the given primary keys (used when resolving chapter ids to text).
	ListByIDs(ctx context.Context, chapterIDs []string) ([]types.Chapter, error)
}

// ITagRepository defines catalog tag storage (deduplicated tag names + document counts).
type ITagRepository interface {
	Create(ctx context.Context, tag *types.Tag) error
	GetByName(ctx context.Context, name string) (*types.Tag, error)
	List(ctx context.Context) ([]types.Tag, error)
	ListByTopic(ctx context.Context, topicID string) ([]types.Tag, error)
	// ListByTopicIDs returns tags whose topic_id is in topicIDs, ordered by topic then name, capped at limit.
	ListByTopicIDs(ctx context.Context, topicIDs []string, limit int) ([]types.Tag, error)
	IncrementDocumentCount(ctx context.Context, tagName string) error
	DeleteAll(ctx context.Context) error
	GetCount(ctx context.Context) (int, error)
}

// ITopicRepository persists topics (themes) produced by LLM regrouping of catalog tags.
type ITopicRepository interface {
	Create(ctx context.Context, topic *types.Topic) error
	GetByID(ctx context.Context, id string) (*types.Topic, error)
	List(ctx context.Context) ([]types.Topic, error)
	DeleteAll(ctx context.Context) error
	GetCount(ctx context.Context) (int, error)
}

// IDeviceTokenRepository persists persistent browser "keep me signed in" tokens.
type IDeviceTokenRepository interface {
	Create(ctx context.Context, t *DeviceToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*DeviceToken, error)
	ListByUser(ctx context.Context, userID string) ([]DeviceToken, error)
	TouchUse(ctx context.Context, id string, at time.Time) error
	Revoke(ctx context.Context, id string, at time.Time) error
	RevokeAllForUser(ctx context.Context, userID string, at time.Time) error
}

// IPasskeyCredentialRepository persists WebAuthn credentials per user.
type IPasskeyCredentialRepository interface {
	Create(ctx context.Context, c *PasskeyCredential) error
	ListByUser(ctx context.Context, userID string) ([]PasskeyCredential, error)
	GetByID(ctx context.Context, id string) (*PasskeyCredential, error)
	GetByCredentialID(ctx context.Context, credentialIDB64 string) (*PasskeyCredential, error)
	UpdateSignCountAndLastUsed(ctx context.Context, id string, signCount int64, at time.Time) error
	Delete(ctx context.Context, id string) error
}

// IPasskeySessionVerificationRepository records recent passkey verification per browser session.
type IPasskeySessionVerificationRepository interface {
	Put(ctx context.Context, v *PasskeySessionVerification) error
	GetBySessionID(ctx context.Context, sessionID string) (*PasskeySessionVerification, error)
	DeleteBySessionID(ctx context.Context, sessionID string) error
}

// ICache defines cache operations
type ICache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
}

// Cold index monitoring DTOs live in pkg/types; type aliases keep IColdIndex method signatures stable here.
// IDeletedDocumentRepository records and cleans up deleted document IDs for cold index GC.
// DeletedDocument represents one tombstone row.
type DeletedDocument struct {
	ID         string
	DocumentID string
	CreatedAt  time.Time
}

// IDeletedDocumentRepository records document deletion and exposes entries for cold index cleanup.
type IDeletedDocumentRepository interface {
	// Insert records a deleted document ID.
	Insert(ctx context.Context, documentID string) error
	// ListSince returns tombstones created after the given time.
	ListSince(ctx context.Context, since time.Time, limit int) ([]DeletedDocument, error)
}

type (
	ColdIndexInvertedStats = types.ColdIndexInvertedStats
	ColdIndexVectorStats   = types.ColdIndexVectorStats
)

// IColdIndex is the cold-document index contract for the service layer.
// It exposes only documents and plain-text queries; ranking strategy and storage layout are implementation-defined.
type IColdIndex interface {
	// AddDocument adds or replaces the document in the cold index.
	AddDocument(ctx context.Context, doc *types.Document) error
	// RemoveDocument removes the document from the cold index.
	RemoveDocument(docID string) error
	// AddChapter indexes a single pre-split cold chapter (embedding, Bleve, HNSW).
	// The caller is responsible for calling RemoveDocument first if replacing a document's chapters.
	AddChapter(ctx context.Context, docID, path, title, content string) error
	// Search returns ranked content matches for the query string.
	Search(ctx context.Context, query string, limit int) ([]ColdIndexHit, error)
	// ApproxEntries returns a non-negative size hint for metrics (implementation-defined, e.g. row count).
	ApproxEntries() int
	// VectorStats returns HNSW / embedding fields for dashboards (zeros when the index is not empty or not initialized).
	VectorStats() ColdIndexVectorStats
	// InvertedIndexStats returns Bleve / inverted-text fields for dashboards.
	InvertedIndexStats() ColdIndexInvertedStats
	// RebuildFromDocuments replaces the entire index from the given documents (typically all cold docs).
	RebuildFromDocuments(ctx context.Context, docs []types.Document) error
	// MarkdownChapters extracts chapters from markdown using the same splitter/token budget as cold ingest.
	// Section display titles use pkg/markdown.ChapterDisplayTitle; returned rows are not persisted (ID/timestamps may be empty).
	MarkdownChapters(docID, title, markdown string) []types.Chapter
	Close() error
}

// OtelSpanRow is one span row written by the OpenTelemetry SQL exporter.
type OtelSpanRow struct {
	TraceID        string
	SpanID         string
	ParentSpanID   string
	Name           string
	Kind           string
	StartUnixNano  int64
	EndUnixNano    int64
	StatusCode     string
	StatusMessage  string
	AttributesJSON string
}

// IOtelSpanRepository persists and reads OpenTelemetry spans (progressive-query debug traces).
type IOtelSpanRepository interface {
	InsertSpan(ctx context.Context, row *OtelSpanRow) error
	// ListTraceSummaries returns recent traces whose spans belong to the given service name
	// (resource attribute `service.name` persisted into attributes_json by the exporter).
	ListTraceSummaries(ctx context.Context, serviceName string, limit, offset int) ([]types.OtelTraceSummary, error)
	ListSpansByTraceID(ctx context.Context, traceID string) ([]types.OtelSpanDTO, error)
}

// ColdIndexHit is one ranked match from IColdIndex.Search (one cold document chapter).
// Source is an optional explainability hint for clients (e.g. how the row was surfaced in the implementation); callers must not branch business logic on it.
type ColdIndexHit struct {
	DocumentID string  `json:"document_id"`
	Path       string  `json:"path,omitempty"` // cold chapter path (doc id + heading path)
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Source     string  `json:"source,omitempty"` // trace only: e.g. bm25, vector, hybrid
}

// ISystemAuthStateRepository reads and updates first-boot / initialized flag.
type ISystemAuthStateRepository interface {
	Get(ctx context.Context) (*SystemAuthState, error)
	MarkInitialized(ctx context.Context) error
}

// IAuthUserRepository persists human-track users (hashed access tokens).
type IAuthUserRepository interface {
	Create(ctx context.Context, u *AuthUser) error
	GetByID(ctx context.Context, id string) (*AuthUser, error)
	GetByUsername(ctx context.Context, username string) (*AuthUser, error)
	GetByAccessTokenHash(ctx context.Context, accessTokenHashHex string) (*AuthUser, error)
	UpdateAccessToken(ctx context.Context, userID, accessTokenHashHex string, validUntil *time.Time) error
	UpdateTokenExpiryMode(ctx context.Context, userID, mode string) error
	UpdateTokenValidUntil(ctx context.Context, userID string, validUntil *time.Time) error
	List(ctx context.Context) ([]AuthUser, error)
}

// IBrowserSessionCoreRepository stores browser session rows (CRUD and per-user listing).
type IBrowserSessionCoreRepository interface {
	Create(ctx context.Context, s *BrowserSession) error
	GetByID(ctx context.Context, sessionID string) (*BrowserSession, error)
	GetBySessionTokenHash(ctx context.Context, sessionTokenHashHex string) (*BrowserSession, error)
	UpdateLastSeen(ctx context.Context, sessionID string, at time.Time) error
	UpdateExpiresAt(ctx context.Context, sessionID string, exp time.Time) error
	UpdateDeviceAlias(ctx context.Context, sessionID, alias string) error
	Delete(ctx context.Context, sessionID string) error
	DeleteByUserAndFingerprint(ctx context.Context, userID, fingerprintHashHex string) error
	DeleteAllForUser(ctx context.Context, userID string) error
	ListByUser(ctx context.Context, userID string) ([]BrowserSession, error)
}

// IBrowserSessionLoginPolicyRepository enforces per-user device / fingerprint limits during login.
type IBrowserSessionLoginPolicyRepository interface {
	HasActiveSessionWithFingerprint(ctx context.Context, userID, fingerprintHashHex string, now time.Time) (bool, error)
	CountActiveDistinctFingerprints(ctx context.Context, userID string, now time.Time) (int, error)
}

// IBrowserSessionAdminRepository supports admin-wide session listings and per-user counts.
type IBrowserSessionAdminRepository interface {
	// ListAllWithUsername returns every browser session with owning username (admin views).
	ListAllWithUsername(ctx context.Context) ([]BrowserSessionAdminListRow, error)
	CountByUser(ctx context.Context, userID string) (int, error)
}

// IBrowserSessionRepository is the full browser-session store (composition root field type).
type IBrowserSessionRepository interface {
	IBrowserSessionCoreRepository
	IBrowserSessionLoginPolicyRepository
	IBrowserSessionAdminRepository
}

// IAPIKeyRepository persists service-track API keys (hashed).
type IAPIKeyRepository interface {
	Create(ctx context.Context, k *APIKey) error
	GetByID(ctx context.Context, id string) (*APIKey, error)
	GetByKeyHash(ctx context.Context, keyHashHex string) (*APIKey, error)
	GetActiveByKeyHash(ctx context.Context, keyHashHex string) (*APIKey, error)
	List(ctx context.Context) ([]APIKey, error)
	Revoke(ctx context.Context, id string) error
	TouchLastUsed(ctx context.Context, id, clientIP string, at time.Time) error
}

// IAPIKeyAuditRepository appends and queries API key usage for auditing.
type IAPIKeyAuditRepository interface {
	Insert(ctx context.Context, apiKeyID, method, path, clientIP string, at time.Time) error
	CountSince(ctx context.Context, apiKeyID string, since time.Time) (int64, error)
	CountsPerKeySince(ctx context.Context, since time.Time) (map[string]int64, error)
}
