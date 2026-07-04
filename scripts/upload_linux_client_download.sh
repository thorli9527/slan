#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

OPS_BASE_URL="${SLAN_REMOTE_OPS_BASE:-${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}}"
WEB_BASE_URL="${SLAN_REMOTE_WEB_BASE:-${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}}"
PACKAGE_PATH="${1:-${SLAN_LINUX_CLIENT_PACKAGE:-}}"
VERSION="${SLAN_LINUX_CLIENT_VERSION:-}"
STATUS="${SLAN_LINUX_CLIENT_STATUS:-active}"
OPS_EMAIL="${SLAN_OPS_EMAIL:-admin1}"
OPS_PASSWORD="${SLAN_OPS_PASSWORD:-admin1}"
RUN_ID="$(date +%s)"

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

need curl
need jq

[[ -n "$PACKAGE_PATH" ]] || fail "usage: upload_linux_client_download.sh /path/to/SLAN-Client-V2-linux-<arch>.tar.gz"
[[ -f "$PACKAGE_PATH" ]] || fail "package not found: $PACKAGE_PATH"

FILE_NAME="$(basename "$PACKAGE_PATH")"
if [[ -z "$VERSION" ]]; then
  VERSION="linux-upload-${RUN_ID}"
fi

log "ops login"
auth_json="$(curl --silent --show-error --fail \
  -X POST "${OPS_BASE_URL}/api/ops/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${OPS_EMAIL}\",\"password\":\"${OPS_PASSWORD}\"}")"
ops_token="$(printf '%s' "$auth_json" | jq -r '.token // .auth.token // .auth.session.token // empty')"
[[ -n "$ops_token" ]] || fail "ops login returned empty token"

log "upload Linux client package"
upload_json="$(curl --silent --show-error --fail \
  -X POST "${OPS_BASE_URL}/api/ops/client-downloads" \
  -H "Authorization: Bearer ${ops_token}" \
  -F platform=linux \
  -F version="${VERSION}" \
  -F status="${STATUS}" \
  -F "file=@${PACKAGE_PATH}")"

download_id="$(printf '%s' "$upload_json" | jq -r '.downloadId // empty')"
download_url="$(printf '%s' "$upload_json" | jq -r '.url // empty')"
[[ -n "$download_id" && -n "$download_url" ]] || fail "upload response missing downloadId or url: $upload_json"

log "verify ops/web listings"
curl --silent --show-error --fail \
  -H "Authorization: Bearer ${ops_token}" \
  "${OPS_BASE_URL}/api/ops/client-downloads" >/dev/null
curl --silent --show-error --fail \
  "${WEB_BASE_URL}/api/web/client-downloads" >/dev/null

echo "linuxClientUpload: ok downloadId=$download_id url=$download_url package=$PACKAGE_PATH version=$VERSION"
