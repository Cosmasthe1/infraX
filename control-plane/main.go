package main

import (
    "context"
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

    srv := startServer(":9090")
    defer func() {
        if err := srv.Shutdown(context.Background()); err != nil {
            log.Printf("shutdown error: %v", err)
        }
    }()
    select {}
}

func startServer(addr string) *http.Server {
    mux := http.NewServeMux()

    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "infraX control plane (skeleton)")
    })

    mux.HandleFunc("/deploy", withAuth(func(w http.ResponseWriter, r *http.Request) {
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
    }))

    mux.HandleFunc("/webhook", withAuth(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        var payload map[string]interface{}
        if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }
        img, _ := payload["image"].(string)
        tenant, _ := payload["tenant"].(string)
        if img != "" {
            if tenant == "" {
                tenant = "webhook"
            }
            if DB != nil {
                if err := RecordDeployment(tenant, img); err != nil {
                    log.Printf("db insert error (webhook): %v", err)
                    http.Error(w, "internal", http.StatusInternalServerError)
                    return
                }
            }
            w.WriteHeader(http.StatusAccepted)
            return
        }
        w.WriteHeader(http.StatusAccepted)
    }))

    srv := &http.Server{Addr: addr, Handler: mux}
    go func() {
        log.Printf("control plane listening %s", addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Printf("server error: %v", err)
        }
    }()
    return srv
}

// withAuth wraps an http.HandlerFunc and enforces ADMIN_API_KEY when set
func withAuth(h http.HandlerFunc) http.HandlerFunc {
    apiKey := os.Getenv("ADMIN_API_KEY")
    if apiKey == "" {
        return h
    }
    return func(w http.ResponseWriter, r *http.Request) {
        got := r.Header.Get("X-API-Key")
        if got == "" || got != apiKey {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        h(w, r)
    }
}

