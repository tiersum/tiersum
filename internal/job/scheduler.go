// Package job implements background jobs and scheduled tasks
// This is the Job layer - internal scheduled/background tasks
package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/pkg/types"
)

// PromoteQueue is a buffered channel for document promotion requests
// When a cold document is accessed 3+ times, its ID is sent to this queue
var PromoteQueue = make(chan string, 100)

// HotIngestQueue carries hot documents that need LLM analysis and indexing after the row is persisted.
var HotIngestQueue = make(chan types.HotIngestWork, 100)

// Scheduler manages and executes scheduled jobs
type Scheduler struct {
	jobs   []Job
	logger *zap.Logger
	ticker *time.Ticker
	stop   chan struct{}
}

// Job represents a scheduled job
type Job interface {
	Name() string
	Interval() time.Duration
	Execute(ctx context.Context) error
}

// NewScheduler creates a new job scheduler
func NewScheduler(logger *zap.Logger) *Scheduler {
	return &Scheduler{
		jobs:   make([]Job, 0),
		logger: logger,
		stop:   make(chan struct{}),
	}
}

// Register registers a job with the scheduler
func (s *Scheduler) Register(job Job) {
	s.jobs = append(s.jobs, job)
	s.logger.Info("registered job", zap.String("name", job.Name()), zap.Duration("interval", job.Interval()))
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	if len(s.jobs) == 0 {
		s.logger.Warn("no jobs registered")
		return
	}

	// Find shortest interval for ticker
	minInterval := s.jobs[0].Interval()
	for _, job := range s.jobs {
		if job.Interval() < minInterval {
			minInterval = job.Interval()
		}
	}

	s.ticker = time.NewTicker(minInterval)
	go s.run()
	s.logger.Info("scheduler started", zap.Int("jobs", len(s.jobs)))
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stop)
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.logger.Info("scheduler stopped")
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	// Track last execution time for each job
	lastRuns := make(map[string]time.Time)

	for {
		select {
		case <-s.ticker.C:
			now := time.Now()
			for _, job := range s.jobs {
				lastRun, ok := lastRuns[job.Name()]
				if !ok || now.Sub(lastRun) >= job.Interval() {
					s.executeJob(job)
					lastRuns[job.Name()] = now
				}
			}
		case <-s.stop:
			return
		}
	}
}

// executeJob executes a single job with error handling
func (s *Scheduler) executeJob(job Job) {
	s.logger.Debug("executing job", zap.String("name", job.Name()))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := job.Execute(ctx); err != nil {
		s.logger.Error("job execution failed", zap.String("name", job.Name()), zap.Error(err))
	} else {
		s.logger.Debug("job completed", zap.String("name", job.Name()))
	}
}

// ExecuteNow executes a job immediately (manual trigger)
func (s *Scheduler) ExecuteNow(jobName string) {
	for _, job := range s.jobs {
		if job.Name() == jobName {
			s.executeJob(job)
			return
		}
	}
	s.logger.Warn("job not found", zap.String("name", jobName))
}
