package main

import (
    "bytes"
    "database/sql"
    "encoding/json"
    "net/http"
    "os"
    "testing"
    "time"

    _ "github.com/lib/pq"
)

func TestDeployEndpointRecordsToDB(t *testing.T) {
    // Expect a Postgres instance available via DATABASE_URL
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        t.Skip("DATABASE_URL not set; skipping integration test")
    }
    // Set API key for auth
    os.Setenv("ADMIN_API_KEY", "testkey")

    // Start the control plane in a goroutine
    go main()

    // Wait for server to start
    deadline := time.Now().Add(10 * time.Second)
    for time.Now().Before(deadline) {
        resp, err := http.Get("http://localhost:9090/")
        if err == nil && resp.StatusCode == http.StatusOK {
            break
        }
        time.Sleep(200 * time.Millisecond)
    }

    // Post a deployment
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

    // Verify DB record
    db, err := sql.Open("postgres", dbURL)
    if err != nil {
        t.Fatalf("db open: %v", err)
    }
    defer db.Close()

    var id int
    var tenant, image string
    var created time.Time
    deadline = time.Now().Add(5 * time.Second)
    for time.Now().Before(deadline) {
        err = db.QueryRow("SELECT id, tenant, image, created_at FROM deployments WHERE tenant=$1 ORDER BY id DESC LIMIT 1", "itest").Scan(&id, &tenant, &image, &created)
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
