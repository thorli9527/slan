#!/bin/sh

set -eu

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
ENV_FILE="$ROOT_DIR/.env.local"
COMPOSE_FILE="$ROOT_DIR/docker-compose.local.yml"
LOCAL_CONTEXT="${SLAN_LOCAL_DOCKER_CONTEXT:-desktop-linux}"

if [ "${SLAN_ALLOW_LOCAL_DOCKER:-0}" != "1" ]; then
  cat >&2 <<'EOF'
Local Docker stack is disabled for this project.

Development and deployment now use the remote Docker host only:
  sh scripts/setup_remote_docker_context.sh
  .tmp/remote-deploy/deploy_to_47.245.40.231.sh

If you intentionally need to start a local Docker stack for one-off debugging:
  SLAN_ALLOW_LOCAL_DOCKER=1 sh scripts/local_docker_up.sh
EOF
  exit 2
fi

if [ ! -f "$ENV_FILE" ]; then
  echo ".env.local not found: $ENV_FILE" >&2
  exit 1
fi

docker --context "$LOCAL_CONTEXT" compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --build --remove-orphans \
  postgres redis emqx \
  server-biz server-biz-ops server-wire server-wire-b server-wire-relay server-wire-relay-b server-wire-punch server-wire-derp server-wire-derp-b \
  opt-ui caddy
docker --context "$LOCAL_CONTEXT" compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T caddy caddy reload --config /etc/caddy/Caddyfile
docker --context "$LOCAL_CONTEXT" compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T caddy caddy validate --config /etc/caddy/Caddyfile

cat <<'EOF'

Local SLAN stack is up for one-off debugging only.

Entrypoints:
  Ops Console: https://127.0.0.1:18443/
  Ops Console: http://127.0.0.1:18080/
  Ops Console: https://main.slan.localhost:18443/
  Public API:   https://slan.localhost:18443/healthz
  Ops Console: https://ops.slan.localhost:18443/

Run checks:
  sh scripts/local_docker_check.sh
EOF
