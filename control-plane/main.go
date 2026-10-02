package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type deployReq struct {
	Tenant      string            `json:"tenant"`
	Image       string            `json:"image"`
	Namespace   string            `json:"namespace,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
}

var validImageRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@-]*$`)

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

func normalizeDeployRequest(req deployReq) deployReq {
	req.Tenant = strings.TrimSpace(req.Tenant)
	req.Image = strings.TrimSpace(req.Image)
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Environment = strings.TrimSpace(req.Environment)
	if req.Namespace == "" {
		req.Namespace = req.Tenant
	}
	if req.Environment == "" {
		req.Environment = "dev"
	}
	if req.Config == nil {
		req.Config = map[string]string{}
	}
	for k, v := range req.Config {
		req.Config[k] = strings.TrimSpace(v)
	}
	return req
}

func validateDeployRequest(req deployReq) error {
	req = normalizeDeployRequest(req)
	if req.Tenant == "" {
		return errors.New("tenant is required")
	}
	if req.Image == "" {
		return errors.New("image is required")
	}
	if !validImageRef.MatchString(req.Image) {
		return errors.New("image must be a valid container image reference")
	}
	return nil
}

func newServerMux(logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "infraX control plane")
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	mux.HandleFunc("/jobs", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			var job DeploymentJob
			var ok bool
			var err error
			if DB != nil {
				job, ok, err = NextQueuedJob()
			} else {
				job, ok, err = GlobalJobQueue.Next()
			}
			if err != nil {
				logger.Error("next queued job failed", "err", err)
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
			if !ok {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(job)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var job DeploymentJob
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if job.Image == "" || job.Tenant == "" {
			http.Error(w, "tenant and image are required", http.StatusBadRequest)
			return
		}
		if job.MaxRetries <= 0 {
			job.MaxRetries = 3
		}
		job.Status = StatusQueued
		if job.EnqueuedAt.IsZero() {
			job.EnqueuedAt = time.Now()
		}
		if DB != nil {
			persisted, err := CreateDeploymentJob(job.DeploymentID, job.Tenant, job.Image)
			if err != nil {
				logger.Error("persist job failed", "deployment_id", job.DeploymentID, "tenant", job.Tenant, "image", job.Image, "err", err)
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
			job = persisted
		} else {
			GlobalJobQueue.Enqueue(job)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(job)
	}))

	mux.HandleFunc("/deployments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		deployments, err := ListDeployments()
		if err != nil {
			logger.Error("list deployments failed", "err", err)
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deployments)
	})

	mux.HandleFunc("/deployments/", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		idStr := strings.TrimPrefix(r.URL.Path, "/deployments/")
		if idStr == "" || strings.Contains(idStr, "/") {
			http.Error(w, "invalid deployment id", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid deployment id", http.StatusBadRequest)
			return
		}
		var payload struct {
			Status     string `json:"status"`
			Attempts   int    `json:"attempts"`
			LastError  string `json:"last_error"`
			MaxRetries int    `json:"max_retries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		status := DeploymentStatus(strings.TrimSpace(payload.Status))
		if status == "" {
			http.Error(w, "status is required", http.StatusBadRequest)
			return
		}
		if payload.MaxRetries > 0 {
			if err := updateDeploymentJobStatusByDeploymentID(DB, id, status, payload.Attempts, payload.LastError); err != nil {
				logger.Error("deployment job status update failed", "deployment_id", id, "status", status, "err", err)
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
		}
		if err := UpdateDeploymentStatus(id, status); err != nil {
			logger.Error("deployment status update failed", "id", id, "status", status, "err", err)
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "status": status})
	}))

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
		req = normalizeDeployRequest(req)
		if err := validateDeployRequest(req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var deploymentID int64
		if DB != nil {
			id, err := CreateDeployment(req.Tenant, req.Image)
			if err != nil {
				logger.Error("deployment insert failed", "tenant", req.Tenant, "image", req.Image, "err", err)
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
			deploymentID = id
			job := DeploymentJob{
				DeploymentID: id,
				Tenant:       req.Tenant,
				Image:        req.Image,
				Namespace:    req.Namespace,
				Environment:  req.Environment,
				Config:       req.Config,
				Status:       StatusQueued,
				Attempts:     0,
				MaxRetries:   3,
				EnqueuedAt:   time.Now(),
			}
			job, err = CreateDeploymentJobWithMetadata(id, req.Tenant, req.Image, req.Namespace, req.Environment, req.Config)
			if err != nil {
				logger.Error("deployment job insert failed", "tenant", req.Tenant, "image", req.Image, "err", err)
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
			if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
				if err := PublishDeploymentJob(natsURL, job); err != nil {
					logger.Warn("nats publish failed; falling back to queue", "deployment_id", id, "err", err)
				}
			}
			GlobalJobQueue.Enqueue(job)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "accepted", "tenant": req.Tenant, "image": req.Image, "namespace": req.Namespace, "environment": req.Environment, "config": req.Config, "deployment_id": deploymentID})
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
		namespace, _ := payload["namespace"].(string)
		environment, _ := payload["environment"].(string)
		configMap, _ := payload["config"].(map[string]any)
		if img == "" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if tenant == "" {
			tenant = "webhook"
		}
		config := make(map[string]string, len(configMap))
		for k, v := range configMap {
			if s, ok := v.(string); ok {
				config[k] = s
			}
		}
		candidate := normalizeDeployRequest(deployReq{Tenant: tenant, Image: img, Namespace: namespace, Environment: environment, Config: config})
		if err := validateDeployRequest(candidate); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		namespace = candidate.Namespace
		environment = candidate.Environment
		config = candidate.Config
		tenant = candidate.Tenant
		img = candidate.Image
		var deploymentID int64
		if DB != nil {
			id, err := CreateDeployment(tenant, img)
			if err != nil {
				logger.Error("webhook deployment insert failed", "tenant", tenant, "image", img, "err", err)
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
			deploymentID = id
			job := DeploymentJob{
				DeploymentID: id,
				Tenant:       tenant,
				Image:        img,
				Namespace:    namespace,
				Environment:  environment,
				Config:       config,
				Status:       StatusQueued,
				Attempts:     0,
				MaxRetries:   3,
				EnqueuedAt:   time.Now(),
			}
			job, err = CreateDeploymentJobWithMetadata(id, tenant, img, namespace, environment, config)
			if err != nil {
				logger.Error("webhook deployment job insert failed", "tenant", tenant, "image", img, "err", err)
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
			if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
				if err := PublishDeploymentJob(natsURL, job); err != nil {
					logger.Warn("nats publish failed; falling back to queue", "deployment_id", id, "err", err)
				}
			}
			GlobalJobQueue.Enqueue(job)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "accepted", "tenant": tenant, "image": img, "namespace": namespace, "environment": environment, "config": config, "deployment_id": deploymentID})
	}))

	return mux
}

func startServer(addr string, logger *slog.Logger) *http.Server {
	mux := newServerMux(logger)

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
