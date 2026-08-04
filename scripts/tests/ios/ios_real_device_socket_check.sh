#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

list_flutter_ios_devices() {
  flutter devices 2>/dev/null | grep -E '• ios •' || true
}

pick_single_real_ios_device() {
  local matches
  matches="$(flutter devices 2>/dev/null | grep -E '• ios •' | grep -v 'simulator' || true)"
  if [[ -z "$matches" ]]; then
    return 1
  fi
  local count
  count="$(printf '%s\n' "$matches" | sed '/^$/d' | wc -l | tr -d ' ')"
  if [[ "$count" != "1" ]]; then
    return 2
  fi
  printf '%s\n' "$matches" | awk -F'•' 'NR==1 {gsub(/^ +| +$/, "", $2); print $2}'
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  SLAN_IOS_FLUTTER_DEVICE="<real ios device id or name>" bash scripts/ios_real_device_socket_check.sh

Required environment variables:
  SLAN_IOS_FLUTTER_DEVICE   Real iPhone/iPad target recognized by `flutter devices`

Optional environment variables:
  SLAN_ANDROID_FLUTTER_DEVICE
  SLAN_BIZ_URL
  SLAN_OPS_BASE_URL
EOF
  exit 0
fi

if [[ -z "${SLAN_IOS_FLUTTER_DEVICE:-}" ]]; then
  if detected_device="$(pick_single_real_ios_device)"; then
    SLAN_IOS_FLUTTER_DEVICE="$detected_device"
    export SLAN_IOS_FLUTTER_DEVICE
    echo "==> auto-selected real iOS device: ${SLAN_IOS_FLUTTER_DEVICE}"
  else
    status=$?
    echo "missing SLAN_IOS_FLUTTER_DEVICE" >&2
    if [[ "$status" == "1" ]]; then
      echo "no real iOS devices detected by 'flutter devices'" >&2
    else
      echo "multiple real iOS devices detected; set SLAN_IOS_FLUTTER_DEVICE explicitly" >&2
    fi
    echo "visible iOS devices:" >&2
    list_flutter_ios_devices >&2
    exit 1
  fi
fi

if flutter devices 2>/dev/null | grep -F "${SLAN_IOS_FLUTTER_DEVICE}" | grep -q 'simulator'; then
  echo "SLAN_IOS_FLUTTER_DEVICE points to an iOS simulator; a real device is required for true PacketTunnel UDP/TCP tests" >&2
  echo "visible iOS devices:" >&2
  list_flutter_ios_devices >&2
  exit 1
fi

echo "==> real iOS device target: ${SLAN_IOS_FLUTTER_DEVICE}"
echo "==> forcing true PacketTunnel UDP/TCP send path"
echo "==> default biz url: ${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"

cd "$ROOT_DIR"
SLAN_IOS_SEND_UDP=1 bash scripts/ios_android_socket_check.sh
