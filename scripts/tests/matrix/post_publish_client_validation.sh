#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

DEFAULT_RESULT_ROOT="${TMPDIR:-/tmp}/slan-post-publish"
RUN_TIMESTAMP="$(date '+%Y%m%d_%H%M%S')"
RESULT_ROOT="${SLAN_POST_PUBLISH_RESULT_ROOT:-$DEFAULT_RESULT_ROOT}"
RESULT_DIR="${SLAN_POST_PUBLISH_RESULT_DIR:-$RESULT_ROOT/runs/$RUN_TIMESTAMP}"
LATEST_LINK="${SLAN_POST_PUBLISH_LATEST_LINK:-$RESULT_ROOT/latest}"
SUMMARY_FILE="$RESULT_DIR/summary.txt"
LOG_DIR="$RESULT_DIR/logs"

BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
OPS_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
MQTT_HOST="${SLAN_EXPECT_MQTT_HOST:-$SLAN_DEFAULT_MQTT_HOST}"
# Dual-docker smoke currently targets the remote publish surface exposed on the
# separate Web port rather than the standard client Web Console endpoint.
DUAL_DOCKER_WEB_BASE_URL="${SLAN_DUAL_DOCKER_WEB_BASE_URL:-http://${SLAN_DEFAULT_MQTT_HOST}:28081}"

RUN_LINUX="${SLAN_RUN_POST_PUBLISH_LINUX:-1}"
RUN_LINUX_DUAL_DOCKER="${SLAN_RUN_POST_PUBLISH_LINUX_DUAL_DOCKER:-1}"
RUN_ANDROID_DUAL_QUICK="${SLAN_RUN_POST_PUBLISH_ANDROID_DUAL_QUICK:-0}"
RUN_IOS_DUAL_QUICK="${SLAN_RUN_POST_PUBLISH_IOS_DUAL_QUICK:-0}"
RUN_MAC_ANDROID_QUICK="${SLAN_RUN_POST_PUBLISH_MAC_ANDROID_QUICK:-0}"
RUN_MAC_IOS="${SLAN_RUN_POST_PUBLISH_MAC_IOS:-1}"
RUN_STABLE_MATRIX="${SLAN_RUN_POST_PUBLISH_STABLE_MATRIX:-0}"
IOS_MODE="${SLAN_POST_PUBLISH_IOS_MODE:-quick-only}"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/post_publish_client_validation.sh

Optional environment variables:
  SLAN_RUN_POST_PUBLISH_LINUX=1|0
  SLAN_RUN_POST_PUBLISH_LINUX_DUAL_DOCKER=1|0
  SLAN_RUN_POST_PUBLISH_ANDROID_DUAL_QUICK=1|0
  SLAN_RUN_POST_PUBLISH_IOS_DUAL_QUICK=1|0
  SLAN_RUN_POST_PUBLISH_MAC_ANDROID_QUICK=1|0
  SLAN_RUN_POST_PUBLISH_MAC_IOS=1|0
  SLAN_RUN_POST_PUBLISH_STABLE_MATRIX=1|0
  SLAN_POST_PUBLISH_IOS_MODE=quick-only|message-only|default
  SLAN_POST_PUBLISH_RESULT_ROOT
  SLAN_POST_PUBLISH_RESULT_DIR
  SLAN_POST_PUBLISH_LATEST_LINK

Examples:
  bash scripts/post_publish_client_validation.sh
  SLAN_POST_PUBLISH_RESULT_ROOT=/tmp/slan-post-publish bash scripts/post_publish_client_validation.sh
  SLAN_RUN_POST_PUBLISH_LINUX_DUAL_DOCKER=0 bash scripts/post_publish_client_validation.sh
  SLAN_RUN_POST_PUBLISH_STABLE_MATRIX=1 bash scripts/post_publish_client_validation.sh
  SLAN_RUN_POST_PUBLISH_IOS_DUAL_QUICK=1 bash scripts/post_publish_client_validation.sh
  SLAN_RUN_POST_PUBLISH_IOS_DUAL_QUICK=1 SLAN_POST_PUBLISH_IOS_MODE=message-only bash scripts/post_publish_client_validation.sh
  SLAN_RUN_POST_PUBLISH_LINUX=0 SLAN_RUN_POST_PUBLISH_MAC_IOS=0 bash scripts/post_publish_client_validation.sh
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

run_linux() {
  (
    cd "$ROOT_DIR"
    SLAN_TEST_API_URL="$BIZ_URL" \
      SLAN_TEST_WEB_URL="$WEB_BASE_URL" \
      SLAN_TEST_OPS_URL="$OPS_URL" \
      SLAN_TEST_MQTT_HOST="$MQTT_HOST" \
      bash scripts/vm_client_matrix_test.sh linux
  )
}

run_linux_dual_docker() {
  (
    cd "$ROOT_DIR"
    SLAN_BIZ_URL="$BIZ_URL" \
      SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$DUAL_DOCKER_WEB_BASE_URL}" \
      bash scripts/linux_dual_docker_packet_smoke.sh
  )
}

run_stable_matrix() {
  (
    cd "$ROOT_DIR"
    SLAN_BIZ_URL="$BIZ_URL" \
      SLAN_WEB_BASE_URL="$WEB_BASE_URL" \
      bash scripts/mac_android_ios_stable_check.sh
  )
}

run_android_dual_quick() {
  (
    cd "$ROOT_DIR"
    SLAN_BIZ_URL="$BIZ_URL" \
      SLAN_WEB_BASE_URL="$WEB_BASE_URL" \
      bash scripts/android_dual_fast_check.sh
  )
}

run_ios_dual_quick() {
  local mode_arg=""
  case "$IOS_MODE" in
    default)
      ;;
    quick-only|message-only)
      mode_arg="--${IOS_MODE}"
      ;;
    *)
      printf 'unsupported SLAN_POST_PUBLISH_IOS_MODE: %s\n' "$IOS_MODE" >&2
      exit 1
      ;;
  esac
  (
    cd "$ROOT_DIR"
    SLAN_BIZ_URL="$BIZ_URL" \
      SLAN_WEB_BASE_URL="$WEB_BASE_URL" \
      SLAN_EXPECT_MQTT_HOST="$MQTT_HOST" \
      bash scripts/ios_dual_fast_check.sh ${mode_arg:+"$mode_arg"}
  )
}

run_mac_android_quick() {
  (
    cd "$ROOT_DIR"
    SLAN_BIZ_URL="$BIZ_URL" \
      SLAN_WEB_BASE_URL="$WEB_BASE_URL" \
      bash scripts/mac_android_fast_check.sh
  )
}

run_mac_ios() {
  (
    cd "$ROOT_DIR"
    SLAN_BIZ_URL="$BIZ_URL" \
      SLAN_WEB_BASE_URL="$WEB_BASE_URL" \
      SLAN_EXPECT_MQTT_HOST="$MQTT_HOST" \
      bash scripts/mac_ios_fast_check.sh
  )
}

mkdir -p "$(dirname "$RESULT_DIR")"
mkdir -p "$RESULT_DIR"
mkdir -p "$LOG_DIR"
{
  printf 'slan post publish client validation\n'
  printf 'run_timestamp=%s\n' "$RUN_TIMESTAMP"
  printf 'result_root=%s\n' "$RESULT_ROOT"
  printf 'result_dir=%s\n' "$RESULT_DIR"
  printf 'latest_link=%s\n' "$LATEST_LINK"
  printf 'log_dir=%s\n' "$LOG_DIR"
  printf 'control_url=%s\n' "$BIZ_URL"
  printf 'web_url=%s\n' "$WEB_BASE_URL"
  printf '\n'
} > "$SUMMARY_FILE"

ln -sfn "$RESULT_DIR" "$LATEST_LINK"

log "result dir: ${RESULT_DIR}"
log "latest link: ${LATEST_LINK}"

if [[ "$RUN_LINUX" == "1" ]]; then
  run_step "Linux client validation" linux_client run_linux
fi

if [[ "$RUN_LINUX_DUAL_DOCKER" == "1" ]]; then
  run_step "Linux dual-Docker full validation" linux_dual_docker run_linux_dual_docker
fi

if [[ "$RUN_STABLE_MATRIX" == "1" ]]; then
  run_step "Stable multi-client validation" stable_matrix run_stable_matrix
fi

if [[ "$RUN_ANDROID_DUAL_QUICK" == "1" ]]; then
  run_step "Android dual-emulator quick validation" android_dual_quick run_android_dual_quick
fi

if [[ "$RUN_IOS_DUAL_QUICK" == "1" ]]; then
  run_step "iOS dual-client fast validation" ios_dual_quick run_ios_dual_quick
fi

if [[ "$RUN_MAC_ANDROID_QUICK" == "1" ]]; then
  run_step "Mac + Android quick validation" mac_android_quick run_mac_android_quick
fi

if [[ "$RUN_MAC_IOS" == "1" ]]; then
  run_step "Mac + iOS integration validation" mac_ios run_mac_ios
fi

log "post publish client validation complete"
record_summary
record_summary "post publish client validation complete"
record_summary "result_dir=$RESULT_DIR"
record_summary "latest_link=$LATEST_LINK"
