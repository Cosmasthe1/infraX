package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ReleaseConfig struct {
	Namespace   string
	AppName     string
	Image       string
	Environment string
	Replicas    int
	Port        int
	Config      map[string]string
}

type kubectlRunner func(args ...string) ([]byte, error)

type releaseExecutor struct {
	runner kubectlRunner
}

func clusterAuthArgs() []string {
	if kubeconfig := strings.TrimSpace(os.Getenv("KUBECONFIG")); kubeconfig != "" {
		return []string{"--kubeconfig", kubeconfig}
	}
	args := []string{}
	if server := strings.TrimSpace(os.Getenv("KUBE_SERVER")); server != "" {
		args = append(args, "--server", server)
	}
	if host := strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_HOST")); host != "" {
		port := strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_PORT"))
		if port == "" {
			port = "443"
		}
		if !containsArg(args, "--server") {
			args = append(args, "--server", "https://"+host+":"+port)
		}
	}
	if token := strings.TrimSpace(os.Getenv("KUBE_TOKEN")); token != "" {
		args = append(args, "--token", token)
	}
	if ca := strings.TrimSpace(os.Getenv("KUBE_CA_CERT")); ca != "" {
		args = append(args, "--certificate-authority", ca)
	}
	return args
}

func containsArg(args []string, want string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == want {
			return true
		}
	}
	return false
}

func newDefaultExecutor() *releaseExecutor {
	return &releaseExecutor{runner: func(args ...string) ([]byte, error) {
		cmdArgs := append(clusterAuthArgs(), args...)
		cmd := exec.Command("kubectl", cmdArgs...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return out.Bytes(), err
		}
		return out.Bytes(), nil
	}}
}

func buildReleaseConfig(image, namespace string) ReleaseConfig {
	cfg := ReleaseConfig{
		Namespace:   normalizeNamespace(namespace),
		AppName:     strings.TrimSpace(os.Getenv("APP_NAME")),
		Image:       image,
		Environment: strings.TrimSpace(os.Getenv("APP_ENV")),
		Replicas:    1,
		Port:        8080,
		Config:      map[string]string{},
	}
	if cfg.AppName == "" {
		cfg.AppName = "infrax-app"
	}
	if cfg.Image == "" {
		cfg.Image = "nginx:latest"
	}
	if cfg.Environment == "" {
		cfg.Environment = "dev"
	}
	if p, err := strconv.Atoi(strings.TrimSpace(os.Getenv("APP_PORT"))); err == nil && p > 0 {
		cfg.Port = p
	}
	if r, err := strconv.Atoi(strings.TrimSpace(os.Getenv("APP_REPLICAS"))); err == nil && r > 0 {
		cfg.Replicas = r
	}
	cfg.Config["APP_ENV"] = cfg.Environment
	cfg.Config["APP_NAME"] = cfg.AppName
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "APP_CONFIG_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimPrefix(parts[0], "APP_CONFIG_")
			cfg.Config[key] = parts[1]
		}
	}
	return cfg
}

func renderReleaseManifest(args ...any) string {
	cfg := buildReleaseConfig("", "")
	switch len(args) {
	case 0:
		cfg = buildReleaseConfig("", "")
	case 1:
		if c, ok := args[0].(ReleaseConfig); ok {
			cfg = c
		}
	case 2:
		image, _ := args[0].(string)
		ns, _ := args[1].(string)
		cfg = buildReleaseConfig(image, ns)
	}
	if cfg.Config == nil {
		cfg.Config = map[string]string{}
	}
	cfg.Namespace = normalizeNamespace(cfg.Namespace)
	cfg.AppName = strings.TrimSpace(cfg.AppName)
	if cfg.AppName == "" {
		cfg.AppName = "infrax-app"
	}
	cfg.Environment = strings.TrimSpace(cfg.Environment)
	if cfg.Environment == "" {
		cfg.Environment = "dev"
	}
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	if cfg.Replicas <= 0 {
		cfg.Replicas = 1
	}
	cfg.Config["APP_ENV"] = cfg.Environment
	cfg.Config["APP_NAME"] = cfg.AppName

	keys := make([]string, 0, len(cfg.Config))
	for k := range cfg.Config {
		keys = append(keys, k)
	}
	dataBlock := ""
	for _, k := range keys {
		dataBlock += fmt.Sprintf("  %s: %q\n", k, cfg.Config[k])
	}

	return fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s-config
  namespace: %s
data:
%s---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    environment: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
        environment: %s
    spec:
      containers:
        - name: %s
          image: %s
          ports:
            - name: http
              containerPort: %d
          envFrom:
            - configMapRef:
                name: %s-config
---
apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    app: %s
  ports:
    - name: http
      port: 80
      targetPort: %d
`, cfg.AppName, cfg.Namespace, dataBlock, cfg.AppName, cfg.Namespace, cfg.AppName, cfg.Environment, cfg.Replicas, cfg.AppName, cfg.AppName, cfg.Environment, cfg.AppName, cfg.Image, cfg.Port, cfg.AppName, cfg.AppName, cfg.Namespace, cfg.AppName, cfg.Port)
}

func runReleaseWithRunner(runner kubectlRunner, image, namespace string) error {
	if runner == nil {
		runner = newDefaultExecutor().runner
	}
	cfg := buildReleaseConfig(image, namespace)
	releaseName := cfg.AppName
	if _, err := runner("apply", "-f", "-", "-n", cfg.Namespace); err != nil {
		_, _ = runner("rollout", "undo", "deployment/"+releaseName, "-n", cfg.Namespace)
		return fmt.Errorf("kubectl apply failed: %w", err)
	}
	if _, err := runner("rollout", "status", "deployment/"+releaseName, "-n", cfg.Namespace, "--timeout=120s"); err != nil {
		_, _ = runner("rollout", "undo", "deployment/"+releaseName, "-n", cfg.Namespace)
		return fmt.Errorf("kubectl rollout status failed: %w", err)
	}
	return nil
}

func runKubectlApply(manifest, namespace string) error {
	args := append(clusterAuthArgs(), "apply", "-f", "-", "-n", namespace)
	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = strings.NewReader(manifest)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply failed: %w: %s", err, strings.TrimSpace(out.String()))
	}
	return nil
}

func runKubectlRolloutStatus(namespace, name string, timeout time.Duration) error {
	args := append(clusterAuthArgs(), "rollout", "status", "deployment/"+name, "-n", namespace, "--timeout="+timeout.String())
	cmd := exec.Command("kubectl", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl rollout status failed: %w: %s", err, strings.TrimSpace(out.String()))
	}
	return nil
}

func rollbackRelease(namespace, name string) error {
	args := append(clusterAuthArgs(), "rollout", "undo", "deployment/"+name, "-n", namespace)
	cmd := exec.Command("kubectl", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl rollback failed: %w: %s", err, strings.TrimSpace(out.String()))
	}
	return nil
}

func executeReleaseWithKubectl(image, namespace string) error {
	cfg := buildReleaseConfig(image, namespace)
	manifest := renderReleaseManifest(cfg)
	if err := runKubectlApply(manifest, cfg.Namespace); err != nil {
		_ = rollbackRelease(cfg.Namespace, cfg.AppName)
		return err
	}
	if err := runKubectlRolloutStatus(cfg.Namespace, cfg.AppName, 120*time.Second); err != nil {
		_ = rollbackRelease(cfg.Namespace, cfg.AppName)
		return err
	}
	return nil
}

func normalizeNamespace(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return "default"
	}
	return v
}

func deployToKubernetes(image, namespace string) error {
	ns := normalizeNamespace(namespace)
	return executeReleaseWithKubectl(image, ns)
}

func waitForRollout(driver *releaseExecutor, namespace, name string, timeout time.Duration) error {
	if driver == nil {
		driver = newDefaultExecutor()
	}
	_, err := driver.runner("rollout", "status", "deployment/"+name, "-n", namespace, "--timeout="+timeout.String())
	if err != nil {
		_, _ = driver.runner("rollout", "undo", "deployment/"+name, "-n", namespace)
		return err
	}
	return nil
}
