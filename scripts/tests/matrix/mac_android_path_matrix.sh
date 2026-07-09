#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
ANDROID_BIZ_URL="${SLAN_ANDROID_BIZ_URL:-$BIZ_URL}"
SERVER_HOST="${SLAN_SERVER_HOST:-$SLAN_DEFAULT_MQTT_HOST}"
MODES="${SLAN_PATH_MATRIX_MODES:-direct,udp-relay,tcp-relay}"
MANAGE_MAC_SERVICE="${SLAN_PATH_MATRIX_MANAGE_MAC_SERVICE:-0}"
RESTORE_ACTIVE="${SLAN_PATH_MATRIX_RESTORE_ACTIVE:-1}"
SKIP_ADMIN_PATCH="${SLAN_PATH_MATRIX_SKIP_ADMIN_PATCH:-0}"
MAC_SERVICE_HOST="${SLAN_MAC_SERVICE_HOST:-127.0.0.1:46392}"
MACOS_APP_PATH="${SLAN_MACOS_APP_PATH:-client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app}"
MACOS_SERVICE_BINARY="${SLAN_MACOS_SERVICE_BINARY:-}"
TOKEN="${SLAN_INTERNAL_WIRE_TOKEN:-}"
SUDO_PASSWORD="${SLAN_SUDO_PASSWORD:-}"

cd "$ROOT_DIR"

if [[ -z "$TOKEN" && -f .env.prod ]]; then
  TOKEN="$(grep '^SLAN_INTERNAL_WIRE_TOKEN=' .env.prod | cut -d= -f2- || true)"
fi

fail() {
  echo "$*" >&2
  exit 1
}

curl_internal() {
  [[ -n "$TOKEN" ]] || fail "missing SLAN_INTERNAL_WIRE_TOKEN; set env or keep .env.prod readable"
  curl --silent --show-error --fail \
    -H "X-Slan-Internal-Token: ${TOKEN}" \
    "$@"
}

list_region_node_ids() {
  local kind="$1"
  local region="$2"
  local path
  case "$kind" in
    relay) path="relay-nodes" ;;
    derp) path="derp-nodes" ;;
    *) fail "unknown node kind: $kind" ;;
  esac
  curl_internal "${BIZ_URL}/internal/wire/admin/${path}" | \
    python3 - "$region" <<'PY'
import json
import sys

region = sys.argv[1].strip()
payload = json.load(sys.stdin)
items = payload.get("items") or []
for item in items:
    if str(item.get("regionId", "")).strip() != region:
        continue
    node_id = str(item.get("nodeId", "")).strip()
    if node_id:
        print(node_id)
PY
}

patch_node_status() {
  if [[ "$SKIP_ADMIN_PATCH" == "1" ]]; then
    echo "skip internal admin patch: kind=$1 region=$2 node=$3 enabled=$4 healthy=${5:-true}"
    return 0
  fi
  local kind="$1"
  local region="$2"
  local node="$3"
  local enabled="$4"
  local healthy="${5:-true}"
  local path
  case "$kind" in
    relay) path="relay-nodes" ;;
    derp) path="derp-nodes" ;;
    *) fail "unknown node kind: $kind" ;;
  esac
  curl_internal \
    -X PATCH \
    -H 'Content-Type: application/json' \
    -d "{\"enabled\":${enabled},\"healthy\":${healthy}}" \
    "${BIZ_URL}/internal/wire/admin/${path}/${region}/${node}/status" >/dev/null
}

set_relay_nodes() {
  local enabled="$1"
  local -a nodes=()
  while IFS= read -r node; do
    [[ -n "$node" ]] && nodes+=("$node")
  done < <(list_region_node_ids relay dev)
  [[ ${#nodes[@]} -gt 0 ]] || fail "no live relay nodes found in region=dev"
  local node
  for node in "${nodes[@]}"; do
    patch_node_status relay dev "$node" "$enabled" true
  done
}

set_derp_nodes() {
  local enabled="$1"
  local -a nodes=()
  while IFS= read -r node; do
    [[ -n "$node" ]] && nodes+=("$node")
  done < <(list_region_node_ids derp dev)
  [[ ${#nodes[@]} -gt 0 ]] || fail "no live derp nodes found in region=dev"
  local node
  for node in "${nodes[@]}"; do
    patch_node_status derp dev "$node" "$enabled" true
  done
}

sudo_run() {
  if [[ -n "$SUDO_PASSWORD" ]]; then
    printf '%s\n' "$SUDO_PASSWORD" | sudo -S "$@"
  else
    sudo "$@"
  fi
}

install_macos_service_direct() {
  if [[ "$MANAGE_MAC_SERVICE" != "1" ]]; then
    return 0
  fi
  echo "+ install macOS service with direct UDP enabled"
  if [[ -n "$MACOS_SERVICE_BINARY" ]]; then
    sudo_run env SLAN_DIRECT_UDP_ENDPOINT= scripts/install_macos_service.sh --binary "$MACOS_SERVICE_BINARY"
  else
    sudo_run env SLAN_DIRECT_UDP_ENDPOINT= scripts/install_macos_service.sh --app "$MACOS_APP_PATH"
  fi
}

install_macos_service_no_direct() {
  if [[ "$MANAGE_MAC_SERVICE" != "1" ]]; then
    echo "SLAN_PATH_MATRIX_MANAGE_MAC_SERVICE is not 1; assuming current Mac service direct UDP state is already suitable"
    return 0
  fi
  echo "+ install macOS service with direct UDP disabled"
  if [[ -n "$MACOS_SERVICE_BINARY" ]]; then
    sudo_run env \
      SLAN_DIRECT_UDP_ENDPOINT=disabled \
      SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST="${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST:-}" \
      scripts/install_macos_service.sh --binary "$MACOS_SERVICE_BINARY"
  else
    sudo_run env \
      SLAN_DIRECT_UDP_ENDPOINT=disabled \
      SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST="${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST:-}" \
      scripts/install_macos_service.sh --app "$MACOS_APP_PATH"
  fi
}

restore_nodes() {
  if [[ "$RESTORE_ACTIVE" != "1" ]]; then
    return 0
  fi
  set +e
  set_relay_nodes true
  set_derp_nodes true
  if [[ "$MANAGE_MAC_SERVICE" == "1" ]]; then
    install_macos_service_direct
  fi
  set -e
}
trap restore_nodes EXIT

run_socket_check() {
  local mode="$1"
  shift
  local email="path-${mode}-$(date +%s%N)@example.com"
  local expected_service_bin="${MACOS_SERVICE_BINARY:-}"
  local -a env_args
  if [[ -z "$expected_service_bin" && -d "$MACOS_APP_PATH" ]]; then
    expected_service_bin="${MACOS_APP_PATH%/}/Contents/MacOS/client-core-service"
  fi
  env_args=(
    SLAN_USE_EXISTING_MAC_SERVICE=1
    SLAN_MAC_SERVICE_HOST="$MAC_SERVICE_HOST"
    SLAN_MACOS_APP_PATH="$MACOS_APP_PATH"
    SLAN_BIZ_URL="$BIZ_URL"
    SLAN_ANDROID_BIZ_URL="$ANDROID_BIZ_URL"
    SLAN_TEST_EMAIL="$email"
    SLAN_CLEANUP_REMOTE_TEST_DEVICES=1
  )
  if [[ -n "$expected_service_bin" ]]; then
    env_args+=(SLAN_CLIENT_CORE_SERVICE_BIN="$expected_service_bin")
  fi
  echo "==> mac/android path check: $mode"
  env \
    "${env_args[@]}" \
    "$@" \
    scripts/mac_android_socket_check.sh
}

run_direct() {
  set_relay_nodes true
  set_derp_nodes true
  install_macos_service_direct
  run_socket_check direct \
    SLAN_EXPECT_ANDROID_PATH_KIND_CONTAINS=direct_udp \
    SLAN_EXPECT_ANDROID_DIRECT_CANDIDATES_CONTAINS="[0-9]" \
    SLAN_EXPECT_ANDROID_DIRECT_READY_MIN="${SLAN_EXPECT_DIRECT_READY_MIN:-1}" \
    SLAN_EXPECT_ANDROID_DIRECT_FRAMES_SENT_MIN="${SLAN_EXPECT_DIRECT_FRAMES_SENT_MIN:-1}"
}

run_udp_relay() {
  set_relay_nodes true
  set_derp_nodes true
  install_macos_service_no_direct
  run_socket_check udp-relay \
    SLAN_SKIP_ANDROID_TCP_SEND=1 \
    SLAN_EXPECT_ANDROID_RELAY_URL_CONTAINS="udp://" \
    SLAN_EXPECT_ANDROID_PATH_KIND_CONTAINS=relay_udp \
    ${SLAN_EXPECT_RELAY_FRAMES_SENT_MIN:+SLAN_EXPECT_ANDROID_RELAY_FRAMES_SENT_MIN="$SLAN_EXPECT_RELAY_FRAMES_SENT_MIN"}
}

run_tcp_relay() {
  set_relay_nodes false
  set_derp_nodes true
  export SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST="${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST:-derp_tcp_tls_443}"
  install_macos_service_no_direct
  run_socket_check tcp-relay \
    SLAN_SKIP_ANDROID_UDP_SEND=1 \
    SLAN_EXPECT_ANDROID_RELAY_URL_CONTAINS="derp://" \
    SLAN_EXPECT_ANDROID_PATH_KIND_CONTAINS=derp_tcp_tls_443 \
    SLAN_EXPECT_ANDROID_DERP_PEER_IPS_CONTAINS="node-" \
    ${SLAN_EXPECT_RELAY_TCP_FRAMES_RECEIVED_MIN:+SLAN_EXPECT_ANDROID_RELAY_TCP_FRAMES_RECEIVED_MIN="$SLAN_EXPECT_RELAY_TCP_FRAMES_RECEIVED_MIN"}
}

IFS=',' read -r -a mode_list <<<"$MODES"
for mode in "${mode_list[@]}"; do
  mode="$(printf '%s' "$mode" | tr -d '[:space:]')"
  case "$mode" in
    direct) run_direct ;;
    udp-relay) run_udp_relay ;;
    tcp-relay) run_tcp_relay ;;
    "") ;;
    *) fail "unknown path matrix mode: $mode" ;;
  esac
done

echo "macAndroidPathMatrix: ok modes=$MODES"
