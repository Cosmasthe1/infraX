package workers
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
