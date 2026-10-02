package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// RenderDeploymentManifest produces a GitOps-ready Kubernetes release manifest
// for a target container image. It includes environment-specific namespace,
// app metadata, and config values so the generated release is suitable for
// multi-environment deployment flow.
func RenderDeploymentManifest(image, namespace string) string {
	appName := strings.TrimSpace(os.Getenv("APP_NAME"))
	if appName == "" {
		appName = "infrax-app"
	}
	if namespace == "" {
		namespace = strings.TrimSpace(os.Getenv("KUBERNETES_NAMESPACE"))
	}
	if namespace == "" {
		namespace = "default"
	}
	if image == "" {
		image = "nginx:latest"
	}
	appEnv := strings.TrimSpace(os.Getenv("APP_ENV"))
	if appEnv == "" {
		appEnv = "dev"
	}
	port := 8080
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("APP_PORT"))); err == nil && v > 0 {
		port = v
	}
	replicas := 1
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("APP_REPLICAS"))); err == nil && v > 0 {
		replicas = v
	}
	config := map[string]string{
		"APP_ENV":  appEnv,
		"APP_NAME": appName,
	}
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "APP_CONFIG_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}
			config[strings.TrimPrefix(parts[0], "APP_CONFIG_")] = parts[1]
		}
	}
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	dataBlock := ""
	for _, k := range keys {
		dataBlock += fmt.Sprintf("  %s: %q\n", k, config[k])
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
`, appName, namespace, dataBlock, appName, namespace, appName, appEnv, replicas, appName, appName, appEnv, appName, image, port, appName, appName, namespace, appName, port)
}
