package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type kubectlRunner func(args ...string) ([]byte, error)

type releaseExecutor struct {
	runner kubectlRunner
}

func newDefaultExecutor() *releaseExecutor {
	return &releaseExecutor{runner: func(args ...string) ([]byte, error) {
		cmd := exec.Command("kubectl", args...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return out.Bytes(), err
		}
		return out.Bytes(), nil
	}}
}

func renderReleaseManifest(image, namespace string) string {
	if namespace == "" {
		namespace = "default"
	}
	if image == "" {
		image = "nginx:latest"
	}

	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: infrax-app
  namespace: %s
  labels:
    app: infrax-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: infrax-app
  template:
    metadata:
      labels:
        app: infrax-app
    spec:
      containers:
        - name: infrax-app
          image: %s
          ports:
            - containerPort: 8080
`, namespace, image)
}

func runReleaseWithRunner(runner kubectlRunner, image, namespace string) error {
	if runner == nil {
		runner = newDefaultExecutor().runner
	}
	ns := normalizeNamespace(namespace)
	releaseName := "infrax-app"
	if _, err := runner("apply", "-f", "-", "-n", ns); err != nil {
		_, _ = runner("rollout", "undo", "deployment/"+releaseName, "-n", ns)
		return fmt.Errorf("kubectl apply failed: %w", err)
	}
	if _, err := runner("rollout", "status", "deployment/"+releaseName, "-n", ns, "--timeout=120s"); err != nil {
		_, _ = runner("rollout", "undo", "deployment/"+releaseName, "-n", ns)
		return fmt.Errorf("kubectl rollout status failed: %w", err)
	}
	_ = image
	return nil
}

func runKubectlApply(manifest, namespace string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", namespace)
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
	cmd := exec.Command("kubectl", "rollout", "status", "deployment/"+name, "-n", namespace, "--timeout="+timeout.String())
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl rollout status failed: %w: %s", err, strings.TrimSpace(out.String()))
	}
	return nil
}

func rollbackRelease(namespace, name string) error {
	cmd := exec.Command("kubectl", "rollout", "undo", "deployment/"+name, "-n", namespace)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl rollback failed: %w: %s", err, strings.TrimSpace(out.String()))
	}
	return nil
}

func executeReleaseWithKubectl(image, namespace string) error {
	manifest := renderReleaseManifest(image, namespace)
	name := "infrax-app"
	if err := runKubectlApply(manifest, namespace); err != nil {
		_ = rollbackRelease(namespace, name)
		return err
	}
	if err := runKubectlRolloutStatus(namespace, name, 120*time.Second); err != nil {
		_ = rollbackRelease(namespace, name)
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
	name := "infrax-app"
	ns := normalizeNamespace(namespace)
	if err := executeReleaseWithKubectl(image, ns); err != nil {
		_ = name
		return err
	}
	return nil
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
