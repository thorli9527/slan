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
RUN_REMOTE_SMOKE="${RUN_REMOTE_SMOKE:-1}"
REMOTE_SMOKE_SEED_WIRE_NODES="${REMOTE_SMOKE_SEED_WIRE_NODES:-0}"
RUN_REMOTE_PUNCH_SMOKE="${RUN_REMOTE_PUNCH_SMOKE:-1}"
RUN_REMOTE_UI_OPS_SMOKE="${RUN_REMOTE_UI_OPS_SMOKE:-0}"
RUN_REMOTE_APP_DNS_ACL_SMOKE="${RUN_REMOTE_APP_DNS_ACL_SMOKE:-0}"
RUN_POST_PUBLISH_CLIENT_VALIDATION="${RUN_POST_PUBLISH_CLIENT_VALIDATION:-0}"
RUN_LOCAL_PRECHECKS="${RUN_LOCAL_PRECHECKS:-1}"
TOKEN_SCHEMA_MODE="${TOKEN_SCHEMA_MODE:-compatible}"
TOKEN_SCHEMA_RESET="${TOKEN_SCHEMA_RESET:-0}"

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
  RUN_REMOTE_SMOKE=1
  REMOTE_SMOKE_SEED_WIRE_NODES=0
  RUN_REMOTE_PUNCH_SMOKE=1
  RUN_REMOTE_UI_OPS_SMOKE=0
  RUN_REMOTE_APP_DNS_ACL_SMOKE=0
  RUN_POST_PUBLISH_CLIENT_VALIDATION=0
  RUN_LOCAL_PRECHECKS=1
  TOKEN_SCHEMA_MODE=compatible|strict
  TOKEN_SCHEMA_RESET=0|1
  SSHPASS='password'   # optional, only used if sshpass is installed

Smoke presets:
  Default publish validation:
    RUN_REMOTE_SMOKE=1
    RUN_REMOTE_PUNCH_SMOKE=1

  App client DNS/ACL/message validation:
    RUN_REMOTE_APP_DNS_ACL_SMOKE=1

  Heavy UI/OPS end-to-end validation:
    RUN_REMOTE_UI_OPS_SMOKE=1

  Client follow-up validation from this Mac / VM:
    RUN_POST_PUBLISH_CLIENT_VALIDATION=1
EOF
  exit 2
fi

case "$TOKEN_SCHEMA_MODE" in
  compatible|strict)
    ;;
  *)
    echo "invalid TOKEN_SCHEMA_MODE: $TOKEN_SCHEMA_MODE (expected compatible|strict)" >&2
    exit 2
    ;;
esac

if [ "$TOKEN_SCHEMA_MODE" = "strict" ] && [ "$TOKEN_SCHEMA_RESET" != "1" ]; then
  cat >&2 <<'EOF'
Refusing strict token-schema deploy without explicit reset confirmation.

Set:
  TOKEN_SCHEMA_MODE=strict
  TOKEN_SCHEMA_RESET=1

Strict mode means token-related database/API compatibility is not preserved.
Use it only when the target database has been cleared or rebuilt for the new schema.
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

env_value() {
  local key="$1"
  awk -F= -v key="$key" '$1 == key { print substr($0, index($0, "=") + 1) }' "$ENV_SOURCE" | tail -n1
}

if [ ! -f "$ROOT_DIR/$COMPOSE_FILE" ]; then
  echo "compose file not found: $ROOT_DIR/$COMPOSE_FILE" >&2
  exit 1
fi

if [ ! -f "$ENV_SOURCE" ]; then
  echo "env file not found: $ENV_SOURCE" >&2
  exit 1
fi

if [ "$RUN_LOCAL_PRECHECKS" = "1" ]; then
  echo "==> Local prechecks: service-biz tests"
  (cd "$ROOT_DIR/server/service-biz" && go test ./...)

  echo "==> Local prechecks: web-ui build"
  (cd "$ROOT_DIR/server/web-ui" && npm run build)

  echo "==> Local prechecks: client-core-service compile check"
  (cd "$ROOT_DIR/client_v2/rust" && cargo test -p client-core-service --no-run)
fi

echo "==> Checking remote docker on ${SSH_TARGET}"
remote_ssh "docker --version >/dev/null && docker compose version >/dev/null"

if [ "$TOKEN_SCHEMA_MODE" = "strict" ]; then
  echo "==> Strict token schema mode enabled; token compatibility is intentionally disabled"
fi

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

set_env_value() {
  local file="$1"
  local key="$2"
  local value="$3"
  awk -v key="$key" -v value="$value" '
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
  ' "$file" > "$file.next"
  mv "$file.next" "$file"
}

env_value() {
  local file="$1"
  local key="$2"
  awk -F= -v key="$key" '$1 == key { print substr($0, index($0, "=") + 1) }' "$file" | tail -n1
}

normalize_ticket_secrets() {
  local file="$1"
  local relay_secret
  local wire_secret
  local wire_ring

  relay_secret="$(env_value "$file" "SLAN_RELAY_TICKET_SECRET")"
  relay_secret="${relay_secret#"${relay_secret%%[![:space:]]*}"}"
  relay_secret="${relay_secret%"${relay_secret##*[![:space:]]}"}"
  if [ -z "$relay_secret" ]; then
    return
  fi

  wire_secret="$(env_value "$file" "SLAN_WIRE_TICKET_SECRET")"
  wire_secret="${wire_secret#"${wire_secret%%[![:space:]]*}"}"
  wire_secret="${wire_secret%"${wire_secret##*[![:space:]]}"}"
  if [ -z "$wire_secret" ] || [ "$wire_secret" != "$relay_secret" ]; then
    set_env_value "$file" "SLAN_WIRE_TICKET_SECRET" "$relay_secret"
  fi

  wire_ring="$(env_value "$file" "SLAN_WIRE_TICKET_SECRETS")"
  wire_ring="${wire_ring#"${wire_ring%%[![:space:]]*}"}"
  wire_ring="${wire_ring%"${wire_ring##*[![:space:]]}"}"
  case ",$wire_ring," in
    *",$relay_secret,"*)
      if [ "${wire_ring%%,*}" != "$relay_secret" ]; then
        set_env_value "$file" "SLAN_WIRE_TICKET_SECRETS" "$relay_secret,$wire_ring"
      fi
      ;;
    "")
      set_env_value "$file" "SLAN_WIRE_TICKET_SECRETS" "$relay_secret"
      ;;
    *)
      set_env_value "$file" "SLAN_WIRE_TICKET_SECRETS" "$relay_secret,$wire_ring"
      ;;
  esac
}

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

normalize_ticket_secrets "$incoming"
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

if [ "$TOKEN_SCHEMA_MODE" = "strict" ] && [ "$TOKEN_SCHEMA_RESET" = "1" ]; then
  echo "==> Strict token schema reset: clearing token/session tables on remote postgres"
  remote_bash "$REMOTE_DIR" "$ENV_FILE" "$COMPOSE_FILE" <<'EOF'
set -euo pipefail

remote_dir="$1"
env_file="$2"
compose_file="$3"

cd "$remote_dir"
docker compose --env-file "$env_file" -f "$compose_file" exec -T -u postgres postgres \
  sh -lc "psql -U postgres -d postgres -v ON_ERROR_STOP=1 <<'SQL'
DO \$\$
DECLARE
  names text[] := ARRAY[
    'gorm_user_session_records',
    'gorm_user_session_record',
    'gorm_console_login_key_records',
    'gorm_console_login_key_record',
    'gorm_device_session_records',
    'gorm_device_session_record',
    'gorm_bootstrap_key_records',
    'gorm_bootstrap_key_record',
    'gorm_operator_session_records',
    'gorm_operator_session_record'
  ];
  item text;
  existing text[] := ARRAY[]::text[];
BEGIN
  FOREACH item IN ARRAY names LOOP
    IF to_regclass(item) IS NOT NULL THEN
      existing := array_append(existing, item);
    END IF;
  END LOOP;
  IF array_length(existing, 1) IS NULL THEN
    RAISE NOTICE 'No token/session tables found to truncate';
    RETURN;
  END IF;
  EXECUTE 'TRUNCATE TABLE ' || array_to_string(existing, ', ') || ' RESTART IDENTITY';
END
\$\$;
SQL"
EOF
fi

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

if [ "$RUN_REMOTE_SMOKE" = "1" ]; then
  SLAN_BIZ_PUBLIC_PORT="$(env_value SLAN_BIZ_PUBLIC_PORT)"
  SLAN_WEB_PORT="$(env_value SLAN_WEB_PORT)"
  SLAN_BIZ_OPS_PUBLIC_PORT="$(env_value SLAN_BIZ_OPS_PUBLIC_PORT)"
  SLAN_INTERNAL_WIRE_TOKEN_VALUE="$(env_value SLAN_INTERNAL_WIRE_TOKEN)"

  APP_SMOKE_URL="${SLAN_APP_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}"
  WEB_SMOKE_URL="${SLAN_WEB_BASE_URL:-http://${REMOTE_HOST}:${SLAN_WEB_PORT:-24200}}"
  OPS_SMOKE_URL="${SLAN_OPS_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_OPS_PUBLIC_PORT:-28082}}"

  if [ -z "${SLAN_INTERNAL_WIRE_TOKEN_VALUE}" ]; then
    echo "remote smoke skipped: SLAN_INTERNAL_WIRE_TOKEN missing from ${ENV_SOURCE}" >&2
  else
    echo "==> Remote business smoke"
    SLAN_INTERNAL_WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN_VALUE}" \
    SLAN_BIZ_REMOTE_BASE_URL="${APP_SMOKE_URL}" \
    SLAN_WEB_REMOTE_BASE_URL="${WEB_SMOKE_URL}" \
    SLAN_OPS_REMOTE_BASE_URL="${OPS_SMOKE_URL}" \
    SLAN_SERVICE_BIZ_SMOKE_SEED_WIRE_NODES="${REMOTE_SMOKE_SEED_WIRE_NODES}" \
    bash "$ROOT_DIR/scripts/service_biz_remote_smoke.sh"
  fi
fi

if [ "$RUN_REMOTE_PUNCH_SMOKE" = "1" ]; then
  echo "==> Remote punch smoke"
  SLAN_BIZ_URL="${SLAN_APP_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
  bash "$ROOT_DIR/scripts/punch_biz_smoke.sh"
fi

if [ "$RUN_REMOTE_UI_OPS_SMOKE" = "1" ]; then
  echo "==> Remote UI/OPS smoke"
  SLAN_REMOTE_HOST="${REMOTE_HOST}" \
  SLAN_REMOTE_WEB_BASE="${SLAN_WEB_BASE_URL:-http://${REMOTE_HOST}:${SLAN_WEB_PORT:-24200}}" \
  SLAN_REMOTE_OPS_BASE="${SLAN_OPS_BASE_URL:-http://${REMOTE_HOST}:${SLAN_MAIN_PORT:-24201}}" \
  SLAN_REMOTE_BIZ_BASE="${SLAN_APP_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
  bash "$ROOT_DIR/scripts/remote_ui_ops_smoke.sh"
fi

if [ "$RUN_REMOTE_APP_DNS_ACL_SMOKE" = "1" ]; then
  echo "==> Remote app DNS/ACL/message smoke"
  SLAN_BIZ_URL="${SLAN_APP_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
  SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-http://${REMOTE_HOST}:${SLAN_WEB_PORT:-24200}}" \
  SLAN_EXPECT_MQTT_HOST="${SLAN_EXPECT_MQTT_HOST:-${REMOTE_HOST}}" \
  bash "$ROOT_DIR/scripts/app_dns_acl_message_smoke.sh"
fi

if [ "$RUN_POST_PUBLISH_CLIENT_VALIDATION" = "1" ]; then
  echo "==> Post-publish Linux/iOS client validation"
  SLAN_BIZ_URL="${SLAN_APP_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
  SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-http://${REMOTE_HOST}:${SLAN_WEB_PORT:-24200}}" \
  SLAN_OPS_BASE_URL="${SLAN_OPS_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_OPS_PUBLIC_PORT:-28082}}" \
  SLAN_EXPECT_MQTT_HOST="${SLAN_EXPECT_MQTT_HOST:-${REMOTE_HOST}}" \
  bash "$ROOT_DIR/scripts/post_publish_client_validation.sh"
fi

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
