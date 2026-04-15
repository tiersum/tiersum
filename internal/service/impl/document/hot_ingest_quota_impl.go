package document

import (
	"sync"
	"time"

	"github.com/spf13/viper"
)

// HotIngestQuota is a minimal hourly quota gate for "auto" hot ingest.
// It is intentionally in-memory (process-local); during rewrite we avoid DB state for quotas.
type HotIngestQuota struct {
	mu          sync.Mutex
	windowStart time.Time
	used        int
}

func NewHotIngestQuota() *HotIngestQuota {
	return &HotIngestQuota{}
}

func (q *HotIngestQuota) perHour() int {
	n := viper.GetInt("quota.per_hour")
	if n <= 0 {
		return 100
	}
	return n
}

func (q *HotIngestQuota) roll(now time.Time) {
	ws := now.Truncate(time.Hour)
	if q.windowStart.IsZero() || !q.windowStart.Equal(ws) {
		q.windowStart = ws
		q.used = 0
	}
}

// CheckAndConsume returns true and consumes one unit when quota remains; otherwise false.
func (q *HotIngestQuota) CheckAndConsume() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	q.roll(now)
	total := q.perHour()
	if q.used >= total {
		return false
	}
	q.used++
	return true
}

// GetQuota returns the used/total counters and the next reset time.
func (q *HotIngestQuota) GetQuota() (used int, total int, resetAt time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	q.roll(now)
	return q.used, q.perHour(), q.windowStart.Add(time.Hour)
}

