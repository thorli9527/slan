#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
ENV_FILE="$ROOT_DIR/.env.local"
COMPOSE_FILE="$ROOT_DIR/docker-compose.local.yml"

if [ ! -f "$ENV_FILE" ]; then
  echo ".env.local not found: $ENV_FILE" >&2
  exit 1
fi

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --build postgres redis server-biz server-relay caddy
