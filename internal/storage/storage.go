package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
)

// Config holds storage configuration
type Config struct {
	DatabaseURL string
}

// Storage provides access to all storage backends
type Storage struct {
	DB    *pgxpool.Pool
	Cache *Cache
}

// New creates a new storage instance
func New(cfg Config) (*Storage, error) {
	ctx := context.Background()

	// Connect to PostgreSQL
	var db *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("parse database config: %w", err)
		}

		db, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err != nil {
			return nil, fmt.Errorf("connect to database: %w", err)
		}

		if err := db.Ping(ctx); err != nil {
			return nil, fmt.Errorf("ping database: %w", err)
		}
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
func (s *Storage) Close() {
	if s.DB != nil {
		s.DB.Close()
	}
	if s.Cache != nil {
		s.Cache.Clear()
	}
}

// Health checks if storage is healthy
func (s *Storage) Health(ctx context.Context) error {
	if s.DB != nil {
		if err := s.DB.Ping(ctx); err != nil {
			return fmt.Errorf("database unhealthy: %w", err)
		}
	}
	return nil
}
