package worker

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nephila016/emailchecker/internal/debug"
	"github.com/nephila016/emailchecker/internal/verifier"
)

// Job represents a verification job
type Job struct {
	Email string
	Index int
}

// Pool manages concurrent workers
type Pool struct {
	workers        int
	verifier       *verifier.Verifier
	delay          time.Duration
	jitter         time.Duration
	healthEmail    string
	healthInterval int

	// Channels
	jobs    chan Job
	results chan *verifier.Result

	// Lifecycle
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	jobsOnce   sync.Once // ensures jobs channel is closed exactly once
	resultsOnce sync.Once // ensures results channel is closed exactly once

	// Metrics (atomic)
	processed   int64
	errors      int64
	healthFails int64

	// Callbacks
	onResult   func(*verifier.Result)
	onProgress func(processed, total int)
}

// PoolConfig holds pool configuration
type PoolConfig struct {
	Workers        int
	Delay          time.Duration
	Jitter         time.Duration
	HealthEmail    string
	HealthInterval int
	BufferSize     int
}

// DefaultPoolConfig returns default configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		Workers:        3,
		Delay:          2 * time.Second,
		Jitter:         1 * time.Second,
		HealthInterval: 10,
		BufferSize:     100,
	}
}

// NewPool creates a new worker pool
func NewPool(v *verifier.Verifier, config *PoolConfig) *Pool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pool{
		workers:        config.Workers,
		verifier:       v,
		delay:          config.Delay,
		jitter:         config.Jitter,
		healthEmail:    config.HealthEmail,
		healthInterval: config.HealthInterval,
		jobs:           make(chan Job, config.BufferSize),
		results:        make(chan *verifier.Result, config.BufferSize),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// SetCallbacks sets optional callback functions
func (p *Pool) SetCallbacks(onResult func(*verifier.Result), onProgress func(processed, total int)) {
	p.onResult = onResult
	p.onProgress = onProgress
}

// Start launches all worker goroutines. Must be called before submitting jobs.
func (p *Pool) Start() {
	log := debug.GetLogger()
	log.Info("POOL", "Starting %d workers", p.workers)

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Submit enqueues a job. Blocks until the job is accepted or the pool is stopped.
func (p *Pool) Submit(email string, index int) {
	select {
	case p.jobs <- Job{Email: email, Index: index}:
	case <-p.ctx.Done():
	}
}

// Results returns the read-only results channel.
func (p *Pool) Results() <-chan *verifier.Result {
	return p.results
}

// closeJobs closes the jobs channel exactly once (safe to call concurrently).
func (p *Pool) closeJobs() {
	p.jobsOnce.Do(func() { close(p.jobs) })
}

// closeResults closes the results channel exactly once (safe to call concurrently).
func (p *Pool) closeResults() {
	p.resultsOnce.Do(func() { close(p.results) })
}

// Close signals workers that no more jobs are coming, waits for all workers to
// finish, then closes the results channel. Call this after all Submit calls.
func (p *Pool) Close() {
	p.closeJobs()
	p.wg.Wait()
	p.closeResults()
}

// Stop cancels all in-flight work immediately and waits for workers to exit.
func (p *Pool) Stop() {
	p.cancel()      // signal workers to exit
	p.closeJobs()   // unblock any workers waiting on jobs
	p.wg.Wait()
	p.closeResults()
}

// Processed returns the number of jobs processed so far (thread-safe).
func (p *Pool) Processed() int64 {
	return atomic.LoadInt64(&p.processed)
}

// Errors returns the number of jobs that resulted in an error (thread-safe).
func (p *Pool) Errors() int64 {
	return atomic.LoadInt64(&p.errors)
}

// HealthFails returns the number of health check failures (thread-safe).
func (p *Pool) HealthFails() int64 {
	return atomic.LoadInt64(&p.healthFails)
}

// worker processes jobs from the queue until it is closed or the context is done.
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	log := debug.GetLogger()
	log.Detail("WORKER", "Worker %d started", id)

	localProcessed := 0

	for {
		select {
		case job, ok := <-p.jobs:
			if !ok {
				log.Detail("WORKER", "Worker %d shutting down (processed %d)", id, localProcessed)
				return
			}

			// Periodic health check — re-queue the current job on failure so it
			// is not silently dropped.
			if p.healthEmail != "" && p.healthInterval > 0 {
				if localProcessed > 0 && localProcessed%p.healthInterval == 0 {
					if !p.runHealthCheck() {
						log.Error("WORKER", "Worker %d: health check failed, pausing 30s", id)
						atomic.AddInt64(&p.healthFails, 1)
						time.Sleep(30 * time.Second)

						// Re-queue the job so it is not lost
						select {
						case p.jobs <- job:
						case <-p.ctx.Done():
							return
						}
						continue
					}
				}
			}

			// Verify the email
			result := p.verifier.Verify(job.Email)

			atomic.AddInt64(&p.processed, 1)
			if result.Status == verifier.StatusError {
				atomic.AddInt64(&p.errors, 1)
			}

			// Forward result — respects cancellation
			select {
			case p.results <- result:
			case <-p.ctx.Done():
				return
			}

			// Fire result callback (if set)
			if p.onResult != nil {
				p.onResult(result)
			}

			localProcessed++

			// Rate-limit with jitter between verifications
			p.rateLimitDelay()

		case <-p.ctx.Done():
			log.Detail("WORKER", "Worker %d cancelled", id)
			return
		}
	}
}

// rateLimitDelay sleeps for the configured delay plus a random jitter.
func (p *Pool) rateLimitDelay() {
	if p.delay <= 0 {
		return
	}
	delay := p.delay
	if p.jitter > 0 {
		delay += time.Duration(rand.Int63n(int64(p.jitter)))
	}
	time.Sleep(delay)
}

// runHealthCheck verifies the configured health email to confirm the SMTP
// path is still working.
func (p *Pool) runHealthCheck() bool {
	log := debug.GetLogger()
	log.Info("HEALTH", "Running health check with: %s", p.healthEmail)

	result := p.verifier.Verify(p.healthEmail)
	if result.Status == verifier.StatusValid {
		log.Success("HEALTH", "Health check passed")
		return true
	}

	log.Error("HEALTH", "Health check failed: %s (status: %s)", p.healthEmail, result.Status)
	return false
}

// ProcessEmails is a convenience wrapper: starts the pool, submits all emails,
// waits for completion, and returns results in the original order.
//
// NOTE: Do not mix ProcessEmails with manual Start/Submit/Close calls.
func (p *Pool) ProcessEmails(emails []string) []*verifier.Result {
	resultMap := make(map[string]*verifier.Result, len(emails))
	var mu sync.Mutex

	// Collect results in background
	done := make(chan struct{})
	go func() {
		for result := range p.results {
			mu.Lock()
			resultMap[result.Email] = result
			mu.Unlock()
		}
		close(done)
	}()

	// Start workers (ProcessEmails owns the lifecycle)
	p.Start()

	// Submit all jobs
	for i, email := range emails {
		p.Submit(email, i)
	}

	// Signal no more jobs and wait for everything to finish
	p.Close()
	<-done

	// Return results in original submission order
	results := make([]*verifier.Result, 0, len(emails))
	for _, email := range emails {
		if result, ok := resultMap[email]; ok {
			results = append(results, result)
		}
	}
	return results
}

// Stats holds a snapshot of pool statistics
type Stats struct {
	Processed   int64
	Errors      int64
	HealthFails int64
	Duration    time.Duration
	Rate        float64 // emails per second
}

// GetStats returns a current statistics snapshot
func (p *Pool) GetStats(startTime time.Time) *Stats {
	processed := p.Processed()
	duration := time.Since(startTime)

	var rate float64
	if duration.Seconds() > 0 {
		rate = float64(processed) / duration.Seconds()
	}

	return &Stats{
		Processed:   processed,
		Errors:      p.Errors(),
		HealthFails: p.HealthFails(),
		Duration:    duration,
		Rate:        rate,
	}
}
