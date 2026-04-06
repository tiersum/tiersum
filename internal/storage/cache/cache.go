// Package cache implements in-memory cache storage layer
package cache

import (
	"sync"
	"time"

	"github.com/tiersum/tiersum/internal/storage"
)

// Cache is a simple in-memory cache with TTL support
type Cache struct {
	data map[string]cacheItem
	mu   sync.RWMutex
	ttl  time.Duration
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewCache creates a new cache with the specified TTL
func NewCache(ttl time.Duration) *Cache {
	if ttl == 0 {
		ttl = 10 * time.Minute
	}
	c := &Cache{
		data: make(map[string]cacheItem),
		ttl:  ttl,
	}
	go c.cleanup()
	return c
}

// Get implements storage.ICache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.data[key]
	if !exists {
		return nil, false
	}
	if time.Now().After(item.expiration) {
		return nil, false
	}
	return item.value, true
}

// Set implements storage.ICache
func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cacheItem{
		value:      value,
		expiration: time.Now().Add(c.ttl),
	}
}

// cleanup periodically removes expired items
func (c *Cache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.data {
			if now.After(item.expiration) {
				delete(c.data, key)
			}
		}
		c.mu.Unlock()
	}
}

var _ storage.ICache = (*Cache)(nil)
