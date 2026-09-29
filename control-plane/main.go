package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
)

type deployReq struct {
    Tenant string `json:"tenant"`
    Image  string `json:"image"`
}

func main() {
    dsn := os.Getenv("DATABASE_URL")
    if dsn != "" {
        if err := InitDB(dsn); err != nil {
            log.Printf("warning: could not connect to DB: %v", err)
        } else {
            if err := EnsureMigrations(); err != nil {
                log.Printf("warning: could not ensure migrations: %v", err)
            }
        }
    }

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "infraX control plane (skeleton)")
    })

    // Simple endpoint to record a deployment (tenant + image) into Postgres if configured
    http.HandleFunc("/deploy", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        var req deployReq
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }
        if req.Tenant == "" || req.Image == "" {
            http.Error(w, "missing fields", http.StatusBadRequest)
            return
        }
        if DB != nil {
            if err := RecordDeployment(req.Tenant, req.Image); err != nil {
                log.Printf("db insert error: %v", err)
                http.Error(w, "internal", http.StatusInternalServerError)
                return
            }
        }
        w.WriteHeader(http.StatusAccepted)
    })

    log.Println("control plane listening :9090")
    log.Fatal(http.ListenAndServe(":9090", nil))
}

