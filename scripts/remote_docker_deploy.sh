#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)

REMOTE_HOST="${1:-${REMOTE_HOST:-}}"
REMOTE_DIR="${2:-${REMOTE_DIR:-/opt/slan}}"
ENV_FILE="${3:-${ENV_FILE:-.env.local}}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.local.yml}"
REMOTE_USER="${REMOTE_USER:-root}"

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
  COMPOSE_FILE=docker-compose.local.yml
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

if [ ! -f "$ROOT_DIR/$ENV_FILE" ]; then
  echo "env file not found: $ROOT_DIR/$ENV_FILE" >&2
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
  --exclude 'node_modules/' \
  --exclude 'dist/' \
  --exclude 'build/' \
  --exclude 'target/' \
  --exclude '.dart_tool/' \
  --exclude '.gradle/' \
  --exclude '.tmp/' \
  --exclude 'client_v2/app_flutter/build/' \
  "$ROOT_DIR/" "$SSH_TARGET:$REMOTE_DIR/"

echo "==> Building and starting docker compose on remote"
remote_ssh "cd '$REMOTE_DIR' && docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' up -d --build --remove-orphans"

echo "==> Remote compose status"
remote_ssh "cd '$REMOTE_DIR' && docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' ps"

echo "==> Health checks"
remote_ssh "cd '$REMOTE_DIR' && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T server-biz /bin/sh -lc 'wget -qO- http://127.0.0.1:8080/healthz' && echo && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T server-ui-web /bin/sh -lc 'wget -qO- http://127.0.0.1/ | grep -q \"<app-root\"' && echo web-ui-ok && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T server-main /bin/sh -lc 'wget -qO- http://127.0.0.1/ | grep -q \"<ops-root\"' && echo ops-ui-ok"

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
