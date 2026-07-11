#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

DEFAULT_RESULT_ROOT="${TMPDIR:-/tmp}/slan-current-regression"
RUN_TIMESTAMP="$(date '+%Y%m%d_%H%M%S')"
RESULT_ROOT="${SLAN_CURRENT_REGRESSION_RESULT_ROOT:-$DEFAULT_RESULT_ROOT}"
RESULT_DIR="${SLAN_CURRENT_REGRESSION_RESULT_DIR:-$RESULT_ROOT/runs/$RUN_TIMESTAMP}"
LATEST_LINK="${SLAN_CURRENT_REGRESSION_LATEST_LINK:-$RESULT_ROOT/latest}"
SUMMARY_FILE="$RESULT_DIR/summary.txt"
LOG_DIR="$RESULT_DIR/logs"

RUN_ANDROID_DUAL="${SLAN_RUN_CURRENT_ANDROID_DUAL:-1}"
RUN_ANDROID_DUAL_PHASE2_ONLY="${SLAN_RUN_CURRENT_ANDROID_DUAL_PHASE2_ONLY:-0}"
RUN_IOS_DUAL="${SLAN_RUN_CURRENT_IOS_DUAL:-1}"
RUN_IOS_DUAL_QUICK="${SLAN_RUN_CURRENT_IOS_DUAL_QUICK:-1}"
RUN_IOS_DUAL_FLUTTER_MESSAGE="${SLAN_RUN_CURRENT_IOS_DUAL_FLUTTER_MESSAGE:-1}"
RUN_LINUX_DUAL_DOCKER="${SLAN_RUN_CURRENT_LINUX_DUAL_DOCKER:-1}"
RUN_MAC_ANDROID="${SLAN_RUN_CURRENT_MAC_ANDROID:-1}"
RUN_MAC_ANDROID_ACTIVE="${SLAN_RUN_CURRENT_MAC_ANDROID_ACTIVE:-1}"
RUN_MAC_IOS_FAST="${SLAN_RUN_CURRENT_MAC_IOS_FAST:-1}"
RUN_MAC_LOCAL_DNS_SMOKE="${SLAN_RUN_CURRENT_MAC_LOCAL_DNS_SMOKE:-1}"
RUN_MAC_REMOTE_LINUX="${SLAN_RUN_CURRENT_MAC_REMOTE_LINUX:-0}"
RUN_IOS_ANDROID_PARTIAL="${SLAN_RUN_CURRENT_IOS_ANDROID_PARTIAL:-1}"
RUN_TRI_MESSAGE="${SLAN_RUN_CURRENT_TRI_MESSAGE:-1}"
RUN_IOS_TRI_MATRIX="${SLAN_RUN_CURRENT_IOS_TRI_MATRIX:-1}"
RUN_LINUX_DUAL_DOCKER_PACKET="${SLAN_RUN_CURRENT_LINUX_DUAL_DOCKER_PACKET:-0}"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/tests/matrix/current_client_regression.sh

Purpose:
  Run the client regression set that is currently feasible on this machine:
  dual Android, dual iOS simulator, dual Docker Linux, Mac + Android,
  Mac + iOS fast, optional Mac + remote Linux, iOS + Android partial
  readiness, tri-device control-message smoke, and the local iOS
  tri-client protocol matrix.

This wrapper intentionally excludes checks that require a real iOS device.

Optional environment variables:
  SLAN_RUN_CURRENT_ANDROID_DUAL=1|0
  SLAN_RUN_CURRENT_ANDROID_DUAL_PHASE2_ONLY=1|0
  SLAN_RUN_CURRENT_IOS_DUAL=1|0
  SLAN_RUN_CURRENT_IOS_DUAL_QUICK=1|0
  SLAN_RUN_CURRENT_IOS_DUAL_FLUTTER_MESSAGE=1|0
  SLAN_RUN_CURRENT_LINUX_DUAL_DOCKER=1|0
  SLAN_RUN_CURRENT_LINUX_DUAL_DOCKER_PACKET=1|0
  SLAN_RUN_CURRENT_MAC_ANDROID=1|0
  SLAN_RUN_CURRENT_MAC_ANDROID_ACTIVE=1|0
  SLAN_RUN_CURRENT_MAC_IOS_FAST=1|0
  SLAN_RUN_CURRENT_MAC_LOCAL_DNS_SMOKE=1|0
  SLAN_RUN_CURRENT_MAC_REMOTE_LINUX=1|0
  SLAN_RUN_CURRENT_IOS_ANDROID_PARTIAL=1|0
  SLAN_RUN_CURRENT_TRI_MESSAGE=1|0
  SLAN_RUN_CURRENT_IOS_TRI_MATRIX=1|0
  SLAN_CURRENT_REGRESSION_RESULT_ROOT
  SLAN_CURRENT_REGRESSION_RESULT_DIR
  SLAN_CURRENT_REGRESSION_LATEST_LINK
  SLAN_BIZ_URL
  SLAN_WEB_BASE_URL
  SLAN_SUDO_PASSWORD
  SLAN_ANDROID_FLUTTER_DEVICE
  SLAN_IOS_FLUTTER_DEVICE
  SLAN_REMOTE_LINUX_HOST
  SLAN_REMOTE_LINUX_USER
  SLAN_REMOTE_LINUX_PASSWORD or SLAN_REMOTE_LINUX_SSH_KEY

Examples:
  bash scripts/tests/matrix/current_client_regression.sh
  SLAN_CURRENT_REGRESSION_RESULT_ROOT=/tmp/slan-regression bash scripts/tests/matrix/current_client_regression.sh
  SLAN_RUN_CURRENT_LINUX_DUAL_DOCKER=0 bash scripts/tests/matrix/current_client_regression.sh
  SLAN_RUN_CURRENT_IOS_ANDROID_PARTIAL=0 bash scripts/tests/matrix/current_client_regression.sh
  SLAN_RUN_CURRENT_MAC_ANDROID_ACTIVE=0 bash scripts/tests/matrix/current_client_regression.sh
  SLAN_RUN_CURRENT_MAC_REMOTE_LINUX=1 bash scripts/tests/matrix/current_client_regression.sh
EOF
  exit 0
fi

log() {
  printf '==> %s\n' "$*"
}

record_summary() {
  printf '%s\n' "$*" >> "$SUMMARY_FILE"
}

run_step() {
  local name="$1"
  local slug="$2"
  shift 2
  local log_file="$LOG_DIR/${slug}.log"
  log "$name"
  record_summary "[RUN ] $name"
  record_summary "       log=$log_file"
  if (
    "$@"
  ) > >(tee "$log_file") 2>&1; then
    record_summary "[PASS] $name"
    return 0
  fi
  local status=$?
  record_summary "[FAIL] $name exit=$status"
  record_summary "       log=$log_file"
  return "$status"
}

run_in_root() {
  (
    cd "$ROOT_DIR"
    "$@"
  )
}

export SLAN_BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
export SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"

mkdir -p "$(dirname "$RESULT_DIR")"
mkdir -p "$RESULT_DIR"
mkdir -p "$LOG_DIR"
{
  printf 'slan current client regression\n'
  printf 'run_timestamp=%s\n' "$RUN_TIMESTAMP"
  printf 'result_root=%s\n' "$RESULT_ROOT"
  printf 'result_dir=%s\n' "$RESULT_DIR"
  printf 'latest_link=%s\n' "$LATEST_LINK"
  printf 'log_dir=%s\n' "$LOG_DIR"
  printf 'control_url=%s\n' "$SLAN_BIZ_URL"
  printf '\n'
} > "$SUMMARY_FILE"

ln -sfn "$RESULT_DIR" "$LATEST_LINK"

log "current regression toggles: android-dual=${RUN_ANDROID_DUAL} android-dual-phase2-only=${RUN_ANDROID_DUAL_PHASE2_ONLY} ios-dual=${RUN_IOS_DUAL} ios-dual-quick=${RUN_IOS_DUAL_QUICK} ios-dual-message=${RUN_IOS_DUAL_FLUTTER_MESSAGE} linux-dual-docker=${RUN_LINUX_DUAL_DOCKER} linux-dual-docker-packet=${RUN_LINUX_DUAL_DOCKER_PACKET} mac-android=${RUN_MAC_ANDROID} mac-android-active=${RUN_MAC_ANDROID_ACTIVE} mac-ios-fast=${RUN_MAC_IOS_FAST} mac-remote-linux=${RUN_MAC_REMOTE_LINUX} ios-android-partial=${RUN_IOS_ANDROID_PARTIAL} tri-message=${RUN_TRI_MESSAGE} ios-tri-matrix=${RUN_IOS_TRI_MATRIX}"
log "control url: ${SLAN_BIZ_URL}"
log "result dir: ${RESULT_DIR}"
log "latest link: ${LATEST_LINK}"
log "run mac local dns smoke: ${RUN_MAC_LOCAL_DNS_SMOKE}"

if [[ "$RUN_ANDROID_DUAL_PHASE2_ONLY" == "1" ]]; then
  run_step "Run dual Android phase2 bidirectional message chain" android_dual_phase2 run_in_root env \
    SLAN_ANDROID_DUAL_PHASE34_RELAY_TRANSPORT_ALLOWLIST="${SLAN_ANDROID_DUAL_PHASE34_RELAY_TRANSPORT_ALLOWLIST:-udp}" \
    bash scripts/tests/android/android_dual_fast_check.sh --phase2-only
fi

if [[ "$RUN_ANDROID_DUAL" == "1" ]]; then
  run_step "Run dual Android full chain" android_dual run_in_root env \
    SLAN_ANDROID_DUAL_PHASE34_RELAY_TRANSPORT_ALLOWLIST="${SLAN_ANDROID_DUAL_PHASE34_RELAY_TRANSPORT_ALLOWLIST:-udp}" \
    bash scripts/tests/android/android_dual_fast_check.sh --full-stable
fi

if [[ "$RUN_IOS_DUAL" == "1" ]]; then
  if [[ "$RUN_IOS_DUAL_QUICK" == "1" ]]; then
    run_step "Run dual iOS simulator DNS/ACL/MQTT quick chain" ios_dual_quick run_in_root env \
      SLAN_RUN_IOS_START_SIMS=1 \
      bash scripts/tests/ios/ios_dual_fast_check.sh --quick-only
  fi
  if [[ "$RUN_IOS_DUAL_FLUTTER_MESSAGE" == "1" ]]; then
    run_step "Run dual iOS simulator Flutter message chain" ios_dual_message run_in_root env \
      SLAN_RUN_IOS_START_SIMS=0 \
      bash scripts/tests/ios/ios_dual_fast_check.sh --message-only
  fi
fi

if [[ "$RUN_LINUX_DUAL_DOCKER" == "1" ]]; then
  run_step "Run dual Docker Linux DNS/ACL/message control-plane chain" linux_dual_docker run_in_root bash scripts/tests/linux/linux_dual_docker_integration.sh
fi

if [[ "$RUN_LINUX_DUAL_DOCKER_PACKET" == "1" ]]; then
  run_step "Run dual Docker Linux UDP/TCP packet chain" linux_dual_docker_packet run_in_root bash scripts/tests/linux/linux_dual_docker_packet_smoke.sh
fi

if [[ "$RUN_MAC_ANDROID" == "1" ]]; then
  run_step "Run Mac + Android passive-direction chain" mac_android_passive run_in_root env \
    SLAN_RUN_MAC_LOCAL_DNS_SMOKE=0 \
    bash scripts/tests/matrix/mac_android_fast_check.sh
fi

if [[ "$RUN_MAC_ANDROID_ACTIVE" == "1" ]]; then
  run_step "Run Mac -> Android active socket chain" mac_android_active run_in_root env \
    SLAN_RUN_MAC_ANDROID_ACTIVE=1 \
    SLAN_RUN_MAC_IOS_ACTIVE=0 \
    SLAN_RUN_MAC_LINUX_ACTIVE=0 \
    bash scripts/tests/matrix/mac_active_socket_matrix.sh
fi

if [[ "$RUN_MAC_IOS_FAST" == "1" ]]; then
  run_step "Run Mac + iOS fast chain" mac_ios_fast run_in_root env \
    SLAN_RUN_MAC_LOCAL_DNS_SMOKE=0 \
    bash scripts/tests/matrix/mac_ios_fast_check.sh
fi

if [[ "$RUN_MAC_LOCAL_DNS_SMOKE" == "1" ]]; then
  run_step "Run macOS local DNS smoke" mac_local_dns run_in_root env \
    SLAN_RUN_LOCAL_DNS_SMOKE=1 \
    bash scripts/tests/macos/macos_service_smoke.sh
fi

if [[ "$RUN_MAC_REMOTE_LINUX" == "1" ]]; then
  run_step "Run Mac + remote Linux fast chain" mac_remote_linux run_in_root bash scripts/mac_remote_linux_fast_check.sh
fi

if [[ "$RUN_IOS_ANDROID_PARTIAL" == "1" ]]; then
  run_step "Run iOS + Android partial readiness chain" ios_android_partial run_in_root bash scripts/tests/ios/ios_android_socket_check.sh
fi

if [[ "$RUN_TRI_MESSAGE" == "1" ]]; then
  run_step "Run Mac + Android + iOS tri-device message chain" tri_message run_in_root env \
    SLAN_CLIENT_CORE_SERVICE_BIN="$ROOT_DIR/client_v2/rust/target/debug/client-core-service" \
    bash scripts/tests/matrix/mac_android_ios_message_check.sh
fi

if [[ "$RUN_IOS_TRI_MATRIX" == "1" ]]; then
  run_step "Run iOS tri-client local protocol matrix" ios_tri_matrix run_in_root bash scripts/tests/ios/ios_triclient_packet_matrix.sh
fi

log "current client regression complete"
record_summary
record_summary "current client regression complete"
record_summary "result_dir=$RESULT_DIR"
record_summary "latest_link=$LATEST_LINK"
