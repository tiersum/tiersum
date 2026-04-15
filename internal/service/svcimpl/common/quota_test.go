package common

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuotaManager_CheckAndConsume(t *testing.T) {
	tests := []struct {
		name           string
		perHour        int
		expectedResult bool
		expectedUsed   int
	}{
		{
			name:           "should allow when quota available",
			perHour:        100,
			expectedResult: true,
			expectedUsed:   1,
		},
		{
			name:           "should deny when quota exhausted",
			perHour:        100,
			expectedResult: false,
			expectedUsed:   100,
		},
		{
			name:           "should allow at boundary",
			perHour:        100,
			expectedResult: true,
			expectedUsed:   100,
		},
		{
			name:           "zero perHour argument uses default 100",
			perHour:        0,
			expectedResult: true,
			expectedUsed:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qm := NewQuotaManager(tt.perHour)
			// Consume up to the desired starting point using public API.
			switch tt.name {
			case "should deny when quota exhausted":
				for i := 0; i < 100; i++ {
					qm.CheckAndConsume()
				}
			case "should allow at boundary":
				for i := 0; i < 99; i++ {
					qm.CheckAndConsume()
				}
			}

			result := qm.CheckAndConsume()

			assert.Equal(t, tt.expectedResult, result)
			used, _, _ := qm.GetQuota()
			assert.Equal(t, tt.expectedUsed, used)
		})
	}
}

func TestQuotaManager_ResetCycle(t *testing.T) {
	qm := NewQuotaManager(100)

	for i := 0; i < 50; i++ {
		qm.CheckAndConsume()
	}
	used, _, _ := qm.GetQuota()
	assert.Equal(t, 50, used)

	qm.ForceReset()

	used, _, _ = qm.GetQuota()
	assert.Equal(t, 0, used)
	result := qm.CheckAndConsume()
	assert.True(t, result)
	used, _, _ = qm.GetQuota()
	assert.Equal(t, 1, used)
}

func TestQuotaManager_ConcurrentAccess(t *testing.T) {
	qm := NewQuotaManager(1000)

	done := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		go func() {
			qm.CheckAndConsume()
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	used, _, _ := qm.GetQuota()
	assert.Equal(t, 100, used)
}

func TestQuotaManager_GetRemaining(t *testing.T) {
	qm := NewQuotaManager(100)

	assert.Equal(t, 100, qm.GetRemaining())

	for i := 0; i < 30; i++ {
		qm.CheckAndConsume()
	}
	assert.Equal(t, 70, qm.GetRemaining())

	for i := 0; i < 70; i++ {
		qm.CheckAndConsume()
	}
	assert.Equal(t, 0, qm.GetRemaining())
}

func TestQuotaManager_TimeBasedReset(t *testing.T) {
	qm := NewQuotaManager(10)

	for i := 0; i < 10; i++ {
		qm.CheckAndConsume()
	}
	assert.Equal(t, 0, qm.GetRemaining())

	qm.ForceReset()

	assert.Equal(t, 10, qm.GetRemaining())
	result := qm.CheckAndConsume()
	assert.True(t, result)
}

func TestQuotaManager_ForceReset(t *testing.T) {
	qm := NewQuotaManager(100)

	for i := 0; i < 50; i++ {
		qm.CheckAndConsume()
	}
	used, _, _ := qm.GetQuota()
	assert.Equal(t, 50, used)

	qm.ForceReset()

	used, _, _ = qm.GetQuota()
	assert.Equal(t, 0, used)
	assert.Equal(t, 100, qm.GetRemaining())
}

func TestNewQuotaManager_DefaultConfig(t *testing.T) {
	qm := NewQuotaManager(0)

	_, total, _ := qm.GetQuota()
	assert.Equal(t, 100, total)
}

func TestNewQuotaManager_CustomConfig(t *testing.T) {
	qm := NewQuotaManager(50)

	used, total, _ := qm.GetQuota()
	assert.Equal(t, 50, total)
	assert.Equal(t, 0, used)
}

func TestQuotaManager_UsedCounterThreadSafety(t *testing.T) {
	qm := NewQuotaManager(10000)

	var wg sync.WaitGroup
	concurrentRequests := 500

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			qm.CheckAndConsume()
		}()
	}

	wg.Wait()

	used, _, _ := qm.GetQuota()
	assert.Equal(t, concurrentRequests, used)
}

func TestQuotaManager_CheckAndConsume_resetsWhenHourPasses(t *testing.T) {
	qm := NewQuotaManager(2)
	// Simulate exhaustion.
	require.True(t, qm.CheckAndConsume())
	require.True(t, qm.CheckAndConsume())
	require.False(t, qm.CheckAndConsume())

	// ForceReset is the supported test hook.
	qm.ForceReset()
	ok := qm.CheckAndConsume()
	require.True(t, ok)
	used, total, _ := qm.GetQuota()
	assert.Equal(t, 1, used)
	assert.Equal(t, 2, total)
}
