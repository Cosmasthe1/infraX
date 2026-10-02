# infraX

infraX is a lightweight multi-tenant deployment platform scaffold with a Go control plane, a placeholder worker, a sample Node app, and basic GitOps/IaC conventions.

## Prerequisites

- Go 1.20+
- Docker and Docker Compose
- PostgreSQL 15 (or the bundled Compose service)
- Optional: Terraform 1.8+

## Install

```bash
git clone <repo-url>
cd infraX
cp .env.example .env
# edit .env if you want custom values
```

## Run locally

### Option 1: start the full stack with Docker Compose

```bash
docker compose up --build
```

This starts:
- PostgreSQL on localhost:5432
- the control plane on http://localhost:9090
- the sample app on http://localhost:8080
- a worker service polling for work

### Option 2: run the Go services manually

```bash
cd control-plane
export $(grep -v '^#' ../.env | xargs)
go run .
```

In another terminal:

```bash
cd workers
export $(grep -v '^#' ../.env | xargs)
go run .
```

## Test

```bash
cd control-plane
go test ./... -v
```

The default test suite is unit-test friendly and avoids requiring a live external Postgres instance. The integration test remains opt-in behind the `integration` build tag.

## Environment variables

The project reads the following variables:

- `ADMIN_API_KEY`: optional shared API key for protected endpoints
- `DATABASE_URL`: Postgres DSN for the control plane
- `PORT`: optional app port for the sample service

See `.env.example` for the default values.

## Architecture

The project is organized into:

- `control-plane`: HTTP service and DB persistence layer
- `workers`: placeholder job worker for future scheduling work
- `apps/sample-app`: sample Node service used to exercise the deploy flow
- `deploy/gitops`: Kubernetes manifests
- `infra/postgres`: local Postgres setup
- `terraform`: minimal IaC module structure for future environments

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the implementation notes and next steps.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for recent notable changes and the current Unreleased section.

## Quick verification checklist

- Start the stack with Docker Compose:

```bash
docker compose up --build
```

- Confirm the control plane is serving:

```bash
curl -sS http://localhost:9090/
# expect: infraX control plane (skeleton)
```

- Run unit tests locally:

```bash
cd control-plane && go test ./... -v
cd ../workers && go test ./... -v
```

## Bootstrap Terraform backend (optional)

If you want to create an S3 bucket and DynamoDB table to use as a remote Terraform backend, a small bootstrap module is provided at `terraform/bootstrap`.

With AWS credentials configured locally, run:

```bash
cd terraform/bootstrap
terraform init
terraform apply -var='bucket_name=infrax-terraform-state' -var='dynamodb_table=infrax-terraform-locks' -auto-approve
```

Then update `terraform/environments/dev/backend.tf` with the created bucket and table names and run `terraform init` in that directory.


