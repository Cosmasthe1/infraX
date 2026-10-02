package main

import (
	"context"
	"errors"
	"strings"
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

func TestProcessDeploymentJobRetryPolicy(t *testing.T) {
	job := DeploymentJob{ID: "job-1", DeploymentID: 7, Tenant: "team-a", Image: "example/app:1", Attempts: 1, MaxRetries: 3}
	err := processDeploymentJob(job, func(s string) error { return nil })
	if err != nil {
		t.Fatalf("expected successful processDeploymentJob, got %v", err)
	}

	job.Attempts = 3
	job.MaxRetries = 3
	err = processDeploymentJob(job, func(s string) error { return errors.New("boom") })
	if err == nil {
		t.Fatal("expected failure when retries are exhausted")
	}
}

func TestRenderReleaseManifest(t *testing.T) {
	manifest := renderReleaseManifest("ghcr.io/acme/app:1.2.3", "team-a")
	if manifest == "" {
		t.Fatal("expected rendered manifest content")
	}
	if !strings.Contains(manifest, "kind: Deployment") {
		t.Fatal("expected deployment manifest")
	}
	if !strings.Contains(manifest, "ghcr.io/acme/app:1.2.3") {
		t.Fatal("expected image in manifest")
	}
	if !strings.Contains(manifest, "namespace: team-a") {
		t.Fatal("expected namespace in manifest")
	}
}

func TestReleaseFlowRollbackOnApplyFailure(t *testing.T) {
	calls := []string{}
	runner := func(args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		if len(args) >= 2 && args[0] == "apply" {
			return nil, errors.New("apply failed")
		}
		return []byte("ok"), nil
	}

	if err := runReleaseWithRunner(runner, "ghcr.io/acme/app:1.2.3", "team-a"); err == nil {
		t.Fatal("expected apply failure to be returned")
	}
	if len(calls) == 0 || !contains(calls, "rollout undo") {
		t.Fatal("expected rollback attempt after apply failure")
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}
