package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Queue provides jobs to the Poller. Next returns (job, ok, err).
type DeploymentJob struct {
	ID           string `json:"id"`
	DeploymentID int64  `json:"deployment_id"`
	Tenant       string `json:"tenant"`
	Image        string `json:"image"`
	Attempts     int    `json:"attempts"`
	MaxRetries   int    `json:"max_retries"`
	Status       string `json:"status"`
	LastError    string `json:"last_error,omitempty"`
}

func processDeploymentJob(job DeploymentJob, execute func(string) error) error {
	if job.MaxRetries <= 0 {
		job.MaxRetries = 3
	}
	if job.Attempts >= job.MaxRetries {
		return fmt.Errorf("deployment %s exhausted retries (%d/%d)", job.ID, job.Attempts, job.MaxRetries)
	}
	if err := execute(job.Image); err != nil {
		job.Attempts++
		if job.Attempts >= job.MaxRetries {
			return fmt.Errorf("deployment %s failed after %d attempts: %w", job.ID, job.Attempts, err)
		}
		return nil
	}
	return nil
}

type Queue interface {
	Next() (string, bool, error)
}

type HTTPQueue struct {
	BaseURL string
	Client  *http.Client
}

func (q *HTTPQueue) Next() (string, bool, error) {
	if q == nil || q.BaseURL == "" {
		return "", false, nil
	}
	client := q.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	url := strings.TrimRight(q.BaseURL, "/") + "/jobs"
	resp, err := client.Get(url)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return "", false, nil
	}
	if resp.StatusCode >= 400 {
		return "", false, fmt.Errorf("queue request failed: %s", resp.Status)
	}
	var job DeploymentJob
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return "", false, err
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return "", false, err
	}
	return string(payload), true, nil
}

// MemoryQueue is a simple in-memory job queue suitable for local and test deployments.
type MemoryQueue struct {
	mu   sync.Mutex
	jobs []string
}

func (q *MemoryQueue) Enqueue(job string) {
	if job == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, job)
}

func (q *MemoryQueue) Next() (string, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) == 0 {
		return "", false, nil
	}
	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job, true, nil
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
