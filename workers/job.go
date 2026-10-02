package main

import (
	"context"
	"time"
)

// Queue provides jobs to the Poller. Next returns (job, ok, err).
type Queue interface {
	Next() (string, bool, error)
}

// Poller polls a Queue and invokes Process for each job.
type Poller struct {
	Interval time.Duration
	Queue    Queue
	Process  func(job string) error
}

// Run starts polling until the context is cancelled.
func (p *Poller) Run(ctx context.Context) error {
	if p.Interval <= 0 {
		p.Interval = 1 * time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		job, ok, err := p.Queue.Next()
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(p.Interval):
				continue
			}
		}
		if !ok {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(p.Interval):
				continue
			}
		}
		if p.Process != nil {
			_ = p.Process(job)
		}
	}
}
