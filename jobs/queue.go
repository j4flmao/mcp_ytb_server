package jobs

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"video-pipeline-mcp/config"
)

type JobStatus string

const (
	StatusQueued    JobStatus = "QUEUED"
	StatusRunning   JobStatus = "RUNNING"
	StatusDone      JobStatus = "DONE"
	StatusFailed    JobStatus = "FAILED"
	StatusCancelled JobStatus = "CANCELLED"
)

type Job struct {
	ID        string    `json:"id"`
	Tool      string    `json:"tool"`
	Status    JobStatus `json:"status"`
	Progress  string    `json:"progress"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	cancel    context.CancelFunc
}

type Queue struct {
	mu   sync.RWMutex
	jobs map[string]*Job
	sem  chan struct{}
	cfg  *config.Config
}

func NewQueue(cfg *config.Config) *Queue {
	maxC := cfg.MaxConcurrent
	if maxC < 4 {
		maxC = 4
	}
	return &Queue{
		jobs: make(map[string]*Job),
		sem:  make(chan struct{}, maxC),
		cfg:  cfg,
	}
}

func newID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return fmt.Sprintf("job_%s", string(b))
}

// Submit queues a new job and runs it in the background.
// Returns the job ID immediately.
func (q *Queue) Submit(tool string, fn func(ctx context.Context, job *Job) (string, error)) string {
	id := newID()
	ctx, cancel := context.WithCancel(context.Background())
	job := &Job{
		ID:        id,
		Tool:      tool,
		Status:    StatusQueued,
		Progress:  "0%",
		CreatedAt: time.Now(),
		cancel:    cancel,
	}

	q.mu.Lock()
	q.jobs[id] = job
	q.mu.Unlock()

	go q.run(ctx, job, fn)

	return id
}

func (q *Queue) run(ctx context.Context, job *Job, fn func(ctx context.Context, job *Job) (string, error)) {
	// Wait for semaphore slot
	select {
	case q.sem <- struct{}{}:
	case <-ctx.Done():
		q.mu.Lock()
		job.Status = StatusCancelled
		q.mu.Unlock()
		return
	}

	// ALWAYS release semaphore, even on panic
	defer func() {
		<-q.sem
		if r := recover(); r != nil {
			q.mu.Lock()
			job.Status = StatusFailed
			job.Error = fmt.Sprintf("panic: %v", r)
			q.mu.Unlock()
		}
	}()

	q.mu.Lock()
	job.Status = StatusRunning
	job.Progress = "0%"
	q.mu.Unlock()

	// Add timeout so jobs can't block forever (30 min max)
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 30*time.Minute)
	defer timeoutCancel()

	output, err := fn(timeoutCtx, job)

	q.mu.Lock()
	defer q.mu.Unlock()

	if ctx.Err() != nil {
		job.Status = StatusCancelled
		return
	}
	if timeoutCtx.Err() == context.DeadlineExceeded {
		job.Status = StatusFailed
		job.Error = "job timed out after 30 minutes"
		return
	}
	if err != nil {
		job.Status = StatusFailed
		job.Error = err.Error()
		return
	}

	job.Status = StatusDone
	job.Progress = "100%"
	job.Output = output
}

// SetProgress updates a job's progress (thread-safe).
func (q *Queue) SetProgress(id, progress string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if job, ok := q.jobs[id]; ok {
		job.Progress = progress
	}
}

// Get returns a copy of a job by ID.
func (q *Queue) Get(id string) (*Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	job, ok := q.jobs[id]
	if !ok {
		return nil, false
	}
	cp := *job
	return &cp, true
}

// List returns all jobs.
func (q *Queue) List() []*Job {
	q.mu.RLock()
	defer q.mu.RUnlock()
	result := make([]*Job, 0, len(q.jobs))
	for _, job := range q.jobs {
		cp := *job
		result = append(result, &cp)
	}
	return result
}

// Cancel cancels a job by ID.
func (q *Queue) Cancel(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	job, ok := q.jobs[id]
	if !ok {
		return false
	}
	if job.Status == StatusQueued || job.Status == StatusRunning {
		job.cancel()
		job.Status = StatusCancelled
		return true
	}
	return false
}

// ClearDone removes all completed/failed/cancelled jobs from memory.
func (q *Queue) ClearDone() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	count := 0
	for id, job := range q.jobs {
		if job.Status == StatusDone || job.Status == StatusFailed || job.Status == StatusCancelled {
			delete(q.jobs, id)
			count++
		}
	}
	return count
}
