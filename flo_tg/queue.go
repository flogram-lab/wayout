package main

// A basic queue is synchronous operations pool we use to ensure that at a given time moment only one data read/write operation is exec.
// This needed by design , since there is no need in async operations in this project.
// RPC request handlers and telegram message handlers both end up in a shared queue of operations.

import (
	"context"
)

type Op func(context.Context)

type Queue struct {
	ctx    context.Context
	cancel context.CancelFunc
	op     chan Op
}

func NewQueue(backlog int) *Queue {
	return &Queue{
		op: make(chan Op, backlog),
	}
}

func (q *Queue) Initialize(ctx context.Context) {
	q.ctx, q.cancel = context.WithCancel(ctx)
}

func (q *Queue) Enqueue(op Op) {
	q.op <- op
}

func (q *Queue) Terminate() {
	q.cancel()
	close(q.op)
	q.ctx = nil
	q.cancel = nil
}

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

func (q *Queue) IsReady() bool {
	return q.ctx != nil && q.cancel != nil && q.op != nil
}

func (q *Queue) EnqueueAndWait(op Op) {
	c := make(chan bool)
	defer close(c)

	q.op <- func(ctx context.Context) {
		op(ctx)
		c <- true
	}

	<- c
}