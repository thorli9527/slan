#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
ENV_FILE="$ROOT_DIR/.env.local"
COMPOSE_FILE="$ROOT_DIR/docker-compose.local.yml"
COMPOSE="docker compose --env-file $ENV_FILE -f $COMPOSE_FILE"

if [ ! -f "$ENV_FILE" ]; then
  echo ".env.local not found: $ENV_FILE" >&2
  exit 1
fi

if [ "${SKIP_PROTOCOL_CONTRACT_CHECK:-0}" != "1" ]; then
  "$ROOT_DIR/scripts/check_protocol_contracts.sh"
fi

request() {
  method="$1"
  path="$2"
  body="${3-}"
  auth="${4-}"

  if [ "$method" = "GET" ]; then
    if [ -n "$auth" ]; then
      $COMPOSE exec -T server-biz /bin/sh -lc \
        "wget -qO- --header='Authorization: Bearer $auth' http://127.0.0.1:8080$path"
    else
      $COMPOSE exec -T server-biz /bin/sh -lc \
        "wget -qO- http://127.0.0.1:8080$path"
    fi
    return
  fi

  if [ -n "$auth" ]; then
    $COMPOSE exec -T server-biz /bin/sh -lc \
      "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $auth' --post-data='$body' http://127.0.0.1:8080$path"
  else
    $COMPOSE exec -T server-biz /bin/sh -lc \
      "wget -qO- --header='Content-Type: application/json' --post-data='$body' http://127.0.0.1:8080$path"
  fi
}

json_get() {
  key="$1"
  printf '%s' "$2" | sed -n "s/.*\"$key\":\"\\([^\"]*\\)\".*/\\1/p"
}

json_get_bool() {
  key="$1"
  printf '%s' "$2" | sed -n "s/.*\"$key\":\\(true\\|false\\).*/\\1/p"
}

json_get_nested() {
  parent="$1"
  key="$2"
  printf '%s' "$3" | sed -n "s/.*\"$parent\":{[^}]*\"$key\":\"\\([^\"]*\\)\".*/\\1/p"
}

require_nonempty() {
  name="$1"
  value="$2"
  if [ -z "$value" ]; then
    echo "missing required field: $name" >&2
    exit 1
  fi
}

STAMP=$(date +%s)
EMAIL="${SMOKE_EMAIL:-e2e-user-$STAMP@local.slan}"
PASSWORD="${SMOKE_PASSWORD:-e2e-user-password-2026}"
MACHINE_ID="${SMOKE_MACHINE_ID:-machine-e2e-$STAMP}"
NODE_ID="${SMOKE_NODE_ID:-node-e2e-$STAMP}"

REGISTER=$(request POST /auth/register "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
TOKEN=$(json_get accessToken "$REGISTER")
USER_ID=$(json_get userId "$REGISTER")
require_nonempty accessToken "$TOKEN"
require_nonempty userId "$USER_ID"

DEVICE=$(request POST /devices/register "{\"name\":\"MacBook E2E\",\"platform\":\"macos\",\"machineId\":\"$MACHINE_ID\",\"publicKey\":\"pub-device-e2e\"}" "$TOKEN")
DEVICE_ID=$(json_get deviceId "$DEVICE")
require_nonempty deviceId "$DEVICE_ID"

HOME_BEFORE=$(request GET /networks/home "" "$TOKEN")
NETWORK_ID=$(json_get_nested ownedNetwork networkId "$HOME_BEFORE")
NETWORK="$HOME_BEFORE"
if [ -z "$NETWORK_ID" ]; then
  NETWORK=$(request POST /networks '{"name":"Local E2E Net","description":"small-scale rollout check","cidr":"100.96.0.0/24"}' "$TOKEN")
  NETWORK_ID=$(json_get networkId "$NETWORK")
fi
require_nonempty networkId "$NETWORK_ID"

NODE=$(request POST /nodes/register "{\"deviceId\":\"$DEVICE_ID\",\"nodeId\":\"$NODE_ID\",\"nodePublicKey\":\"node-pub-e2e\",\"capabilities\":[\"desktop\"]}" "$TOKEN")
REGISTERED_NODE_ID=$(json_get nodeId "$NODE")
require_nonempty nodeId "$REGISTERED_NODE_ID"

JOIN=$(request POST "/networks/$NETWORK_ID/join" "{\"deviceId\":\"$DEVICE_ID\"}" "$TOKEN")
JOIN_MEMBER_ID=$(json_get_nested member memberId "$JOIN")
require_nonempty join.memberId "$JOIN_MEMBER_ID"

ACTIVATE=$(request POST "/networks/$NETWORK_ID/activate" "{\"deviceId\":\"$DEVICE_ID\"}" "$TOKEN")
ACTIVATE_MEMBER_ID=$(json_get_nested member memberId "$ACTIVATE")
ACTIVATE_ATTACHMENT_ID=$(json_get_nested attachment attachmentId "$ACTIVATE")
require_nonempty activate.memberId "$ACTIVATE_MEMBER_ID"
require_nonempty activate.attachmentId "$ACTIVATE_ATTACHMENT_ID"

CONTROL=$(request POST /control/sessions "{\"nodeId\":\"$NODE_ID\",\"networkId\":\"$NETWORK_ID\"}" "$TOKEN")
CONTROL_SESSION_ID=$(json_get controlSessionId "$CONTROL")
require_nonempty controlSessionId "$CONTROL_SESSION_ID"

BOOTSTRAP=$(request POST /bootstrap "{\"nodeId\":\"$NODE_ID\",\"networkId\":\"$NETWORK_ID\"}" "$TOKEN")
BOOTSTRAP_NETWORK_ID=$(json_get_nested networkMap networkId "$BOOTSTRAP")
BOOTSTRAP_WS_URL=$(json_get_nested controlPlane wsUrl "$BOOTSTRAP")
require_nonempty bootstrap.networkMap.networkId "$BOOTSTRAP_NETWORK_ID"
require_nonempty bootstrap.controlPlane.wsUrl "$BOOTSTRAP_WS_URL"

TICKET=$(request POST /relay/tickets "{\"networkId\":\"$NETWORK_ID\",\"srcNodeId\":\"$NODE_ID\",\"dstNodeId\":\"$NODE_ID\",\"reason\":\"e2e-smoke\"}" "$TOKEN")
TICKET_ID=$(json_get ticketId "$TICKET")
TICKET_RELAY_URL=$(json_get relayUrl "$TICKET")
require_nonempty ticketId "$TICKET_ID"
require_nonempty relayUrl "$TICKET_RELAY_URL"

HOME=$(request GET /networks/home "" "$TOKEN")
HAS_NETWORK=$(json_get_bool hasNetwork "$HOME")
if [ "$HAS_NETWORK" != "true" ]; then
  echo "expected hasNetwork=true after join, got: $HOME" >&2
  exit 1
fi

printf 'REGISTER=%s\n' "$REGISTER"
printf 'USER_ID=%s\n' "$USER_ID"
printf 'DEVICE=%s\n' "$DEVICE"
printf 'NETWORK=%s\n' "$NETWORK"
printf 'NODE=%s\n' "$NODE"
printf 'JOIN=%s\n' "$JOIN"
printf 'ACTIVATE=%s\n' "$ACTIVATE"
printf 'CONTROL=%s\n' "$CONTROL"
printf 'BOOTSTRAP=%s\n' "$BOOTSTRAP"
printf 'TICKET=%s\n' "$TICKET"
printf 'HOME=%s\n' "$HOME"
printf 'SMOKE_OK network=%s device=%s node=%s session=%s ticket=%s\n' \
  "$NETWORK_ID" "$DEVICE_ID" "$REGISTERED_NODE_ID" "$CONTROL_SESSION_ID" "$TICKET_ID"
