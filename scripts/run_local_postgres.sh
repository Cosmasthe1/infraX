#!/usr/bin/env bash
set -euo pipefail

# Start local Postgres for infraX (docker-compose must be installed)
DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
pushd "$DIR" >/dev/null

echo "Starting Postgres via docker-compose..."
docker-compose -f infra/postgres/docker-compose.yml up -d

echo "Waiting for Postgres to be ready..."
until docker-compose -f infra/postgres/docker-compose.yml exec -T db pg_isready -U infrax >/dev/null 2>&1; do
  sleep 1
done

export DATABASE_URL="postgres://infrax:changeme@localhost:5432/infrax?sslmode=disable"

echo "Postgres ready. Set this env to run the control plane locally:"
echo
echo "  export DATABASE_URL=$DATABASE_URL"
echo
echo "Then run the control plane:"
echo "  (cd control-plane && DATABASE_URL=\$DATABASE_URL go run main.go)"

popd >/dev/null
