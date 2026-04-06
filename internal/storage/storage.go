package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Config holds storage configuration
type Config struct {
	DatabaseURL string
	RedisAddr   string
}

// Storage provides access to all storage backends
type Storage struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
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

	// Connect to Redis
	var rdb *redis.Client
	if cfg.RedisAddr != "" {
		opts := &redis.Options{
			Addr: cfg.RedisAddr,
		}
		rdb = redis.NewClient(opts)

		if err := rdb.Ping(ctx).Err(); err != nil {
			// Log warning but don't fail - Redis is optional
			fmt.Printf("Warning: Redis connection failed: %v\n", err)
			rdb = nil
		}
	}

	return &Storage{
		DB:    db,
		Redis: rdb,
	}, nil
}

// Close closes all storage connections
func (s *Storage) Close() {
	if s.DB != nil {
		s.DB.Close()
	}
	if s.Redis != nil {
		s.Redis.Close()
	}
}

// Health checks if storage is healthy
func (s *Storage) Health(ctx context.Context) error {
	if s.DB != nil {
		if err := s.DB.Ping(ctx); err != nil {
			return fmt.Errorf("database unhealthy: %w", err)
		}
	}
	if s.Redis != nil {
		if err := s.Redis.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis unhealthy: %w", err)
		}
	}
	return nil
}
