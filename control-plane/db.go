package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type dbExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
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
	_, err := exec.Exec(`INSERT INTO deployments (tenant, image) VALUES ($1, $2)`, tenant, image)
	return err
}
