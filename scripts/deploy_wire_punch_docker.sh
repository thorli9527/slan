#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

IMAGE_NAME="${SLAN_WIRE_PUNCH_IMAGE:-slan/server-wire-punch:local}"
CONTAINER_NAME="${SLAN_WIRE_PUNCH_CONTAINER:-server-wire-punch}"
UDP_PORT="${SLAN_WIRE_PUNCH_PORT:-29130}"
HTTP_PORT="${SLAN_WIRE_PUNCH_HTTP_PORT:-29131}"
PUBLIC_HOST="${SLAN_WIRE_PUNCH_PUBLIC_HOST:-}"
PUBLIC_UDP_PORT="${SLAN_WIRE_PUNCH_PUBLIC_UDP_PORT:-$UDP_PORT}"
LISTEN_ADDR="${SLAN_WIRE_PUNCH_LISTEN_ADDR:-:29130}"
HTTP_LISTEN_ADDR="${SLAN_WIRE_PUNCH_HTTP_LISTEN_ADDR:-:29131}"
INTERNAL_WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN:-change-me-wire-internal-token}"
ENDPOINT_TTL_SECONDS="${SLAN_WIRE_PUNCH_ENDPOINT_TTL_SECONDS:-120}"
SESSION_TTL_SECONDS="${SLAN_WIRE_PUNCH_SESSION_TTL_SECONDS:-60}"
DOCKER_NETWORK="${SLAN_WIRE_PUNCH_DOCKER_NETWORK:-bridge}"

usage() {
  cat <<'EOF'
Usage:
  scripts/deploy_wire_punch_docker.sh

Environment variables:
  SLAN_WIRE_PUNCH_IMAGE=slan/server-wire-punch:local
  SLAN_WIRE_PUNCH_CONTAINER=server-wire-punch
  SLAN_WIRE_PUNCH_PORT=29130
  SLAN_WIRE_PUNCH_HTTP_PORT=29131
  SLAN_WIRE_PUNCH_PUBLIC_HOST=203.0.113.10
  SLAN_WIRE_PUNCH_PUBLIC_UDP_PORT=29130
  SLAN_WIRE_PUNCH_LISTEN_ADDR=:29130
  SLAN_WIRE_PUNCH_HTTP_LISTEN_ADDR=:29131
  SLAN_INTERNAL_WIRE_TOKEN=change-me-wire-internal-token
  SLAN_WIRE_PUNCH_ENDPOINT_TTL_SECONDS=120
  SLAN_WIRE_PUNCH_SESSION_TTL_SECONDS=60
  SLAN_WIRE_PUNCH_DOCKER_NETWORK=bridge

Example:
  SLAN_WIRE_PUNCH_PUBLIC_HOST=203.0.113.10 \
  SLAN_INTERNAL_WIRE_TOKEN=replace-with-prod-secret \
  scripts/deploy_wire_punch_docker.sh
EOF
}

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

need docker

if [[ -z "$PUBLIC_HOST" ]]; then
  fail "SLAN_WIRE_PUNCH_PUBLIC_HOST is required"
fi

if [[ "$INTERNAL_WIRE_TOKEN" == *change-me* ]]; then
  fail "SLAN_INTERNAL_WIRE_TOKEN must be replaced with a real secret"
fi

log "build image: $IMAGE_NAME"
docker build -t "$IMAGE_NAME" "$ROOT_DIR/server/server-wire-punch"

log "remove old container if exists: $CONTAINER_NAME"
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true

log "start punch node container"
docker run -d \
  --name "$CONTAINER_NAME" \
  --restart unless-stopped \
  --network "$DOCKER_NETWORK" \
  -p "${UDP_PORT}:29130/udp" \
  -p "${HTTP_PORT}:29131" \
  -e SLAN_ENV=production \
  -e SLAN_WIRE_PUNCH_LISTEN_ADDR="$LISTEN_ADDR" \
  -e SLAN_WIRE_PUNCH_HTTP_LISTEN_ADDR="$HTTP_LISTEN_ADDR" \
  -e SLAN_WIRE_PUNCH_PUBLIC_HOST="$PUBLIC_HOST" \
  -e SLAN_WIRE_PUNCH_PUBLIC_UDP_PORT="$PUBLIC_UDP_PORT" \
  -e SLAN_INTERNAL_WIRE_TOKEN="$INTERNAL_WIRE_TOKEN" \
  -e SLAN_WIRE_PUNCH_ENDPOINT_TTL_SECONDS="$ENDPOINT_TTL_SECONDS" \
  -e SLAN_WIRE_PUNCH_SESSION_TTL_SECONDS="$SESSION_TTL_SECONDS" \
  "$IMAGE_NAME" >/dev/null

log "container status"
docker ps --filter "name=${CONTAINER_NAME}" --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'

cat <<EOF

wirePunchContainer: $CONTAINER_NAME
wirePunchImage: $IMAGE_NAME
wirePunchUdp: ${PUBLIC_HOST}:${PUBLIC_UDP_PORT}
wirePunchHttp: http://${PUBLIC_HOST}:${HTTP_PORT}

Health check:
  curl http://${PUBLIC_HOST}:${HTTP_PORT}/healthz

EOF
