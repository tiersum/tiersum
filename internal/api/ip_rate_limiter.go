package api

import (
	"net"
	"sync"
	"time"
)

// ipRateLimiter is a small in-memory token bucket limiter keyed by client IP.
// It is intentionally process-local (sufficient for single instance / small deployments).
type ipRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*ipRateEntry
	cfg     ipRateConfig
}

type ipRateConfig struct {
	// Capacity is the maximum bucket size (burst).
	Capacity float64
	// RefillPerSec is tokens added per second.
	RefillPerSec float64
	// EntryTTL evicts idle entries.
	EntryTTL time.Duration
}

type ipRateEntry struct {
	tokens   float64
	last     time.Time
	lastSeen time.Time
}

func newIPRateLimiter(cfg ipRateConfig) *ipRateLimiter {
	if cfg.Capacity <= 0 {
		cfg.Capacity = 5
	}
	if cfg.RefillPerSec <= 0 {
		cfg.RefillPerSec = 0.2 // 12/min
	}
	if cfg.EntryTTL <= 0 {
		cfg.EntryTTL = 30 * time.Minute
	}
	return &ipRateLimiter{
		entries: make(map[string]*ipRateEntry),
		cfg:     cfg,
	}
}

func (l *ipRateLimiter) Allow(ip string) bool {
	ip = normalizeClientIP(ip)
	if ip == "" {
		return false
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.evictLocked(now)

	e := l.entries[ip]
	if e == nil {
		e = &ipRateEntry{tokens: l.cfg.Capacity, last: now, lastSeen: now}
		l.entries[ip] = e
	}
	// Refill.
	dt := now.Sub(e.last).Seconds()
	if dt > 0 {
		e.tokens += dt * l.cfg.RefillPerSec
		if e.tokens > l.cfg.Capacity {
			e.tokens = l.cfg.Capacity
		}
	}
	e.last = now
	e.lastSeen = now
	if e.tokens < 1 {
		return false
	}
	e.tokens -= 1
	return true
}

func (l *ipRateLimiter) evictLocked(now time.Time) {
	if l.cfg.EntryTTL <= 0 || len(l.entries) == 0 {
		return
	}
	for k, e := range l.entries {
		if e == nil {
			delete(l.entries, k)
			continue
		}
		if now.Sub(e.lastSeen) > l.cfg.EntryTTL {
			delete(l.entries, k)
		}
	}
}

// normalizeClientIP canonicalizes gin.ClientIP() inputs.
func normalizeClientIP(ip string) string {
	if ip == "" {
		return ""
	}
	// Strip port if any.
	if h, _, err := net.SplitHostPort(ip); err == nil && h != "" {
		ip = h
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	return parsed.String()
}

