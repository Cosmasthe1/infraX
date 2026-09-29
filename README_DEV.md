# infraX development notes

- Start a local k8s cluster: `kind create cluster` or `k3d cluster create`
- Deploy Traefik or Caddy as the ingress for local testing.
- Configure GitHub Actions secrets: `REGISTRY_HOST`, `REGISTRY_USER`, `REGISTRY_TOKEN`.

Quick test flow:

1. Build the sample app image locally: `docker build -t sample-app:local ./apps/sample-app`
2. Push or load into the cluster and apply the manifest in `deploy/gitops`.

Local Postgres test (quick)

1. Start a local Postgres for the control plane:

```bash
./scripts/run_local_postgres.sh
```

2. Export the printed `DATABASE_URL` and run the lightweight test to post a deployment:

```bash
export DATABASE_URL="postgres://infrax:changeme@localhost:5432/infrax?sslmode=disable"
./scripts/test_deploy.sh
```

3. Verify the DB record (example):

```bash
psql "$DATABASE_URL" -c "SELECT id,tenant,image,created_at FROM deployments ORDER BY id DESC LIMIT 1;"
```

