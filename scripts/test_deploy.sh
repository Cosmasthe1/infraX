#!/usr/bin/env bash
set -euo pipefail

# Lightweight test: POST to control plane /deploy and show a reminder to check DB
: "${DATABASE_URL?Set DATABASE_URL to the running Postgres (e.g. postgres://infrax:changeme@localhost:5432/infrax?sslmode=disable)}"

echo "Starting control plane (background)..."
pushd control-plane >/dev/null
DATABASE_URL="$DATABASE_URL" go run main.go &
CP_PID=$!
popd >/dev/null

sleep 1

echo "Posting a test deployment to /deploy"
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"tenant":"team-a","image":"registry/sample-app:latest"}' \
  http://localhost:9090/deploy || true

echo
echo "Control plane pid: $CP_PID"
echo "Give it a second to persist; then verify the DB contains the record. Example psql command:" 
echo
echo "  psql '$DATABASE_URL' -c \"SELECT id,tenant,image,created_at FROM deployments ORDER BY id DESC LIMIT 1;\""
echo
echo "When finished, kill the control plane process: kill $CP_PID"
