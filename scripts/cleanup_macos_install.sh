#!/usr/bin/env bash
set -euo pipefail

SERVICE_LABEL="dev.slan.client-core-service"
APP_LABEL="dev.slan.client-v2"
PACKAGE_IDS=("dev.slan.client-v2")
APP_PROCESS_NAMES=("slan_client_v2" "SLAN Client V2")
SERVICE_PROCESS_NAMES=("client-core-service")

APP_PATHS=(
  "/Applications/SLAN Client V2.app"
)

SYSTEM_FILES=(
  "/Library/LaunchDaemons/${SERVICE_LABEL}.plist"
  "/Library/LaunchAgents/${APP_LABEL}.plist"
)

SYSTEM_DIRS=(
  "/Library/Application Support/SLAN"
  "/Library/Logs/SLAN"
)

USER_RELATIVE_PATHS=(
  "Library/Application Support/SLAN"
  "Library/Application Support/slan_client_v2"
  "Library/Application Support/com.example.slanClientV2"
  "Library/Caches/SLAN"
  "Library/Caches/slan_client_v2"
  "Library/Caches/com.example.slanClientV2"
  "Library/Logs/SLAN"
  "Library/Preferences/com.example.slanClientV2.plist"
  "Library/Preferences/dev.slan.client-v2.plist"
  "Library/Saved Application State/com.example.slanClientV2.savedState"
)

DRY_RUN=0
KEEP_USER_DATA=0
TARGET_ALL_USERS=1
ORIGINAL_ARGS=("$@")

usage() {
  cat <<'EOF'
Usage:
  scripts/cleanup_macos_install.sh [options]

Options:
  --dry-run          Print actions without deleting files.
  --keep-user-data   Keep per-user preferences, caches, logs, and app support data.
  --current-user     Clean only the current console user's per-user data.
  -h, --help         Show this help.

This script removes the installed SLAN Client V2 macOS app, launchd jobs,
service binary, system state, logs, package receipts, and user config/log data.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --keep-user-data)
      KEEP_USER_DATA=1
      shift
      ;;
    --current-user)
      TARGET_ALL_USERS=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "cleanup_macos_install.sh only supports macOS" >&2
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo "$0" ${ORIGINAL_ARGS[@]+"${ORIGINAL_ARGS[@]}"}
fi

run_cmd() {
  if [[ "$DRY_RUN" == "1" ]]; then
    printf 'dry-run:'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

remove_path() {
  local path="$1"
  if [[ -e "$path" || -L "$path" ]]; then
    run_cmd rm -rf "$path"
  elif [[ "$DRY_RUN" == "1" ]]; then
    echo "dry-run: missing $path"
  fi
}

console_uid() {
  stat -f '%u' /dev/console 2>/dev/null || echo ""
}

console_home() {
  local uid
  uid="$(console_uid)"
  if [[ -z "$uid" || "$uid" == "0" ]]; then
    return 0
  fi
  dscl . -search /Users UniqueID "$uid" 2>/dev/null \
    | awk 'NR == 1 { print $1 }' \
    | while read -r user; do
        dscl . -read "/Users/${user}" NFSHomeDirectory 2>/dev/null \
          | awk '{ print $2; exit }'
      done
}

user_homes() {
  if [[ "$TARGET_ALL_USERS" == "0" ]]; then
    console_home
    return 0
  fi
  dscl . -list /Users NFSHomeDirectory 2>/dev/null \
    | awk '$2 ~ /^\/Users\// && $2 !~ /^\/Users\/Shared$/ { print $2 }' \
    | sort -u
}

bootout_launchd_jobs() {
  run_cmd launchctl bootout "system/${SERVICE_LABEL}" >/dev/null 2>&1 || true
  run_cmd launchctl disable "system/${SERVICE_LABEL}" >/dev/null 2>&1 || true

  local uid
  uid="$(console_uid)"
  if [[ -n "$uid" && "$uid" != "0" ]]; then
    run_cmd launchctl bootout "gui/${uid}/${APP_LABEL}" >/dev/null 2>&1 || true
    run_cmd launchctl disable "gui/${uid}/${APP_LABEL}" >/dev/null 2>&1 || true
  fi
}

stop_processes() {
  local name
  for name in "${APP_PROCESS_NAMES[@]}" "${SERVICE_PROCESS_NAMES[@]}"; do
    run_cmd pkill -x "$name" >/dev/null 2>&1 || true
  done
}

forget_receipts() {
  local package_id
  for package_id in "${PACKAGE_IDS[@]}"; do
    if pkgutil --pkg-info "$package_id" >/dev/null 2>&1; then
      run_cmd pkgutil --forget "$package_id" >/dev/null
    elif [[ "$DRY_RUN" == "1" ]]; then
      echo "dry-run: package receipt not found $package_id"
    fi
  done
}

remove_user_data() {
  local home relative path
  while IFS= read -r home; do
    [[ -n "$home" && -d "$home" ]] || continue
    for relative in "${USER_RELATIVE_PATHS[@]}"; do
      path="${home}/${relative}"
      remove_path "$path"
    done
  done < <(user_homes)
}

echo "Cleaning SLAN Client V2 macOS installation..."
bootout_launchd_jobs
stop_processes

for path in "${SYSTEM_FILES[@]}" "${APP_PATHS[@]}"; do
  remove_path "$path"
done

for path in "${SYSTEM_DIRS[@]}"; do
  remove_path "$path"
done

if [[ "$KEEP_USER_DATA" != "1" ]]; then
  remove_user_data
fi

forget_receipts

echo "SLAN Client V2 macOS cleanup finished."
if [[ "$DRY_RUN" == "1" ]]; then
  echo "No files were removed because --dry-run was used."
fi
