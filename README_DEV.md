# infraX development notes

- Start a local k8s cluster: `kind create cluster` or `k3d cluster create`
- Deploy Traefik or Caddy as the ingress for local testing.
- Configure GitHub Actions secrets: `REGISTRY_HOST`, `REGISTRY_USER`, `REGISTRY_TOKEN`.

Quick test flow:

1. Build the sample app image locally: `docker build -t sample-app:local ./apps/sample-app`
2. Push or load into the cluster and apply the manifest in `deploy/gitops`.
