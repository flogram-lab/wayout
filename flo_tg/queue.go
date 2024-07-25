package main

import (
	"context"
	"time"
)

// Context passed to the operation func will tell it is cancelled if queue is stopping
type Op func(context.Context)

// Queue is synchronous operations pool used to ensure that at a given time moment only one database read/write operation is exec.
// There is no need in async operations in this project.
// RPC request handlers and telegram message handlers both end up in a shared queue of operations.
// A minimum level of consistency is then guaranteed.
type Queue struct {
	ctx    context.Context
	cancel context.CancelFunc
	op     chan Op
}

// Makes new Queue (unintialized)
// Without Initialize, Enqueue takes up to to [backlog] operations before blocked.
// [backlog] defines number of operations pre-scheduled (pending) in queue, a non-zero value will lead to losing some if queue is Stopped
func NewQueue(backlog int) *Queue {
	return &Queue{
		op: make(chan Op, backlog),
	}
}

// Create queue context (cancellable) for Run() goroutine
// Initializing queue must be followed by spawning Run() goroutine.
func (q *Queue) Initialize(ctx context.Context) {
	q.ctx, q.cancel = context.WithCancel(ctx)
}

// IsReady tests if queue is intiailized and was not stopped
func (q *Queue) IsReady() bool {
	return q.ctx != nil && q.cancel != nil && q.op != nil
}

// Stop iteration inside Run() loop, preventing executing further queued operations.
// Pending operations on queue are lost (if non-zero backlog used)
// Some operations including running one will not be interrupted and will proceed even after call.
// Context passed to the operation func will tell it is cancelled if queue is stopping
// TODO: block before Run() is exited?
func (q *Queue) Stop() {
	q.cancel()
	close(q.op)
	q.ctx = nil
	q.cancel = nil
}

// Goroutine that performs all future operations in order.
func (q *Queue) Run() {
	for {
		select {
		case op := <-q.op:
			if op == nil { // normally channel termination
				return
			}
			op(q.ctx)
		case <-q.ctx.Done():
			return
		}
	}
}

// Enqueue operation.
// May block if queue blocking (is full)
func (q *Queue) Enqueue(op Op) {
	q.op <- op
}

// Enqueue operation and wait before it is done.
// This function may block for unlimited time.
func (q *Queue) EnqueueAndWait(op Op) {
	c := make(chan bool)
	defer close(c)

	q.op <- func(ctx context.Context) {
		op(ctx)
		c <- true
	}

	<-c
}

// Enqueue operation and wait before it is done (blocking) in order
// Cancelled if queue waiting time was longer than the startTimeout
// Cancelled if context is cancelled
// Returns false only on startTimeout or if context was cancelled before enqueued.
func (q *Queue) Join(ctx context.Context, startTimeout time.Duration, op Op) bool {
	c := make(chan bool)
	defer close(c)

	started := time.Now()

	// TODO: add select, ctx cancellation detected, and timeout using Ticker.

	q.op <- func(ctx context.Context) {
		if time.Since(started) >= startTimeout {
			c <- false
		} else {
			op(ctx)
			c <- true
		}
	}

	return <-c
}
