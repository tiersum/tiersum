package storage

import (
	"context"
	"time"

	"github.com/spf13/viper"
)

// Storage provides access to all storage backends
type Storage struct {
	DB    *DB
	Cache *Cache
}

// New creates a new storage instance
func New() (*Storage, error) {
	// Initialize database
	db, err := NewDB()
	if err != nil {
		return nil, err
	}

	// Initialize schema
	if err := db.InitSchema(); err != nil {
		db.Close()
		return nil, err
	}

	// Initialize local cache
	ttl := viper.GetDuration("storage.cache.ttl")
	if ttl == 0 {
		ttl = 1 * time.Hour
	}
	cache := NewCache(ttl)

	return &Storage{
		DB:    db,
		Cache: cache,
	}, nil
}

// Close closes all storage connections
func (s *Storage) Close() error {
	if s.Cache != nil {
		s.Cache.Clear()
	}
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

// Health checks if storage is healthy
func (s *Storage) Health(ctx context.Context) error {
	if s.DB != nil {
		if err := s.DB.Health(ctx); err != nil {
			return err
		}
	}
	return nil
}
