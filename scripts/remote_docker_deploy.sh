#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

REMOTE_HOST="${1:-${REMOTE_HOST:-${SLAN_REMOTE_HOST:-$SLAN_DEFAULT_MQTT_HOST}}}"
REMOTE_DIR="${2:-${REMOTE_DIR:-/opt/slan}}"
ENV_FILE="${3:-${ENV_FILE:-.env.local}}"
ENV_SOURCE="${ENV_SOURCE:-$ROOT_DIR/$ENV_FILE}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.local.yml}"
REMOTE_USER="${REMOTE_USER:-root}"
APP_SERVICES="${APP_SERVICES:-server-biz server-biz-ops server-wire server-wire-b server-wire-relay server-wire-relay-b server-wire-punch server-wire-punch-b server-wire-punch-c server-wire-derp server-wire-derp-b opt-ui caddy}"
INFRA_SERVICES="${INFRA_SERVICES:-postgres redis}"
BROKER_SERVICES="${BROKER_SERVICES:-bifromq}"
POST_BROKER_STABILIZATION_SECONDS="${POST_BROKER_STABILIZATION_SECONDS:-15}"
PRESERVE_ENV_KEYS="${PRESERVE_ENV_KEYS:-POSTGRES_PASSWORD SLAN_RELAY_TICKET_SECRET SLAN_INTERNAL_WIRE_TOKEN SLAN_WIRE_TICKET_SECRET SLAN_WIRE_TICKET_SECRETS SLAN_MQTT_PASSWORD_SECRET}"
RUN_REMOTE_SMOKE="${RUN_REMOTE_SMOKE:-1}"
REMOTE_SMOKE_EXECUTION="${REMOTE_SMOKE_EXECUTION:-server}"
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
  APP_SERVICES="server-biz server-biz-ops ..."
  INFRA_SERVICES="postgres redis"
  BROKER_SERVICES="bifromq"
  PRESERVE_ENV_KEYS="POSTGRES_PASSWORD ..."
  RUN_REMOTE_SMOKE=1
  REMOTE_SMOKE_EXECUTION=server|local
  REMOTE_SMOKE_SEED_WIRE_NODES=0
  RUN_REMOTE_PUNCH_SMOKE=1
  RUN_REMOTE_UI_OPS_SMOKE=0
  RUN_REMOTE_APP_DNS_ACL_SMOKE=0
  POST_BROKER_STABILIZATION_SECONDS=15
  RUN_POST_PUBLISH_CLIENT_VALIDATION=0
  RUN_LOCAL_PRECHECKS=1
  TOKEN_SCHEMA_MODE=compatible|strict
  TOKEN_SCHEMA_RESET=0|1
  SLAN_RUN_POST_PUBLISH_ANDROID_DUAL_QUICK=0
  SLAN_RUN_POST_PUBLISH_IOS_DUAL_QUICK=0
  SLAN_RUN_POST_PUBLISH_MAC_ANDROID_QUICK=0
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

  Client quick follow-up validation from this Mac:
    RUN_POST_PUBLISH_CLIENT_VALIDATION=1
    SLAN_RUN_POST_PUBLISH_ANDROID_DUAL_QUICK=1
    SLAN_RUN_POST_PUBLISH_IOS_DUAL_QUICK=1
    SLAN_RUN_POST_PUBLISH_MAC_ANDROID_QUICK=1
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

  echo "==> Local prechecks: client-core-service compile check"
  (cd "$ROOT_DIR/client/rust" && cargo test -p client-core-service --no-run)
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
  --exclude 'client/app_flutter/build/' \
  --exclude 'client/app_flutter/build.rootcache*/' \
  --exclude 'client/app_flutter/android/.kotlin/' \
  --exclude 'client/app_flutter/android/.gradle/' \
  "$ROOT_DIR/" "$SSH_TARGET:$REMOTE_DIR/"

REMOTE_ENV_TMP="$REMOTE_DIR/.env.deploy.incoming"

echo "==> Uploading env file ${ENV_SOURCE}"
rsync -az \
  -e "$(rsync_ssh_command)" \
  "$ENV_SOURCE" "$SSH_TARGET:$REMOTE_ENV_TMP"

preserve_env_keys_csv="${PRESERVE_ENV_KEYS// /,}"

echo "==> Installing remote env ${ENV_FILE}"
remote_bash "$REMOTE_DIR" "$ENV_FILE" "$REMOTE_ENV_TMP" "$preserve_env_keys_csv" "$REMOTE_HOST" <<'EOF'
set -euo pipefail

remote_dir="$1"
env_file="$2"
incoming="$3"
preserve_keys_csv="$4"
remote_host="$5"
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

normalize_mqtt_public_broker_url() {
  local file="$1"
  local host="$2"

  host="${host#"${host%%[![:space:]]*}"}"
  host="${host%"${host##*[![:space:]]}"}"
  case "$host" in
    ""|localhost|127.0.0.1|0.0.0.0|::1)
      return
      ;;
  esac
  set_env_value "$file" "SLAN_MQTT_PUBLIC_BROKER_URL" "mqtt://${host}:1883"
}

env_value_trimmed() {
  local file="$1"
  local key="$2"
  local value

  value="$(env_value "$file" "$key")"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

env_value_or_default() {
  local file="$1"
  local key="$2"
  local fallback="$3"
  local value

  value="$(env_value_trimmed "$file" "$key")"
  if [ -n "$value" ]; then
    printf '%s' "$value"
    return
  fi
  printf '%s' "$fallback"
}

normalize_wire_public_endpoints() {
  local file="$1"
  local host="$2"
  local relay_port
  local relay_b_port
  local derp_port
  local derp_b_port

  host="${host#"${host%%[![:space:]]*}"}"
  host="${host%"${host##*[![:space:]]}"}"
  case "$host" in
    ""|localhost|127.0.0.1|0.0.0.0|::1)
      return
      ;;
  esac

  relay_port="$(env_value_or_default "$file" "SLAN_WIRE_RELAY_PORT" "29110")"
  relay_b_port="$(env_value_or_default "$file" "SLAN_WIRE_RELAY_B_PORT" "29112")"
  derp_port="$(env_value_or_default "$file" "SLAN_WIRE_DERP_PORT" "29120")"
  derp_b_port="$(env_value_or_default "$file" "SLAN_WIRE_DERP_B_PORT" "29122")"

  set_env_value "$file" "SLAN_WIRE_RELAY_PUBLIC_HOST" "$host"
  set_env_value "$file" "SLAN_WIRE_RELAY_PUBLIC_UDP_PORT" "$relay_port"
  set_env_value "$file" "SLAN_WIRE_RELAY_B_PUBLIC_HOST" "$host"
  set_env_value "$file" "SLAN_WIRE_RELAY_B_PUBLIC_UDP_PORT" "$relay_b_port"
  set_env_value "$file" "SLAN_WIRE_DERP_PUBLIC_HOST" "$host"
  set_env_value "$file" "SLAN_WIRE_DERP_PUBLIC_PORT" "$derp_port"
  set_env_value "$file" "SLAN_WIRE_DERP_B_PUBLIC_HOST" "$host"
  set_env_value "$file" "SLAN_WIRE_DERP_B_PUBLIC_PORT" "$derp_b_port"
  set_env_value "$file" "SLAN_RELAY_ENDPOINTS" "${host}:${relay_port}"
}

mkdir -p "$(dirname "$target")"

if [ -f "$target" ]; then
  cp "$target" "$target.bak"
  old_ifs="$IFS"
  IFS=','
  for key in $preserve_keys_csv; do
    IFS="$old_ifs"
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
    IFS=','
  done
  IFS="$old_ifs"
fi

normalize_ticket_secrets "$incoming"
normalize_mqtt_public_broker_url "$incoming" "$remote_host"
normalize_wire_public_endpoints "$incoming" "$remote_host"
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

for service in $infra_services; do
  container_id="$(docker compose --env-file "$env_file" -f "$compose_file" ps -q "$service")"
  for _ in $(seq 1 60); do
    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id")"
    case "$status" in
      healthy|running)
        break
        ;;
      unhealthy|exited|dead)
        echo "infrastructure service $service failed with status $status" >&2
        exit 1
        ;;
    esac
    sleep 2
  done
  status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id")"
  if [ "$status" != "healthy" ] && [ "$status" != "running" ]; then
    echo "infrastructure service $service did not become ready: $status" >&2
    exit 1
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
	    'gorm_device_session_records',
	    'gorm_device_session_record',
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

if [ -n "$BROKER_SERVICES" ]; then
  echo "==> Ensuring broker services are running after app webhook endpoints are ready"
  remote_bash "$REMOTE_DIR" "$ENV_FILE" "$COMPOSE_FILE" "$BROKER_SERVICES" <<'EOF'
set -euo pipefail

remote_dir="$1"
env_file="$2"
compose_file="$3"
broker_services="$4"

cd "$remote_dir"
for service in $broker_services; do
  docker compose --env-file "$env_file" -f "$compose_file" up -d "$service"
done
EOF

  echo "==> Waiting for broker services to become healthy"
  remote_bash "$REMOTE_DIR" "$ENV_FILE" "$COMPOSE_FILE" "$BROKER_SERVICES" <<'EOF'
set -euo pipefail

remote_dir="$1"
env_file="$2"
compose_file="$3"
broker_services="$4"

cd "$remote_dir"
for service in $broker_services; do
  for _ in $(seq 1 30); do
    status="$(docker compose --env-file "$env_file" -f "$compose_file" ps "$service" --format json 2>/dev/null | sed -n 's/.*"Health":"\([^"]*\)".*/\1/p' | head -n1)"
    if [ "$status" = "healthy" ] || [ -z "$status" ]; then
      break
    fi
    sleep 2
  done
done
EOF
fi

if [ -n "$BROKER_SERVICES" ] && [ "$POST_BROKER_STABILIZATION_SECONDS" -gt 0 ]; then
  echo "==> Waiting ${POST_BROKER_STABILIZATION_SECONDS}s for app MQTT clients to stabilize"
  sleep "$POST_BROKER_STABILIZATION_SECONDS"
fi

echo "==> Remote compose status"
remote_ssh "cd '$REMOTE_DIR' && docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' ps"

echo "==> Health checks"
remote_ssh "cd '$REMOTE_DIR' && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T server-biz /bin/sh -lc 'wget -qO- http://127.0.0.1:8080/healthz' && echo && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T server-biz-ops /bin/sh -lc 'wget -qO- http://127.0.0.1:8080/healthz' && echo && \
   docker compose --env-file '$ENV_FILE' -f '$COMPOSE_FILE' exec -T opt-ui /bin/sh -lc 'wget -qO- http://127.0.0.1/ | grep -q \"<ops-root\"' && echo ops-ui-ok"

if [ "$RUN_REMOTE_SMOKE" = "1" ]; then
  SLAN_BIZ_PUBLIC_PORT="$(env_value SLAN_BIZ_PUBLIC_PORT)"
  SLAN_BIZ_OPS_PUBLIC_PORT="$(env_value SLAN_BIZ_OPS_PUBLIC_PORT)"
  SLAN_INTERNAL_WIRE_TOKEN_VALUE="$(env_value SLAN_INTERNAL_WIRE_TOKEN)"
  SLAN_OPS_DEFAULT_ADMIN_EMAIL_VALUE="$(env_value SLAN_OPS_DEFAULT_ADMIN_EMAIL)"
  SLAN_OPS_DEFAULT_ADMIN_PASSWORD_VALUE="$(env_value SLAN_OPS_DEFAULT_ADMIN_PASSWORD)"

  APP_SMOKE_URL="${SLAN_APP_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}"
  WEB_SMOKE_URL="${SLAN_WEB_BASE_URL:-$APP_SMOKE_URL}"
  OPS_SMOKE_URL="${SLAN_OPS_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_OPS_PUBLIC_PORT:-28082}}"

  if [ -z "${SLAN_INTERNAL_WIRE_TOKEN_VALUE}" ]; then
    echo "remote smoke skipped: SLAN_INTERNAL_WIRE_TOKEN missing from ${ENV_SOURCE}" >&2
  else
    echo "==> Remote business smoke"
    if [ "${REMOTE_SMOKE_EXECUTION}" = "server" ]; then
      remote_ssh "cd '$REMOTE_DIR' && \
        SLAN_INTERNAL_WIRE_TOKEN='${SLAN_INTERNAL_WIRE_TOKEN_VALUE}' \
        SLAN_OPS_EMAIL='${SLAN_OPS_DEFAULT_ADMIN_EMAIL_VALUE:-admin1}' \
        SLAN_OPS_PASSWORD='${SLAN_OPS_DEFAULT_ADMIN_PASSWORD_VALUE}' \
        SLAN_BIZ_SMOKE_START=0 \
        SLAN_APP_BASE_URL='http://127.0.0.1:${SLAN_BIZ_PUBLIC_PORT:-28080}' \
        SLAN_WEB_BASE_URL='http://127.0.0.1:${SLAN_BIZ_PUBLIC_PORT:-28080}' \
        SLAN_OPS_BASE_URL='http://127.0.0.1:${SLAN_BIZ_OPS_PUBLIC_PORT:-28082}' \
        SLAN_SERVICE_BIZ_SMOKE_SEED_WIRE_NODES='${REMOTE_SMOKE_SEED_WIRE_NODES}' \
        bash ./scripts/service_biz_smoke.sh"
    else
      SLAN_INTERNAL_WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN_VALUE}" \
      SLAN_OPS_EMAIL="${SLAN_OPS_DEFAULT_ADMIN_EMAIL_VALUE:-admin1}" \
      SLAN_OPS_PASSWORD="${SLAN_OPS_DEFAULT_ADMIN_PASSWORD_VALUE}" \
      SLAN_BIZ_REMOTE_BASE_URL="${APP_SMOKE_URL}" \
      SLAN_WEB_REMOTE_BASE_URL="${WEB_SMOKE_URL}" \
      SLAN_OPS_REMOTE_BASE_URL="${OPS_SMOKE_URL}" \
      SLAN_SERVICE_BIZ_SMOKE_SEED_WIRE_NODES="${REMOTE_SMOKE_SEED_WIRE_NODES}" \
      bash "$ROOT_DIR/scripts/service_biz_remote_smoke.sh"
    fi
  fi
fi

if [ "$RUN_REMOTE_PUNCH_SMOKE" = "1" ]; then
  echo "==> Remote punch smoke"
  SLAN_BIZ_OPS_PUBLIC_PORT="$(env_value SLAN_BIZ_OPS_PUBLIC_PORT)"
  SLAN_OPS_DEFAULT_ADMIN_EMAIL_VALUE="$(env_value SLAN_OPS_DEFAULT_ADMIN_EMAIL)"
  SLAN_OPS_DEFAULT_ADMIN_PASSWORD_VALUE="$(env_value SLAN_OPS_DEFAULT_ADMIN_PASSWORD)"
  SLAN_BIZ_URL="${SLAN_APP_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
  SLAN_OPS_BASE_URL="${SLAN_OPS_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_OPS_PUBLIC_PORT:-28082}}" \
  SLAN_OPS_EMAIL="${SLAN_OPS_DEFAULT_ADMIN_EMAIL_VALUE:-admin1}" \
  SLAN_OPS_PASSWORD="${SLAN_OPS_DEFAULT_ADMIN_PASSWORD_VALUE}" \
  bash "$ROOT_DIR/scripts/punch_biz_smoke.sh"
fi

if [ "$RUN_REMOTE_UI_OPS_SMOKE" = "1" ]; then
  echo "==> Remote UI/OPS smoke"
  SLAN_REMOTE_HOST="${REMOTE_HOST}" \
  SLAN_REMOTE_WEB_BASE="${SLAN_WEB_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
  SLAN_REMOTE_OPS_BASE="${SLAN_OPS_BASE_URL:-http://${REMOTE_HOST}:${SLAN_MAIN_PORT:-24201}}" \
  SLAN_REMOTE_BIZ_BASE="${SLAN_APP_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
  bash "$ROOT_DIR/scripts/remote_ui_ops_smoke.sh"
fi

if [ "$RUN_REMOTE_APP_DNS_ACL_SMOKE" = "1" ]; then
  echo "==> Remote app DNS/ACL/message smoke"
  SLAN_BIZ_URL="${SLAN_APP_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
  SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
  SLAN_EXPECT_MQTT_HOST="${SLAN_EXPECT_MQTT_HOST:-${REMOTE_HOST}}" \
  bash "$ROOT_DIR/scripts/app_dns_acl_message_smoke.sh"
fi

if [ "$RUN_POST_PUBLISH_CLIENT_VALIDATION" = "1" ]; then
  echo "==> Post-publish Linux/iOS client validation"
  SLAN_BIZ_URL="${SLAN_APP_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
  SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-http://${REMOTE_HOST}:${SLAN_BIZ_PUBLIC_PORT:-28080}}" \
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
