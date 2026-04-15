// Package db implements database storage layer
package db

import (
	"github.com/tiersum/tiersum/internal/storage"
)

// UnitOfWork combines multiple repositories
type UnitOfWork struct {
	Documents       storage.IDocumentRepository
	Chapters        storage.IChapterRepository
	Tags            storage.ITagRepository
	Topics          storage.ITopicRepository
	OtelSpans       storage.IOtelSpanRepository
	SystemAuth      storage.ISystemAuthStateRepository
	AuthUsers       storage.IAuthUserRepository
	BrowserSessions storage.IBrowserSessionRepository
	APIKeys         storage.IAPIKeyRepository
	APIKeyAudit     storage.IAPIKeyAuditRepository
}

// NewUnitOfWork creates a new unit of work
func NewUnitOfWork(db sqlDB, driver string, cache storage.ICache) *UnitOfWork {
	return &UnitOfWork{
		Documents:       NewDocumentRepo(db, driver, cache),
		Chapters:        NewChapterRepo(db, driver, cache),
		Tags:            NewTagRepo(db, driver, cache),
		Topics:          NewTopicRepo(db, driver, cache),
		OtelSpans:       NewOtelSpanRepo(db, driver),
		SystemAuth:      NewSystemAuthStateRepo(db, driver),
		AuthUsers:       NewAuthUserRepo(db, driver),
		BrowserSessions: NewBrowserSessionRepo(db, driver),
		APIKeys:         NewAPIKeyRepo(db, driver),
		APIKeyAudit:     NewAPIKeyAuditRepo(db, driver),
	}
}
