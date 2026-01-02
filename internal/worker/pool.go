package worker

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yourusername/emailverify/internal/debug"
	"github.com/yourusername/emailverify/internal/verifier"
)

// Job represents a verification job
type Job struct {
	Email string
	Index int
}

// Pool manages concurrent workers
type Pool struct {
	workers      int
	verifier     *verifier.Verifier
	delay        time.Duration
	jitter       time.Duration
	healthEmail  string
	healthInterval int

	// Channels
	jobs    chan Job
	results chan *verifier.Result

	// State
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	processed  int64
	errors     int64
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
		workers:       config.Workers,
		verifier:      v,
		delay:         config.Delay,
		jitter:        config.Jitter,
		healthEmail:   config.HealthEmail,
		healthInterval: config.HealthInterval,
		jobs:          make(chan Job, config.BufferSize),
		results:       make(chan *verifier.Result, config.BufferSize),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// SetCallbacks sets callback functions
func (p *Pool) SetCallbacks(onResult func(*verifier.Result), onProgress func(processed, total int)) {
	p.onResult = onResult
	p.onProgress = onProgress
}

// Start starts the worker pool
func (p *Pool) Start() {
	log := debug.GetLogger()
	log.Info("POOL", "Starting %d workers", p.workers)

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Submit submits a job to the pool
func (p *Pool) Submit(email string, index int) {
	select {
	case p.jobs <- Job{Email: email, Index: index}:
	case <-p.ctx.Done():
	}
}

// Results returns the results channel
func (p *Pool) Results() <-chan *verifier.Result {
	return p.results
}

// Close closes the job channel and waits for workers to finish
func (p *Pool) Close() {
	close(p.jobs)
	p.wg.Wait()
	close(p.results)
}

// Stop stops the pool immediately
func (p *Pool) Stop() {
	p.cancel()
	close(p.jobs)
	p.wg.Wait()
	close(p.results)
}

// Processed returns the number of processed jobs
func (p *Pool) Processed() int64 {
	return atomic.LoadInt64(&p.processed)
}

// Errors returns the number of errors
func (p *Pool) Errors() int64 {
	return atomic.LoadInt64(&p.errors)
}

// HealthFails returns the number of health check failures
func (p *Pool) HealthFails() int64 {
	return atomic.LoadInt64(&p.healthFails)
}

// worker processes jobs from the queue
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

			// Health check
			if p.healthEmail != "" && p.healthInterval > 0 {
				if localProcessed > 0 && localProcessed%p.healthInterval == 0 {
					if !p.runHealthCheck() {
						log.Error("WORKER", "Worker %d: Health check failed, pausing...", id)
						atomic.AddInt64(&p.healthFails, 1)
						time.Sleep(30 * time.Second) // Pause on failure
						continue
					}
				}
			}

			// Verify email
			result := p.verifier.Verify(job.Email)

			atomic.AddInt64(&p.processed, 1)
			if result.Status == verifier.StatusError {
				atomic.AddInt64(&p.errors, 1)
			}

			// Send result
			select {
			case p.results <- result:
			case <-p.ctx.Done():
				return
			}

			// Callback
			if p.onResult != nil {
				p.onResult(result)
			}

			localProcessed++

			// Rate limiting with jitter
			p.rateLimitDelay()

		case <-p.ctx.Done():
			log.Detail("WORKER", "Worker %d cancelled", id)
			return
		}
	}
}

// rateLimitDelay applies delay with jitter
func (p *Pool) rateLimitDelay() {
	if p.delay <= 0 {
		return
	}

	delay := p.delay
	if p.jitter > 0 {
		jitter := time.Duration(rand.Int63n(int64(p.jitter)))
		delay += jitter
	}

	time.Sleep(delay)
}

// runHealthCheck verifies the health email
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

// ProcessEmails processes a list of emails and returns results
func (p *Pool) ProcessEmails(emails []string) []*verifier.Result {
	results := make([]*verifier.Result, 0, len(emails))
	resultMap := make(map[string]*verifier.Result)
	var mu sync.Mutex

	// Start result collector
	done := make(chan struct{})
	go func() {
		for result := range p.results {
			mu.Lock()
			resultMap[result.Email] = result
			mu.Unlock()
		}
		close(done)
	}()

	// Submit all jobs
	for i, email := range emails {
		p.Submit(email, i)
	}

	// Close and wait
	p.Close()
	<-done

	// Order results
	for _, email := range emails {
		if result, ok := resultMap[email]; ok {
			results = append(results, result)
		}
	}

	return results
}

// Stats holds pool statistics
type Stats struct {
	Processed   int64
	Errors      int64
	HealthFails int64
	Duration    time.Duration
	Rate        float64 // emails per second
}

// GetStats returns current statistics
func (p *Pool) GetStats(startTime time.Time) *Stats {
	processed := p.Processed()
	duration := time.Since(startTime)

	rate := float64(0)
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
