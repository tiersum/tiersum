package common

import (
	"sync"
	"time"
)

// QuotaManager manages hourly document processing quota.
// Hot documents consume quota, cold documents do not.
type QuotaManager struct {
	mu        sync.RWMutex
	perHour   int
	usedCount int
	resetTime time.Time
}

// NewQuotaManager creates a new quota manager.
func NewQuotaManager(perHour int) *QuotaManager {
	if perHour <= 0 {
		perHour = 100 // default
	}
	return &QuotaManager{
		perHour:   perHour,
		usedCount: 0,
		resetTime: time.Now().Truncate(time.Hour).Add(time.Hour),
	}
}

// CheckAndConsume checks if quota is available and consumes one unit if available.
// Returns true if quota was consumed (should process as hot), false otherwise (should process as cold).
func (q *QuotaManager) CheckAndConsume() bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	if now.After(q.resetTime) {
		q.usedCount = 0
		q.resetTime = now.Truncate(time.Hour).Add(time.Hour)
	}

	if q.usedCount < q.perHour {
		q.usedCount++
		return true
	}
	return false
}

// GetQuota returns current quota status.
func (q *QuotaManager) GetQuota() (used, total int, resetAt time.Time) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	now := time.Now()
	if now.After(q.resetTime) {
		return 0, q.perHour, q.resetTime
	}
	return q.usedCount, q.perHour, q.resetTime
}

// GetRemaining returns how many hot-ingest slots are left this hour.
func (q *QuotaManager) GetRemaining() int {
	used, total, _ := q.GetQuota()
	return total - used
}

// ForceReset resets quota immediately (for testing).
func (q *QuotaManager) ForceReset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.usedCount = 0
	q.resetTime = time.Now().Truncate(time.Hour).Add(time.Hour)
}
