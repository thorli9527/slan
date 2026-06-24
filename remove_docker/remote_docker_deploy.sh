#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)

REMOTE_HOST="${1:-${REMOTE_HOST:-}}"
REMOTE_DIR="${2:-${REMOTE_DIR:-/opt/slan}}"
ENV_FILE="${3:-${ENV_FILE:-.env.local}}"
ENV_SOURCE="${ENV_SOURCE:-$ROOT_DIR/$ENV_FILE}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.local.yml}"
REMOTE_USER="${REMOTE_USER:-root}"
APP_SERVICES="${APP_SERVICES:-server-biz server-biz-web-console server-biz-ops server-wire server-wire-b server-wire-relay server-wire-relay-b server-wire-punch server-wire-derp server-wire-derp-b server-ui-web opt-ui caddy}"
INFRA_SERVICES="${INFRA_SERVICES:-postgres redis bifromq}"
PRESERVE_ENV_KEYS="${PRESERVE_ENV_KEYS:-POSTGRES_PASSWORD SLAN_RELAY_TICKET_SECRET SLAN_INTERNAL_WIRE_TOKEN SLAN_WIRE_TICKET_SECRET SLAN_WIRE_TICKET_SECRETS SLAN_MQTT_PASSWORD_SECRET}"

if [ -z "$REMOTE_HOST" ]; then
  cat >&2 <<'EOF'
Usage:
  scripts/remote_docker_deploy.sh <server-ip-or-domain> [remote-dir] [env-file]

Examples:
  scripts/remote_docker_deploy.sh 1.2.3.4
  scripts/remote_docker_deploy.sh 1.2.3.4 /opt/slan .env.prod

Optional environment variables:
  REMOTE_USER=root
  REMOTE_HOST=1.2.3.4
  REMOTE_DIR=/opt/slan
  ENV_FILE=.env.prod
  ENV_SOURCE=/abs/path/to/.env.prod
  COMPOSE_FILE=docker-compose.local.yml
  APP_SERVICES="server-biz server-biz-web-console ..."
  INFRA_SERVICES="postgres redis bifromq"
  PRESERVE_ENV_KEYS="POSTGRES_PASSWORD ..."
  SSHPASS='password'   # optional, only used if sshpass is installed
EOF
  exit 2
fi

SSH_TARGET="${REMOTE_USER}@${REMOTE_HOST}"
SSH_OPTS=(-o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30)
USE_SSHPASS=false
if [ -n "${SSHPASS:-}" ] && command -v sshpass >/dev/null 2>&1; then
  USE_SSHPASS=true
fi

remote_ssh() {
  if [ "$USE_SSHPASS" = true ]; then
    sshpass -e ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "$@"
  else
    ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "$@"
  fi
}

remote_bash() {
  if [ "$USE_SSHPASS" = true ]; then
    sshpass -e ssh "${SSH_OPTS[@]}" "$SSH_TARGET" bash -s -- "$@"
  else
    ssh "${SSH_OPTS[@]}" "$SSH_TARGET" bash -s -- "$@"
  fi
}

rsync_ssh_command() {
  if [ "$USE_SSHPASS" = true ]; then
    printf 'sshpass -e ssh'
  else
    printf 'ssh'
  fi
  for opt in "${SSH_OPTS[@]}"; do
    printf ' %q' "$opt"
  done
}

if [ ! -f "$ROOT_DIR/$COMPOSE_FILE" ]; then
  echo "compose file not found: $ROOT_DIR/$COMPOSE_FILE" >&2
  exit 1
fi

if [ ! -f "$ENV_SOURCE" ]; then
  echo "env file not found: $ENV_SOURCE" >&2
  exit 1
fi

echo "==> Checking remote docker on ${SSH_TARGET}"
remote_ssh "docker --version >/dev/null && docker compose version >/dev/null"

echo "==> Preparing remote directory ${REMOTE_DIR}"
remote_ssh "mkdir -p '$REMOTE_DIR'"

echo "==> Syncing workspace to ${SSH_TARGET}:${REMOTE_DIR}"
rsync -az --delete \
  -e "$(rsync_ssh_command)" \
  --exclude '.git/' \
  --exclude '.DS_Store' \
  --exclude '.env.local' \
  --exclude '.env.prod' \
  --exclude 'node_modules/' \
  --exclude 'dist/' \
  --exclude 'build/' \
  --exclude 'target/' \
  --exclude '.dart_tool/' \
  --exclude '.gradle/' \
  --exclude '.tmp/' \
  --exclude 'client_v2/app_flutter/build/' \
  "$ROOT_DIR/" "$SSH_TARGET:$REMOTE_DIR/"

REMOTE_ENV_TMP="$REMOTE_DIR/.env.deploy.incoming"

echo "==> Uploading env file ${ENV_SOURCE}"
rsync -az \
  -e "$(rsync_ssh_command)" \
  "$ENV_SOURCE" "$SSH_TARGET:$REMOTE_ENV_TMP"

echo "==> Installing remote env ${ENV_FILE}"
remote_bash "$REMOTE_DIR" "$ENV_FILE" "$REMOTE_ENV_TMP" "$PRESERVE_ENV_KEYS" <<'EOF'
set -euo pipefail

remote_dir="$1"
env_file="$2"
incoming="$3"
preserve_keys_raw="$4"
target="$remote_dir/$env_file"

mkdir -p "$(dirname "$target")"

if [ -f "$target" ]; then
  cp "$target" "$target.bak"
  for key in $preserve_keys_raw; do
    current="$(awk -F= -v key="$key" '$1 == key { print substr($0, index($0, "=") + 1) }' "$target" | tail -n1)"
    if [ -n "$current" ]; then
      awk -v key="$key" -v value="$current" '
        BEGIN { replaced = 0 }
        index($0, key "=") == 1 {
          if (!replaced) {
            print key "=" value
            replaced = 1
          }
          next
        }
        { print }
        END {
          if (!replaced) {
            print key "=" value
          }
        }
      ' "$incoming" > "$incoming.next"
      mv "$incoming.next" "$incoming"
    fi
  done
fi

mv "$incoming" "$target"
EOF

echo "==> Ensuring infrastructure services are running"
remote_bash "$REMOTE_DIR" "$ENV_FILE" "$COMPOSE_FILE" "$INFRA_SERVICES" <<'EOF'
set -euo pipefail

remote_dir="$1"
env_file="$2"
compose_file="$3"
infra_services="$4"

cd "$remote_dir"
for service in $infra_services; do
  if [ -z "$(docker compose --env-file "$env_file" -f "$compose_file" ps --status running -q "$service" 2>/dev/null || true)" ]; then
    docker compose --env-file "$env_file" -f "$compose_file" up -d "$service"
  fi
done
EOF

echo "==> Aligning postgres password with ${ENV_FILE}"
remote_bash "$REMOTE_DIR" "$ENV_FILE" "$COMPOSE_FILE" <<'EOF'
set -euo pipefail

remote_dir="$1"
env_file="$2"
compose_file="$3"

cd "$remote_dir"
password="$(awk -F= '$1 == "POSTGRES_PASSWORD" { print substr($0, index($0, "=") + 1) }' "$env_file" | tail -n1)"
if [ -z "$password" ]; then
  exit 0
fi

if [ -z "$(docker compose --env-file "$env_file" -f "$compose_file" ps -q postgres 2>/dev/null || true)" ]; then
  exit 0
fi

if [ -z "$(docker compose --env-file "$env_file" -f "$compose_file" ps --status running -q postgres 2>/dev/null || true)" ]; then
  docker compose --env-file "$env_file" -f "$compose_file" up -d postgres
fi

escaped_password="${password//\'/\'\'}"
docker compose --env-file "$env_file" -f "$compose_file" exec -T -u postgres postgres \
  sh -lc "psql -U postgres -d postgres -v ON_ERROR_STOP=1 -c \"ALTER USER postgres WITH PASSWORD '$escaped_password';\"" >/dev/null
EOF

echo "==> Building app services"
remote_ssh "cd '$REMOTE_DIR' && docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' build $APP_SERVICES"

echo "==> Starting app services"
remote_ssh "cd '$REMOTE_DIR' && docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' up -d --no-deps --remove-orphans $APP_SERVICES"

echo "==> Remote compose status"
remote_ssh "cd '$REMOTE_DIR' && docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' ps"

echo "==> Health checks"
remote_ssh "cd '$REMOTE_DIR' && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T server-biz /bin/sh -lc 'wget -qO- http://127.0.0.1:8080/healthz' && echo && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T server-biz-web-console /bin/sh -lc 'wget -qO- http://127.0.0.1:8080/healthz' && echo && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T server-biz-ops /bin/sh -lc 'wget -qO- http://127.0.0.1:8080/healthz' && echo && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T server-ui-web /bin/sh -lc 'wget -qO- http://127.0.0.1/ | grep -q \"<app-root\"' && echo web-ui-ok && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T opt-ui /bin/sh -lc 'wget -qO- http://127.0.0.1/ | grep -q \"<ops-root\"' && echo ops-ui-ok"

cat <<EOF

Remote deploy finished.

Server:
  ${SSH_TARGET}

Remote directory:
  ${REMOTE_DIR}

Compose:
  ${COMPOSE_FILE}

Env:
  ${ENV_FILE}

EOF
