#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

log() {
  printf '==> %s\n' "$*"
}

log "Client boundary"
bash "$ROOT_DIR/scripts/check_client_boundary.sh"

log "Client API surface audit"
bash "$ROOT_DIR/scripts/audit_client_api_surface.sh"

log "Protocol/client contract guard"
bash "$ROOT_DIR/scripts/check_protocol_contracts.sh"

log "macOS local service host contract"
bash "$ROOT_DIR/scripts/tests/guard/check_macos_service_host.sh"

log "client stack guard passed"
