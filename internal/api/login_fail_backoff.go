package api

import (
	"sync"
	"time"
)

// loginFailBackoff applies a simple exponential cooldown keyed by IP after repeated failed logins.
// It is intentionally process-local.
type loginFailBackoff struct {
	mu      sync.Mutex
	entries map[string]*loginFailEntry
	ttl     time.Duration
}

type loginFailEntry struct {
	failCount int
	blockedTo time.Time
	lastSeen  time.Time
}

func newLoginFailBackoff(ttl time.Duration) *loginFailBackoff {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &loginFailBackoff{entries: make(map[string]*loginFailEntry), ttl: ttl}
}

func (b *loginFailBackoff) Allowed(ip string) (ok bool, blockedUntil time.Time) {
	ip = normalizeClientIP(ip)
	if ip == "" {
		return false, time.Time{}
	}
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.evictLocked(now)
	e := b.entries[ip]
	if e == nil {
		return true, time.Time{}
	}
	e.lastSeen = now
	if now.Before(e.blockedTo) {
		return false, e.blockedTo
	}
	return true, time.Time{}
}

func (b *loginFailBackoff) RecordFailure(ip string) time.Time {
	ip = normalizeClientIP(ip)
	if ip == "" {
		return time.Time{}
	}
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.evictLocked(now)
	e := b.entries[ip]
	if e == nil {
		e = &loginFailEntry{}
		b.entries[ip] = e
	}
	e.failCount++
	e.lastSeen = now
	// 0.5s,1s,2s,4s,8s,16s,32s,60s,120s,240s,300s...
	delay := 500 * time.Millisecond
	for i := 1; i < e.failCount; i++ {
		delay *= 2
		if delay >= 5*time.Minute {
			delay = 5 * time.Minute
			break
		}
	}
	e.blockedTo = now.Add(delay)
	return e.blockedTo
}

func (b *loginFailBackoff) Reset(ip string) {
	ip = normalizeClientIP(ip)
	if ip == "" {
		return
	}
	b.mu.Lock()
	delete(b.entries, ip)
	b.mu.Unlock()
}

func (b *loginFailBackoff) evictLocked(now time.Time) {
	if b.ttl <= 0 || len(b.entries) == 0 {
		return
	}
	for k, e := range b.entries {
		if e == nil {
			delete(b.entries, k)
			continue
		}
		if now.Sub(e.lastSeen) > b.ttl {
			delete(b.entries, k)
		}
	}
}

