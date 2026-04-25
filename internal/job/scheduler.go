// Package job implements background jobs and scheduled tasks
// This is the Job layer - internal scheduled/background tasks
package job

import (
	"context"
	"time"

	"github.com/tiersum/tiersum/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

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
	// Track last execution time for each job. Initialize to now so the first ticker tick does not
	// fire every job at once (e.g. a 1h job should not run on the first 5-minute tick).
	lastRuns := make(map[string]time.Time)
	start := time.Now().UTC()
	for _, job := range s.jobs {
		lastRuns[job.Name()] = start
	}

	for {
		select {
		case <-s.ticker.C:
			now := time.Now()
			for _, job := range s.jobs {
				lastRun := lastRuns[job.Name()]
				if now.Sub(lastRun) >= job.Interval() {
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

	if telemetry.GlobalTracerActive() {
		tr := otel.Tracer("github.com/tiersum/tiersum/job")
		ctx, span := tr.Start(ctx, "job."+job.Name(), trace.WithSpanKind(trace.SpanKindInternal))
		span.SetAttributes(attribute.String("job_name", job.Name()))
		span.SetAttributes(attribute.String("job_interval", job.Interval().String()))
		err := job.Execute(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.Bool("error", true))
			s.logger.Error("job execution failed", zap.String("name", job.Name()), zap.Error(err))
		} else {
			s.logger.Debug("job completed", zap.String("name", job.Name()))
		}
		span.End()
		return
	}

	if err := job.Execute(ctx); err != nil {
		s.logger.Error("job execution failed", zap.String("name", job.Name()), zap.Error(err))
	} else {
		s.logger.Debug("job completed", zap.String("name", job.Name()))
	}
}
