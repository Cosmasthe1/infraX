package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

type deployReq struct {
	Tenant string `json:"tenant"`
	Image  string `json:"image"`
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func main() {
	logger := newLogger()
	dsn := os.Getenv("DATABASE_URL")
	if dsn != "" {
		if err := InitDB(dsn); err != nil {
			logger.Warn("database unavailable; continuing without persistence", "err", err)
		} else {
			if err := EnsureMigrations(); err != nil {
				logger.Warn("migration check failed", "err", err)
			}
		}
	}

	srv := startServer(":9090", logger)
	defer func() {
		if err := srv.Shutdown(context.Background()); err != nil {
			logger.Error("shutdown failed", "err", err)
		}
	}()
	select {}
}

func startServer(addr string, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "infraX control plane (skeleton)")
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
				logger.Error("deployment insert failed", "tenant", req.Tenant, "image", req.Image, "err", err)
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
		var payload map[string]any
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
					logger.Error("webhook deployment insert failed", "tenant", tenant, "image", img, "err", err)
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
		logger.Info("control plane listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "addr", addr, "err", err)
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
