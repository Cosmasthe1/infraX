package main

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// Deployment captures the minimal persisted record for a platform deploy request.
type DeploymentStatus string

const (
	StatusQueued    DeploymentStatus = "queued"
	StatusRunning   DeploymentStatus = "running"
	StatusSucceeded DeploymentStatus = "succeeded"
	StatusFailed    DeploymentStatus = "failed"
)

type Deployment struct {
	ID        int64            `json:"id"`
	Tenant    string           `json:"tenant"`
	Image     string           `json:"image"`
	Status    DeploymentStatus `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
}

type DeploymentJob struct {
	ID           int64            `json:"id"`
	DeploymentID int64            `json:"deployment_id"`
	Tenant       string           `json:"tenant"`
	Image        string           `json:"image"`
	Status       DeploymentStatus `json:"status"`
	Attempts     int              `json:"attempts"`
	MaxRetries   int              `json:"max_retries"`
	LastError    string           `json:"last_error,omitempty"`
	EnqueuedAt   time.Time        `json:"enqueued_at"`
}

func (j DeploymentJob) NextState(current DeploymentStatus, err error) (DeploymentStatus, bool) {
	if current == StatusQueued && err == nil {
		return StatusRunning, true
	}
	if current == StatusRunning && err == nil {
		return StatusSucceeded, true
	}
	if current == StatusRunning && err != nil {
		limit := j.MaxRetries
		if limit <= 0 {
			limit = 3
		}
		if j.Attempts >= limit-1 {
			return StatusFailed, true
		}
		return StatusQueued, true
	}
	return "", false
}

type JobQueue struct {
	mu   sync.Mutex
	jobs []DeploymentJob
}

func NewJobQueue() *JobQueue {
	return &JobQueue{}
}

func (q *JobQueue) Enqueue(job DeploymentJob) {
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, job)
}

func (q *JobQueue) Next() (DeploymentJob, bool, error) {
	if q == nil {
		return DeploymentJob{}, false, nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) == 0 {
		return DeploymentJob{}, false, nil
	}
	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job, true, nil
}

var GlobalJobQueue = NewJobQueue()

type dbExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

var DB dbExecutor

func InitDB(dsn string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return err
	}
	DB = db
	return nil
}

func EnsureMigrations() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := DB.Exec(`CREATE TABLE IF NOT EXISTS deployments (
        id SERIAL PRIMARY KEY,
        tenant TEXT NOT NULL,
        image TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'queued',
        created_at TIMESTAMPTZ DEFAULT now()
    );`)
	return err
}

func RecordDeployment(tenant, image string) error {
	_, err := CreateDeployment(tenant, image)
	return err
}

func CreateDeployment(tenant, image string) (int64, error) {
	return createDeployment(DB, tenant, image)
}

func createDeployment(exec dbExecutor, tenant, image string) (int64, error) {
	if exec == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	if tenant == "" || image == "" {
		return 0, fmt.Errorf("tenant and image are required")
	}
	var id int64
	err := exec.QueryRow(`INSERT INTO deployments (tenant, image, status) VALUES ($1, $2, 'queued') RETURNING id`, tenant, image).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func recordDeployment(exec dbExecutor, tenant, image string) error {
	if exec == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := exec.Exec(`INSERT INTO deployments (tenant, image, status) VALUES ($1, $2, 'queued')`, tenant, image)
	return err
}

func UpdateDeploymentStatus(id int64, status DeploymentStatus) error {
	return setDeploymentStatus(DB, id, status)
}

func setDeploymentStatus(exec dbExecutor, id int64, status DeploymentStatus) error {
	if exec == nil {
		return fmt.Errorf("database not initialized")
	}
	if status == "" {
		status = StatusQueued
	}
	_, err := exec.Exec(`UPDATE deployments SET status = $1 WHERE id = $2`, string(status), id)
	return err
}

func ListDeployments() ([]Deployment, error) {
	return listDeployments(DB, 20)
}

func listDeployments(exec dbExecutor, limit int) ([]Deployment, error) {
	if exec == nil {
		return []Deployment{}, nil
	}
	if limit <= 0 {
		limit = 20
	}

	rows, err := exec.Query(`SELECT id, tenant, image, status, created_at FROM deployments ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Deployment, 0, limit)
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.Tenant, &d.Image, &d.Status, &d.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
