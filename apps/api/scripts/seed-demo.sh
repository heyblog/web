#!/bin/sh
set -eu

usage() {
  printf '%s\n' 'Usage: sh apps/api/scripts/seed-demo.sh' \
    'Import fictional blogs into the local heyblog-dev-env database.' \
    'Start the development dependencies and API migrations first.' \
    'Repeated imports preserve existing demo edits; conflicts roll back the entire import.'
}

case "${1:-}" in
  --help|-h) usage; exit 0 ;;
  '') ;;
  *) usage >&2; exit 2 ;;
esac
if [ "$#" -gt 1 ]; then
  usage >&2
  exit 2
fi

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/../../.." && pwd)
if ! command -v docker >/dev/null 2>&1; then
  printf '%s\n' 'Docker with Compose is required.' >&2
  exit 1
fi

if [ -n "${DOCKER_CONTEXT:-}" ]; then
  docker_endpoint=$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')
elif [ -n "${DOCKER_HOST:-}" ]; then
  docker_endpoint=$DOCKER_HOST
else
  docker_endpoint=$(docker context inspect --format '{{.Endpoints.docker.Host}}')
fi
case "$docker_endpoint" in
  unix://*) ;;
  *) printf '%s\n' 'Demo import requires a local Docker Unix socket; remote endpoints are refused.' >&2; exit 1 ;;
esac

# Fix the project, service and database; do not load application environment files.
docker compose --env-file /dev/null --project-name heyblog-dev-env \
  -f "$repo_root/infra/docker/docker-compose.env.yaml" \
  exec -T postgres psql --username postgres --dbname heyblog \
  --no-psqlrc --set ON_ERROR_STOP=1 --single-transaction \
  < "$repo_root/apps/api/testdata/demo/seed.sql"
