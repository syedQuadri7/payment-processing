// Package webhook provides webhook delivery for the provider simulator.
package webhook

import (
	"sync"
	"time"

	"provider-simulator/internal/state"
)

// WebhookJob represents a job in the delivery queue.
type WebhookJob struct {
	Webhook     *state.WebhookRecord
	ScheduledAt time.Time
	Priority    int // Higher = more urgent
}

// Queue is a priority queue for webhook delivery.
type Queue struct {
	mu    sync.Mutex
	jobs  []*WebhookJob
	cond  *sync.Cond
	closed bool
}

// NewQueue creates a new webhook queue.
func NewQueue() *Queue {
	q := &Queue{
		jobs: make([]*WebhookJob, 0),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Push adds a job to the queue.
func (q *Queue) Push(job *WebhookJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	// Insert in sorted order by scheduled time
	inserted := false
	for i, j := range q.jobs {
		if job.ScheduledAt.Before(j.ScheduledAt) {
			q.jobs = append(q.jobs[:i], append([]*WebhookJob{job}, q.jobs[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		q.jobs = append(q.jobs, job)
	}

	q.cond.Signal()
}

// Pop removes and returns the next job, blocking until one is available.
// Returns nil if the queue is closed.
func (q *Queue) Pop() *WebhookJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.jobs) == 0 && !q.closed {
		q.cond.Wait()
	}

	if q.closed && len(q.jobs) == 0 {
		return nil
	}

	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job
}

// PopReady removes and returns the next job that's ready to execute.
// Returns nil if no job is ready or queue is closed.
// This is non-blocking - returns immediately.
func (q *Queue) PopReady() *WebhookJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) == 0 || q.closed {
		return nil
	}

	now := time.Now()
	if q.jobs[0].ScheduledAt.After(now) {
		return nil
	}

	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job
}

// Peek returns the next job without removing it.
func (q *Queue) Peek() *WebhookJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) == 0 {
		return nil
	}
	return q.jobs[0]
}

// Len returns the number of jobs in the queue.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

// Close signals the queue to stop accepting jobs and unblocks any waiting Pop calls.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}
