// Package cache implements in-memory cache storage layer
package cache

import (
	"sync"
	"time"

	"github.com/tiersum/tiersum/internal/ports"
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
		ttl = 10 * time.Minute // default TTL
	}
	c := &Cache{
		data: make(map[string]cacheItem),
		ttl:  ttl,
	}
	// Start cleanup goroutine
	go c.cleanup()
	return c
}

// Get retrieves a value from cache (implements ports.Cache)
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.data[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(item.expiration) {
		return nil, false
	}

	return item.value, true
}

// Set stores a value in cache with default TTL (implements ports.Cache)
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.ttl)
}

// SetWithTTL stores a value with custom TTL
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

// Delete removes a value from cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Clear removes all values from cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]cacheItem)
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

// Compile-time interface check
var _ ports.Cache = (*Cache)(nil)
