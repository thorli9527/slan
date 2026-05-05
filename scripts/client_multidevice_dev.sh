#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="$ROOT_DIR/client_v2/app_flutter"
RUST_DIR="$ROOT_DIR/client_v2/rust"

SERVICE_BIND="${SLAN_CLIENT_CORE_SERVICE_BIND:-0.0.0.0:46392}"
SERVICE_PORT="${SERVICE_BIND##*:}"
TARGETS="${SLAN_MULTI_TARGETS:-macos,ios,android}"
START_SERVICE="${SLAN_MULTI_START_SERVICE:-1}"

local_ip() {
  ipconfig getifaddr en0 2>/dev/null \
    || ipconfig getifaddr en1 2>/dev/null \
    || ifconfig | awk '/inet / && $2 !~ /^127\./ { print $2; exit }'
}

MAC_IP="${SLAN_MULTI_MAC_IP:-$(local_ip || true)}"
if [[ -z "$MAC_IP" ]]; then
  echo "failed to detect Mac LAN IP; set SLAN_MULTI_MAC_IP=192.168.x.x" >&2
  exit 1
fi

MACOS_SERVICE_HOST="${SLAN_MULTI_MACOS_SERVICE_HOST:-127.0.0.1:${SERVICE_PORT}}"
IOS_SERVICE_HOST="${SLAN_MULTI_IOS_SERVICE_HOST:-${MAC_IP}:${SERVICE_PORT}}"
ANDROID_SERVICE_HOST="${SLAN_MULTI_ANDROID_SERVICE_HOST:-10.0.2.2:${SERVICE_PORT}}"

PIDS=()

cleanup() {
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
}
trap cleanup EXIT INT TERM

run_flutter() {
  local device="$1"
  local service_host="$2"
  (
    cd "$APP_DIR"
    echo "+ flutter run -d $device --dart-define=SLAN_CLIENT_CORE_SERVICE_HOST=$service_host"
    flutter run -d "$device" \
      --dart-define="SLAN_CLIENT_CORE_SERVICE_HOST=$service_host"
  ) &
  PIDS+=("$!")
}

if [[ "$START_SERVICE" == "1" ]]; then
  (
    cd "$RUST_DIR"
    echo "+ SLAN_CLIENT_CORE_SERVICE_HOST=$SERVICE_BIND cargo run -p client-core-service"
    SLAN_CLIENT_CORE_SERVICE_HOST="$SERVICE_BIND" cargo run -p client-core-service
  ) &
  PIDS+=("$!")
  sleep "${SLAN_MULTI_SERVICE_BOOT_WAIT:-2}"
else
  echo "skip service start; expecting client-core-service on $SERVICE_BIND"
fi

IFS=',' read -ra TARGET_LIST <<< "$TARGETS"
for target in "${TARGET_LIST[@]}"; do
  target="$(echo "$target" | xargs)"
  case "$target" in
    macos)
      run_flutter "${SLAN_MULTI_MACOS_DEVICE:-macos}" "$MACOS_SERVICE_HOST"
      ;;
    ios)
      run_flutter "${SLAN_MULTI_IOS_DEVICE:-ios}" "$IOS_SERVICE_HOST"
      ;;
    android)
      run_flutter "${SLAN_MULTI_ANDROID_DEVICE:-android}" "$ANDROID_SERVICE_HOST"
      ;;
    "")
      ;;
    *)
      echo "unsupported target '$target'; use macos,ios,android or set device ids via env" >&2
      exit 1
      ;;
  esac
done

echo "client multi-device dev hosts:"
echo "  service bind : $SERVICE_BIND"
echo "  macOS host   : $MACOS_SERVICE_HOST"
echo "  iOS host     : $IOS_SERVICE_HOST"
echo "  Android host : $ANDROID_SERVICE_HOST"
echo "  Mac LAN IP   : $MAC_IP"

wait
