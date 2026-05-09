#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="$ROOT_DIR/client_v2/app_flutter"
RUST_DIR="$ROOT_DIR/client_v2/rust"

SERVICE_BIND="${SLAN_CLIENT_CORE_SERVICE_BIND:-0.0.0.0:46392}"
SERVICE_PORT="${SERVICE_BIND##*:}"
TARGETS="${SLAN_MULTI_TARGETS:-macos,ios,android}"
START_SERVICE="${SLAN_MULTI_START_SERVICE:-1}"
BIZ_URL="${SLAN_BIZ_URL:-http://127.0.0.1:28080}"
MACOS_DEVICE_ID="${SLAN_MULTI_MACOS_DEVICE_ID:-}"
IOS_DEVICE_ID="${SLAN_MULTI_IOS_DEVICE_ID:-}"
ANDROID_DEVICE_ID="${SLAN_MULTI_ANDROID_DEVICE_ID:-}"

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
# Mobile targets use embedded client-core-service through client-core-ffi.
# Android emulator reaches the host through 10.0.2.2; iOS simulator can use 127.0.0.1.
IOS_CONTROL_BASE_URL="${SLAN_MULTI_IOS_CONTROL_BASE_URL:-$BIZ_URL}"
ANDROID_CONTROL_BASE_URL="${SLAN_MULTI_ANDROID_CONTROL_BASE_URL:-http://10.0.2.2:28080}"

PIDS=()

maybe_dart_define() {
  local key="$1"
  local value="$2"
  if [[ -n "$value" ]]; then
    printf '%s\n' "--dart-define=$key=$value"
  fi
}

cleanup() {
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
}
trap cleanup EXIT INT TERM

run_flutter() {
  local device="$1"
  shift
  (
    cd "$APP_DIR"
    echo "+ flutter run -d $device $*"
    flutter run -d "$device" "$@"
  ) &
  PIDS+=("$!")
}

if [[ "$START_SERVICE" == "1" ]]; then
  (
    cd "$RUST_DIR"
    echo "+ SLAN_CLIENT_CORE_SERVICE_HOST=$SERVICE_BIND cargo run -p client-core-service"
    if [[ -n "$MACOS_DEVICE_ID" ]]; then
      SLAN_CLIENT_CORE_SERVICE_HOST="$SERVICE_BIND" \
        SLAN_CLIENT_DEVICE_ID="$MACOS_DEVICE_ID" \
        SLAN_CONTROL_BASE_URL="$BIZ_URL" \
        cargo run -p client-core-service
    else
      SLAN_CLIENT_CORE_SERVICE_HOST="$SERVICE_BIND" \
        SLAN_CONTROL_BASE_URL="$BIZ_URL" \
        cargo run -p client-core-service
    fi
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
      run_flutter "${SLAN_MULTI_MACOS_DEVICE:-macos}" \
        --dart-define="SLAN_CLIENT_CORE_SERVICE_HOST=$MACOS_SERVICE_HOST" \
        --dart-define="SLAN_CONTROL_BASE_URL=$BIZ_URL"
      ;;
    ios)
      mapfile -t ios_extra_args < <(maybe_dart_define "SLAN_TEST_DEVICE_ID" "$IOS_DEVICE_ID")
      run_flutter "${SLAN_MULTI_IOS_DEVICE:-ios}" \
        --dart-define="SLAN_EMBEDDED_CONTROL_BASE_URL=$IOS_CONTROL_BASE_URL" \
        --dart-define="SLAN_CONTROL_BASE_URL=$IOS_CONTROL_BASE_URL" \
        "${ios_extra_args[@]}"
      ;;
    android)
      mapfile -t android_extra_args < <(maybe_dart_define "SLAN_TEST_DEVICE_ID" "$ANDROID_DEVICE_ID")
      run_flutter "${SLAN_MULTI_ANDROID_DEVICE:-android}" \
        --dart-define="SLAN_EMBEDDED_CONTROL_BASE_URL=$ANDROID_CONTROL_BASE_URL" \
        --dart-define="SLAN_CONTROL_BASE_URL=$ANDROID_CONTROL_BASE_URL" \
        "${android_extra_args[@]}"
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
echo "  server-biz   : $BIZ_URL"
echo "  iOS control  : $IOS_CONTROL_BASE_URL"
echo "  Android ctrl : $ANDROID_CONTROL_BASE_URL"
echo "  Mac LAN IP   : $MAC_IP"
echo "  macOS dev id : ${MACOS_DEVICE_ID:-auto}"
echo "  iOS dev id   : ${IOS_DEVICE_ID:-auto}"
echo "  Android id   : ${ANDROID_DEVICE_ID:-auto}"

wait
