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
Local Docker cleanup is blocked unless explicitly requested.

The project runtime now lives on the remote Docker host. To clean a previously
created local stack, run:
  SLAN_ALLOW_LOCAL_DOCKER=1 sh scripts/local_docker_down.sh

Remote deploy/cleanup remains:
  .tmp/remote-deploy/deploy_to_47.245.40.231.sh
EOF
  exit 2
fi

docker --context "$LOCAL_CONTEXT" compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" down -v --remove-orphans
