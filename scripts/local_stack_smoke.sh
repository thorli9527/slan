#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
COMPOSE="docker compose -f $ROOT_DIR/docker-compose.local.yml"

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

STAMP=$(date +%s)
EMAIL="${SMOKE_EMAIL:-e2e-user-$STAMP@local.slan}"
PASSWORD="${SMOKE_PASSWORD:-e2e-user-password-2026}"
MACHINE_ID="${SMOKE_MACHINE_ID:-machine-e2e-$STAMP}"
NODE_ID="${SMOKE_NODE_ID:-node-e2e-$STAMP}"

REGISTER=$(request POST /auth/register "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
TOKEN=$(json_get accessToken "$REGISTER")
USER_ID=$(json_get userId "$REGISTER")

DEVICE=$(request POST /devices/register "{\"name\":\"MacBook E2E\",\"platform\":\"macos\",\"machineId\":\"$MACHINE_ID\",\"publicKey\":\"pub-device-e2e\"}" "$TOKEN")
DEVICE_ID=$(json_get deviceId "$DEVICE")

NETWORK=$(request POST /networks '{"name":"Local E2E Net","description":"small-scale rollout check","cidr":"100.96.0.0/24"}' "$TOKEN")
NETWORK_ID=$(json_get networkId "$NETWORK")

NODE=$(request POST /nodes/register "{\"deviceId\":\"$DEVICE_ID\",\"nodeId\":\"$NODE_ID\",\"nodePublicKey\":\"node-pub-e2e\",\"capabilities\":[\"desktop\"]}" "$TOKEN")

JOIN=$(request POST "/networks/$NETWORK_ID/join" "{\"deviceId\":\"$DEVICE_ID\"}" "$TOKEN")

CONTROL=$(request POST /control/sessions "{\"nodeId\":\"$NODE_ID\",\"networkId\":\"$NETWORK_ID\"}" "$TOKEN")
BOOTSTRAP=$(request POST /bootstrap "{\"nodeId\":\"$NODE_ID\",\"networkId\":\"$NETWORK_ID\"}" "$TOKEN")
TICKET=$(request POST /relay/tickets "{\"networkId\":\"$NETWORK_ID\",\"srcNodeId\":\"$NODE_ID\",\"dstNodeId\":\"$NODE_ID\",\"reason\":\"e2e-smoke\"}" "$TOKEN")

printf 'REGISTER=%s\n' "$REGISTER"
printf 'USER_ID=%s\n' "$USER_ID"
printf 'DEVICE=%s\n' "$DEVICE"
printf 'NETWORK=%s\n' "$NETWORK"
printf 'NODE=%s\n' "$NODE"
printf 'JOIN=%s\n' "$JOIN"
printf 'CONTROL=%s\n' "$CONTROL"
printf 'BOOTSTRAP=%s\n' "$BOOTSTRAP"
printf 'TICKET=%s\n' "$TICKET"
