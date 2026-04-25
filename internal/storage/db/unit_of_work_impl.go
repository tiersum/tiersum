// Package db is the storage DB composition root: bundles domain repositories from subpackages.
package db

import (
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/auth"
	"github.com/tiersum/tiersum/internal/storage/db/document"
	"github.com/tiersum/tiersum/internal/storage/db/observability"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
)

// UnitOfWork combines multiple repositories.
type UnitOfWork struct {
	Documents            storage.IDocumentRepository
	Chapters             storage.IChapterRepository
	Tags                 storage.ITagRepository
	Topics               storage.ITopicRepository
	OtelSpans            storage.IOtelSpanRepository
	SystemAuth           storage.ISystemAuthStateRepository
	AuthUsers            storage.IAuthUserRepository
	BrowserSessions      storage.IBrowserSessionRepository
	DeviceTokens         storage.IDeviceTokenRepository
	Passkeys             storage.IPasskeyCredentialRepository
	PasskeyVerifs        storage.IPasskeySessionVerificationRepository
	APIKeys              storage.IAPIKeyRepository
	APIKeyAudit          storage.IAPIKeyAuditRepository
	DeletedDocuments     storage.IDeletedDocumentRepository
}

// NewUnitOfWork creates a new unit of work.
func NewUnitOfWork(db shared.SQLDB, driver string, cache storage.ICache) *UnitOfWork {
	return &UnitOfWork{
		Documents:       document.NewDocumentRepo(db, driver, cache),
		Chapters:        document.NewChapterRepo(db, driver, cache),
		Tags:            document.NewTagRepo(db, driver),
		Topics:          document.NewTopicRepo(db, driver),
		OtelSpans:       observability.NewOtelSpanRepo(db, driver),
		SystemAuth:      auth.NewSystemAuthStateRepo(db, driver),
		AuthUsers:       auth.NewAuthUserRepo(db, driver),
		BrowserSessions: auth.NewBrowserSessionRepo(db, driver),
		DeviceTokens:    auth.NewDeviceTokenRepo(db, driver),
		Passkeys:        auth.NewPasskeyCredentialRepo(db, driver),
		PasskeyVerifs:   auth.NewPasskeySessionVerificationRepo(db, driver),
		APIKeys:           auth.NewAPIKeyRepo(db, driver),
		APIKeyAudit:       auth.NewAPIKeyAuditRepo(db, driver),
		DeletedDocuments:  document.NewDeletedDocumentRepo(db, driver),
	}
}
