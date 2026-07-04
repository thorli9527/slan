#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

RUN_START_SIMS="${SLAN_RUN_IOS_START_SIMS:-1}"
RUN_DUAL_QUICK="${SLAN_RUN_IOS_DUAL_QUICK:-1}"
RUN_DUAL_FLUTTER_MESSAGE="${SLAN_RUN_IOS_DUAL_FLUTTER_MESSAGE:-1}"

MODE="${1:-}"

case "$MODE" in
  ""|--full-stable )
    ;;
  --quick-only )
    RUN_DUAL_QUICK=1
    RUN_DUAL_FLUTTER_MESSAGE=0
    ;;
  --message-only )
    RUN_DUAL_QUICK=0
    RUN_DUAL_FLUTTER_MESSAGE=1
    ;;
  -h|--help )
    ;;
  * )
    printf 'unknown option: %s\n' "$MODE" >&2
    printf 'run with --help for usage\n' >&2
    exit 1
    ;;
esac

if [[ "$MODE" == "-h" || "$MODE" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/ios_dual_fast_check.sh
  bash scripts/ios_dual_fast_check.sh --full-stable
  bash scripts/ios_dual_fast_check.sh --quick-only
  bash scripts/ios_dual_fast_check.sh --message-only

Optional environment variables:
  SLAN_RUN_IOS_START_SIMS=1|0
  SLAN_RUN_IOS_DUAL_QUICK=1|0
  SLAN_RUN_IOS_DUAL_FLUTTER_MESSAGE=1|0
  SLAN_IOS_APP_DNS_ACL_TIMEOUT
  SLAN_IOS_APP_DNS_ACL_CLIENTS
  SLAN_IOS_WAIT_BEFORE_SEND_SECONDS
  SLAN_IOS_DEVICE_ID_WAIT_SECONDS
  SLAN_KEEP_IOS_DUAL_FLUTTER_WORK_DIR=1|0

Examples:
  bash scripts/ios_dual_fast_check.sh
  bash scripts/ios_dual_fast_check.sh --full-stable
  bash scripts/ios_dual_fast_check.sh --quick-only
  bash scripts/ios_dual_fast_check.sh --message-only
  SLAN_RUN_IOS_START_SIMS=0 bash scripts/ios_dual_fast_check.sh
  SLAN_RUN_IOS_DUAL_QUICK=0 bash scripts/ios_dual_fast_check.sh
EOF
  exit 0
fi

log() {
  printf '==> %s\n' "$*"
}

run_in_root() {
  (
    cd "$ROOT_DIR"
    "$@"
  )
}

log "ios dual fast toggles: start_sims=${RUN_START_SIMS} dual_quick=${RUN_DUAL_QUICK} dual_flutter_message=${RUN_DUAL_FLUTTER_MESSAGE}"

export SLAN_BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
export SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
export SLAN_EXPECT_MQTT_HOST="${SLAN_EXPECT_MQTT_HOST:-$SLAN_DEFAULT_MQTT_HOST}"
export SLAN_IOS_APP_DNS_ACL_TIMEOUT="${SLAN_IOS_APP_DNS_ACL_TIMEOUT:-75s}"
export SLAN_IOS_APP_DNS_ACL_CLIENTS="${SLAN_IOS_APP_DNS_ACL_CLIENTS:-2}"
export SLAN_IOS_APP_DNS_ACL_CHECK_MESSAGES="${SLAN_IOS_APP_DNS_ACL_CHECK_MESSAGES:-1}"
export SLAN_IOS_WAIT_BEFORE_SEND_SECONDS="${SLAN_IOS_WAIT_BEFORE_SEND_SECONDS:-8}"
export SLAN_IOS_DEVICE_ID_WAIT_SECONDS="${SLAN_IOS_DEVICE_ID_WAIT_SECONDS:-180}"
export SLAN_KEEP_IOS_DUAL_FLUTTER_WORK_DIR="${SLAN_KEEP_IOS_DUAL_FLUTTER_WORK_DIR:-0}"
export SLAN_IOS_SIM_A_NAME="${SLAN_IOS_SIM_A_NAME:-iPhone 17 Pro}"
export SLAN_IOS_SIM_B_NAME="${SLAN_IOS_SIM_B_NAME:-SLAN iPhone 16 Pro Clean 26.5}"

log "ios stable defaults: biz=${SLAN_BIZ_URL} web=${SLAN_WEB_BASE_URL} mqtt_host=${SLAN_EXPECT_MQTT_HOST}"
log "ios stable defaults: dns_acl_timeout=${SLAN_IOS_APP_DNS_ACL_TIMEOUT} wait_before_send=${SLAN_IOS_WAIT_BEFORE_SEND_SECONDS}s device_id_wait=${SLAN_IOS_DEVICE_ID_WAIT_SECONDS}s"
log "recommended iOS simulators: ${SLAN_IOS_SIM_A_NAME}, ${SLAN_IOS_SIM_B_NAME}"

if [[ "$RUN_START_SIMS" == "1" ]]; then
  log "Start iOS dual simulators"
  run_in_root bash scripts/start_ios_dual_sims.sh
fi

if [[ "$RUN_DUAL_QUICK" == "1" ]]; then
  log "Run iOS dual quick validation"
  (
    cd "$ROOT_DIR"
    echo "==> if simulators are not booted, run: bash scripts/start_ios_dual_sims.sh"
    echo "==> iOS dual DNS/ACL quick validation"
    bash scripts/ios_app_dns_acl_smoke.sh
    echo "==> iOS MQTT client_message quick validation"
    go run scripts/client_message_mqtt_smoke.go \
      -biz-url "$SLAN_BIZ_URL" \
      -expect-mqtt-host "$SLAN_EXPECT_MQTT_HOST"
  )
fi

if [[ "$RUN_DUAL_FLUTTER_MESSAGE" == "1" ]]; then
  log "Run dual iOS Flutter message validation"
  run_in_root bash scripts/ios_dual_flutter_message_check.sh
fi

log "iOS dual fast validation complete"
