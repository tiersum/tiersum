package svcimpl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuotaManager_CheckAndConsume(t *testing.T) {
	tests := []struct {
		name           string
		perHour        int
		initialUsed    int
		expectedResult bool
		expectedUsed   int
	}{
		{
			name:           "should allow when quota available",
			perHour:        100,
			initialUsed:    0,
			expectedResult: true,
			expectedUsed:   1,
		},
		{
			name:           "should deny when quota exhausted",
			perHour:        100,
			initialUsed:    100,
			expectedResult: false,
			expectedUsed:   100,
		},
		{
			name:           "should allow at boundary",
			perHour:        100,
			initialUsed:    99,
			expectedResult: true,
			expectedUsed:   100,
		},
		{
			name:           "should handle zero quota",
			perHour:        0,
			initialUsed:    0,
			expectedResult: false,
			expectedUsed:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qm := NewQuotaManager(tt.perHour)
			qm.used = tt.initialUsed

			result := qm.CheckAndConsume()

			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedUsed, qm.used)
		})
	}
}

func TestQuotaManager_ResetCycle(t *testing.T) {
	qm := NewQuotaManager(100)
	
	// Consume some quota
	for i := 0; i < 50; i++ {
		qm.CheckAndConsume()
	}
	assert.Equal(t, 50, qm.used)
	
	// Reset the cycle
	qm.resetCycle()
	
	// Should be able to consume again
	assert.Equal(t, 0, qm.used)
	result := qm.CheckAndConsume()
	assert.True(t, result)
	assert.Equal(t, 1, qm.used)
}

func TestQuotaManager_ConcurrentAccess(t *testing.T) {
	qm := NewQuotaManager(1000)
	
	done := make(chan bool, 100)
	
	// Spawn 100 goroutines trying to consume quota
	for i := 0; i < 100; i++ {
		go func() {
			qm.CheckAndConsume()
			done <- true
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
	
	// Should have consumed exactly 100
	assert.Equal(t, 100, qm.used)
}

func TestQuotaManager_GetRemaining(t *testing.T) {
	qm := NewQuotaManager(100)
	
	// Initially all available
	assert.Equal(t, 100, qm.GetRemaining())
	
	// Consume some
	for i := 0; i < 30; i++ {
		qm.CheckAndConsume()
	}
	assert.Equal(t, 70, qm.GetRemaining())
	
	// Consume all
	for i := 0; i < 70; i++ {
		qm.CheckAndConsume()
	}
	assert.Equal(t, 0, qm.GetRemaining())
}

func TestQuotaManager_TimeBasedReset(t *testing.T) {
	// This is an integration-style test that verifies reset happens
	// In real scenario, reset is triggered by the first CheckAndConsume after hour boundary
	
	qm := NewQuotaManager(10)
	
	// Consume all quota
	for i := 0; i < 10; i++ {
		qm.CheckAndConsume()
	}
	assert.Equal(t, 0, qm.GetRemaining())
	
	// Simulate time passing by manually resetting
	qm.resetCycle()
	
	// Should be able to consume again
	assert.Equal(t, 10, qm.GetRemaining())
	result := qm.CheckAndConsume()
	assert.True(t, result)
}

func TestQuotaManager_ForceReset(t *testing.T) {
	qm := NewQuotaManager(100)
	
	// Consume some quota
	for i := 0; i < 50; i++ {
		qm.CheckAndConsume()
	}
	assert.Equal(t, 50, qm.used)
	
	// Force reset (typically used in testing)
	qm.ForceReset()
	
	// Should be at zero
	assert.Equal(t, 0, qm.used)
	assert.Equal(t, 100, qm.GetRemaining())
}

func TestNewQuotaManager_DefaultConfig(t *testing.T) {
	// Test with default config (perHour = 0, should use default)
	qm := NewQuotaManager(0)
	
	// Should use default of 100
	assert.Equal(t, 100, qm.perHour)
}

func TestNewQuotaManager_CustomConfig(t *testing.T) {
	qm := NewQuotaManager(50)
	
	assert.Equal(t, 50, qm.perHour)
	assert.Equal(t, 0, qm.used)
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
	
	// Verify exact count despite concurrent access
	assert.Equal(t, concurrentRequests, qm.used)
}
