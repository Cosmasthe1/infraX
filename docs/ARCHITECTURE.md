# infraX Architecture (overview)

High level components:

- Control plane: API that records deployments and coordinates GitOps/CI actions.
- Build pipeline: BuildKit-based builders produce container images and push to a registry.
- GitOps: manifests reconciled by ArgoCD/Flux to deploy apps to Kubernetes.
- Ingress: Traefik or Caddy handles routing and TLS.
- Postgres: stores tenant metadata, deployment history, and audit logs.
- Observability: OpenTelemetry Collector → Prometheus/Loki/Grafana (traces, metrics, logs).
- Edge workers: Go/Rust binaries connecting to NATS JetStream or Redis Streams and using local SQLite.
- Policy scanning: OPA/Rego + tfsec/terrascan for CI and Gatekeeper for admission control.

See the `deploy/`, `ci/`, `control-plane/`, `workers/`, and `observability/` folders for examples.
