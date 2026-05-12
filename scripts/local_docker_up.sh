#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
ENV_FILE="$ROOT_DIR/.env.local"
COMPOSE_FILE="$ROOT_DIR/docker-compose.local.yml"

if [ ! -f "$ENV_FILE" ]; then
  echo ".env.local not found: $ENV_FILE" >&2
  exit 1
fi

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --build --remove-orphans \
  postgres redis bifromq \
  server-biz server-wire server-wire-b server-wire-relay server-wire-relay-b server-wire-derp server-wire-derp-b \
  server-ui-web server-main caddy
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T caddy caddy reload --config /etc/caddy/Caddyfile
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T caddy caddy validate --config /etc/caddy/Caddyfile

cat <<'EOF'

Local SLAN stack is up.

Entrypoints:
  Ops Console: https://127.0.0.1:18443/
  Ops Console: http://127.0.0.1:18080/
  Ops Console: https://main.slan.localhost:18443/
  Web Console: https://web.slan.localhost:18443/
  Public API:   https://slan.localhost:18443/healthz
  Ops Console: https://ops.slan.localhost:18443/

Run checks:
  sh scripts/local_docker_check.sh
EOF
