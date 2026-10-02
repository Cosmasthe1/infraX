package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeQueue struct {
	mu   sync.Mutex
	jobs []string
}

func (f *fakeQueue) Next() (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.jobs) == 0 {
		return "", false, nil
	}
	j := f.jobs[0]
	f.jobs = f.jobs[1:]
	return j, true, nil
}

func TestPollerProcessesJobs(t *testing.T) {
	fq := &fakeQueue{jobs: []string{"a", "b", "c"}}
	var mu sync.Mutex
	processed := []string{}

	p := &Poller{
		Interval: 10 * time.Millisecond,
		Queue:    fq,
		Process: func(job string) error {
			mu.Lock()
			defer mu.Unlock()
			processed = append(processed, job)
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() {
		_ = p.Run(ctx)
	}()

	// wait for processing to complete or timeout
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 3 {
		t.Fatalf("expected 3 processed jobs, got %d: %v", len(processed), processed)
	}
}

func TestMemoryQueueEnqueueAndNext(t *testing.T) {
	q := &MemoryQueue{}
	q.Enqueue("job-1")
	q.Enqueue("job-2")

	job, ok, err := q.Next()
	if err != nil {
		t.Fatalf("Next returned unexpected error: %v", err)
	}
	if !ok || job != "job-1" {
		t.Fatalf("expected first job job-1, got %q, ok=%v", job, ok)
	}

	job, ok, err = q.Next()
	if err != nil {
		t.Fatalf("Next returned unexpected error: %v", err)
	}
	if !ok || job != "job-2" {
		t.Fatalf("expected second job job-2, got %q, ok=%v", job, ok)
	}

	job, ok, err = q.Next()
	if err != nil {
		t.Fatalf("Next returned unexpected error: %v", err)
	}
	if ok || job != "" {
		t.Fatalf("expected empty queue to return ok=false, got job=%q ok=%v", job, ok)
	}
}
