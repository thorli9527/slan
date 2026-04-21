#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
ENV_FILE="$ROOT_DIR/.env.local"
COMPOSE_FILE="$ROOT_DIR/docker-compose.local.yml"

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" down --remove-orphans
