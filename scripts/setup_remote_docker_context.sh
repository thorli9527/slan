#!/usr/bin/env bash
set -euo pipefail

REMOTE_HOST="${1:-${REMOTE_HOST:-47.245.40.231}}"
REMOTE_USER="${2:-${REMOTE_USER:-root}}"
CONTEXT_NAME="${3:-${DOCKER_CONTEXT_NAME:-slan-remote}}"

SSH_TARGET="${REMOTE_USER}@${REMOTE_HOST}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker command not found on this Mac" >&2
  exit 1
fi

echo "==> Checking SSH access to ${SSH_TARGET}"
ssh -o StrictHostKeyChecking=accept-new "$SSH_TARGET" "docker --version && docker compose version"

if docker context inspect "$CONTEXT_NAME" >/dev/null 2>&1; then
  echo "==> Docker context exists: ${CONTEXT_NAME}"
else
  echo "==> Creating Docker context: ${CONTEXT_NAME}"
  docker context create "$CONTEXT_NAME" --docker "host=ssh://${SSH_TARGET}"
fi

echo "==> Switching Docker to remote context: ${CONTEXT_NAME}"
docker context use "$CONTEXT_NAME"

echo "==> Current Docker context"
docker context show

echo "==> Remote Docker containers"
docker ps

cat <<EOF

Remote Docker context is active.

All docker commands on this Mac now target the remote host:
  ${SSH_TARGET}

Project policy:
  Use remote Docker for development and deployment.
  Do not start the local Docker Compose stack unless explicitly debugging.

Check current context:
  docker context show

Deploy:
  .tmp/remote-deploy/deploy_to_47.245.40.231.sh

EOF
