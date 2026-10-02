package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func makeMuxForTest() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("infraX control plane (skeleton)\n"))
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
		// DB may be nil in unit tests; handlers are expected to accept that.
		if DB != nil {
			_ = RecordDeployment(req.Tenant, req.Image)
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
				_ = RecordDeployment(tenant, img)
			}
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))

	return mux
}

func TestRootAndDeployHandlers(t *testing.T) {
	// ensure DB is nil for unit-level semantics
	DB = nil

	// ensure auth wrapper captures the env at registration time
	os.Setenv("ADMIN_API_KEY", "testkey")
	defer os.Unsetenv("ADMIN_API_KEY")

	mux := makeMuxForTest()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// root
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// without API key should be unauthorized when ADMIN_API_KEY is set
	os.Setenv("ADMIN_API_KEY", "testkey")
	defer os.Unsetenv("ADMIN_API_KEY")

	// request without header -> 401
	payload := map[string]string{"tenant": "t", "image": "i"}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/deploy", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /deploy failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized without key, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// with key and valid body -> 202
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/deploy", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "testkey")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /deploy with key failed: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 accepted, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWebhookHandlerDefaultsTenant(t *testing.T) {
	DB = nil
	os.Setenv("ADMIN_API_KEY", "wkey")
	defer os.Unsetenv("ADMIN_API_KEY")
	mux := makeMuxForTest()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	payload := map[string]string{"image": "img:1"}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "wkey")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /webhook failed: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 for webhook, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestValidateDeployRequest(t *testing.T) {
	if err := validateDeployRequest(deployReq{Tenant: "team-a", Image: "ghcr.io/acme/app:1.2.3"}); err != nil {
		t.Fatalf("expected valid deploy request, got err: %v", err)
	}

	if err := validateDeployRequest(deployReq{Tenant: "", Image: "example/app:latest"}); err == nil {
		t.Fatal("expected tenant validation error")
	}

	if err := validateDeployRequest(deployReq{Tenant: "team-a", Image: ""}); err == nil {
		t.Fatal("expected image validation error")
	}
}

func TestPlatformHealthAndDeploymentListing(t *testing.T) {
	DB = nil
	os.Unsetenv("ADMIN_API_KEY")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := newServerMux(logger)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /healthz, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(ts.URL + "/deployments")
	if err != nil {
		t.Fatalf("GET /deployments failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /deployments, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestRenderDeploymentManifest(t *testing.T) {
	manifest := RenderDeploymentManifest("ghcr.io/acme/app:1.2.3", "production")
	if !strings.Contains(manifest, "kind: Deployment") {
		t.Fatal("expected a Kubernetes Deployment manifest")
	}
	if !strings.Contains(manifest, "ghcr.io/acme/app:1.2.3") {
		t.Fatal("expected manifest to include the target image")
	}
	if !strings.Contains(manifest, "namespace: production") {
		t.Fatal("expected manifest to include the target namespace")
	}
}
