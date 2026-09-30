//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestDeployEndpointRecordsToDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	os.Setenv("ADMIN_API_KEY", "testkey")

	if err := InitDB(dbURL); err != nil {
		t.Fatalf("db init: %v", err)
	}
	if err := EnsureMigrations(); err != nil {
		t.Fatalf("db migration: %v", err)
	}

	logger := newLogger()
	srv := startServer(":9090", logger)
	t.Cleanup(func() {
		if err := srv.Shutdown(context.Background()); err != nil {
			t.Logf("shutdown error: %v", err)
		}
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://localhost:9090/")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	payload := map[string]string{"tenant": "itest", "image": "registry/test:ci"}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:9090/deploy", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "testkey")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /deploy failed: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 accepted, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	defer db.Close()

	var tenant, image string
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		err = db.QueryRow("SELECT tenant, image FROM deployments WHERE tenant=$1 ORDER BY id DESC LIMIT 1", "itest").Scan(&tenant, &image)
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("expected record in DB, query error: %v", err)
	}
	if tenant != "itest" || image != "registry/test:ci" {
		t.Fatalf("unexpected row: %v %v", tenant, image)
	}
}
