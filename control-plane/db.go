package main

import (
	"database/sql"
	"fmt"
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
	return recordDeployment(DB, tenant, image)
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
