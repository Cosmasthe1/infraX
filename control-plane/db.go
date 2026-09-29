package main

import (
    "database/sql"
    _ "github.com/lib/pq"
    "log"
)

var DB *sql.DB

func InitDB(dsn string) error {
    var err error
    DB, err = sql.Open("postgres", dsn)
    if err != nil {
        return err
    }
    return DB.Ping()
}

func EnsureMigrations() error {
    // Lightweight migration: create deployments table if not exists
    _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS deployments (
        id SERIAL PRIMARY KEY,
        tenant TEXT NOT NULL,
        image TEXT NOT NULL,
        created_at TIMESTAMPTZ DEFAULT now()
    );`)
    if err != nil {
        log.Printf("migration error: %v", err)
    }
    return err
}

func RecordDeployment(tenant, image string) error {
    _, err := DB.Exec(`INSERT INTO deployments (tenant, image) VALUES ($1, $2)`, tenant, image)
    return err
}
