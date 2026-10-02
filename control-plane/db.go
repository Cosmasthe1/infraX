package main

import (
	"database/sql"
	"encoding/json"
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
	ID          int64            `json:"id"`
	Tenant      string           `json:"tenant"`
	Image       string           `json:"image"`
	Namespace   string           `json:"namespace,omitempty"`
	Environment string           `json:"environment,omitempty"`
	Status      DeploymentStatus `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
}

type DeploymentJob struct {
	ID           int64             `json:"id"`
	DeploymentID int64             `json:"deployment_id"`
	Tenant       string            `json:"tenant"`
	Image        string            `json:"image"`
	Namespace    string            `json:"namespace,omitempty"`
	Environment  string            `json:"environment,omitempty"`
	Config       map[string]string `json:"config,omitempty"`
	Status       DeploymentStatus  `json:"status"`
	Attempts     int               `json:"attempts"`
	MaxRetries   int               `json:"max_retries"`
	LastError    string            `json:"last_error,omitempty"`
	EnqueuedAt   time.Time         `json:"enqueued_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	NextRetryAt  *time.Time        `json:"next_retry_at,omitempty"`
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
        namespace TEXT,
        environment TEXT DEFAULT 'dev',
        status TEXT NOT NULL DEFAULT 'queued',
        created_at TIMESTAMPTZ DEFAULT now()
    );`)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`ALTER TABLE deployments ADD COLUMN IF NOT EXISTS namespace TEXT`)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`ALTER TABLE deployments ADD COLUMN IF NOT EXISTS environment TEXT DEFAULT 'dev'`)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS deployment_jobs (
        id SERIAL PRIMARY KEY,
        deployment_id INTEGER NOT NULL UNIQUE REFERENCES deployments(id) ON DELETE CASCADE,
        tenant TEXT NOT NULL,
        image TEXT NOT NULL,
        namespace TEXT,
        environment TEXT DEFAULT 'dev',
        config JSONB NOT NULL DEFAULT '{}'::jsonb,
        status TEXT NOT NULL DEFAULT 'queued',
        attempts INTEGER NOT NULL DEFAULT 0,
        max_retries INTEGER NOT NULL DEFAULT 3,
        last_error TEXT,
        enqueued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        next_retry_at TIMESTAMPTZ
    );`)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`ALTER TABLE deployment_jobs ADD COLUMN IF NOT EXISTS namespace TEXT`)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`ALTER TABLE deployment_jobs ADD COLUMN IF NOT EXISTS environment TEXT DEFAULT 'dev'`)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`ALTER TABLE deployment_jobs ADD COLUMN IF NOT EXISTS config JSONB NOT NULL DEFAULT '{}'::jsonb`)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`CREATE INDEX IF NOT EXISTS idx_deployment_jobs_status_retry ON deployment_jobs(status, next_retry_at, enqueued_at)`)
	return err
}

func RecordDeployment(tenant, image string) error {
	_, err := CreateDeployment(tenant, image)
	return err
}

func CreateDeployment(tenant, image string) (int64, error) {
	return createDeployment(DB, tenant, image)
}

func CreateDeploymentJob(deploymentID int64, tenant, image string) (DeploymentJob, error) {
	return createDeploymentJob(DB, deploymentID, tenant, image, "", "dev", nil)
}

func CreateDeploymentJobWithMetadata(deploymentID int64, tenant, image, namespace, environment string, config map[string]string) (DeploymentJob, error) {
	return createDeploymentJob(DB, deploymentID, tenant, image, namespace, environment, config)
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
	job, err := createDeploymentJob(exec, id, tenant, image, tenant, "dev", nil)
	if err != nil {
		return 0, err
	}
	_ = job
	return id, nil
}

func createDeploymentJob(exec dbExecutor, deploymentID int64, tenant, image, namespace, environment string, config map[string]string) (DeploymentJob, error) {
	if exec == nil {
		return DeploymentJob{}, fmt.Errorf("database not initialized")
	}
	if namespace == "" {
		namespace = tenant
	}
	if environment == "" {
		environment = "dev"
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return DeploymentJob{}, err
	}
	var job DeploymentJob
	err = exec.QueryRow(`
		INSERT INTO deployment_jobs (deployment_id, tenant, image, namespace, environment, config, status, attempts, max_retries, last_error, enqueued_at, updated_at, next_retry_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'queued', 0, 3, NULL, NOW(), NOW(), NULL)
		RETURNING id, deployment_id, tenant, image, namespace, environment, config, status, attempts, max_retries, last_error, enqueued_at, updated_at, next_retry_at
	`, deploymentID, tenant, image, namespace, environment, string(configJSON)).Scan(
		&job.ID, &job.DeploymentID, &job.Tenant, &job.Image, &job.Namespace, &job.Environment, &job.Config, &job.Status, &job.Attempts, &job.MaxRetries, &job.LastError, &job.EnqueuedAt, &job.UpdatedAt, &job.NextRetryAt,
	)
	if err != nil {
		return DeploymentJob{}, err
	}
	if job.Config == nil {
		job.Config = map[string]string{}
	}
	return job, nil
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

func UpdateDeploymentJobStatus(jobID int64, status DeploymentStatus, attempts int, lastErr string) error {
	return updateDeploymentJobStatus(DB, jobID, status, attempts, lastErr)
}

func UpdateDeploymentJobStatusByDeploymentID(deploymentID int64, status DeploymentStatus, attempts int, lastErr string) error {
	return updateDeploymentJobStatusByDeploymentID(DB, deploymentID, status, attempts, lastErr)
}

func updateDeploymentJobStatus(exec dbExecutor, jobID int64, status DeploymentStatus, attempts int, lastErr string) error {
	if exec == nil {
		return fmt.Errorf("database not initialized")
	}
	if status == "" {
		status = StatusQueued
	}
	_, err := exec.Exec(`
		UPDATE deployment_jobs
		SET status = $1,
		    attempts = $2,
		    last_error = $3,
		    updated_at = now(),
		    next_retry_at = CASE WHEN $1 = 'queued' THEN now() + interval '5 seconds' ELSE NULL END
		WHERE id = $4
	`, string(status), attempts, lastErr, jobID)
	return err
}

func updateDeploymentJobStatusByDeploymentID(exec dbExecutor, deploymentID int64, status DeploymentStatus, attempts int, lastErr string) error {
	if exec == nil {
		return fmt.Errorf("database not initialized")
	}
	if status == "" {
		status = StatusQueued
	}
	_, err := exec.Exec(`
		UPDATE deployment_jobs
		SET status = $1,
		    attempts = $2,
		    last_error = $3,
		    updated_at = now(),
		    next_retry_at = CASE WHEN $1 = 'queued' THEN now() + interval '5 seconds' ELSE NULL END
		WHERE deployment_id = $4
	`, string(status), attempts, lastErr, deploymentID)
	return err
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

func NextQueuedJob() (DeploymentJob, bool, error) {
	return nextQueuedJob(DB)
}

func nextQueuedJob(exec dbExecutor) (DeploymentJob, bool, error) {
	if exec == nil {
		return DeploymentJob{}, false, nil
	}
	var job DeploymentJob
	var configJSON []byte
	err := exec.QueryRow(`
		SELECT id, deployment_id, tenant, image, namespace, environment, config, status, attempts, max_retries, last_error, enqueued_at, updated_at, next_retry_at
		FROM deployment_jobs
		WHERE status = 'queued' AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		ORDER BY enqueued_at ASC
		LIMIT 1
	`).Scan(
		&job.ID, &job.DeploymentID, &job.Tenant, &job.Image, &job.Namespace, &job.Environment, &configJSON, &job.Status, &job.Attempts, &job.MaxRetries, &job.LastError, &job.EnqueuedAt, &job.UpdatedAt, &job.NextRetryAt,
	)
	if err == sql.ErrNoRows {
		return DeploymentJob{}, false, nil
	}
	if err != nil {
		return DeploymentJob{}, false, err
	}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &job.Config); err != nil {
			return DeploymentJob{}, false, err
		}
	}
	if job.Config == nil {
		job.Config = map[string]string{}
	}
	if job.Namespace == "" {
		job.Namespace = job.Tenant
	}
	if job.Environment == "" {
		job.Environment = "dev"
	}
	if _, err := exec.Exec(`UPDATE deployment_jobs SET status='running', updated_at=NOW() WHERE id=$1`, job.ID); err != nil {
		return DeploymentJob{}, false, err
	}
	job.Status = StatusRunning
	return job, true, nil
}
