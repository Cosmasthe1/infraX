package main

import "fmt"

// RenderDeploymentManifest produces a GitOps-ready Kubernetes Deployment manifest
// for a target container image.
func RenderDeploymentManifest(image, namespace string) string {
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
