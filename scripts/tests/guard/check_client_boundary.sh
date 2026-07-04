#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
FLUTTER_LIB_DIR="$ROOT_DIR/client_v2/app_flutter/lib"

check_flutter_lib_no_direct_app_api() {
  if rg -n '"[^"]*/api/app/|'\''[^'\'']*/api/app/'\''' \
    "$FLUTTER_LIB_DIR" \
    --glob '!**/build/**'; then
    echo "Flutter production code must not reference /api/app directly." >&2
    return 1
  fi
}

check_flutter_lib_no_direct_http_client() {
  if rg -n 'HttpClient\(|package:http|http\.(get|post|put|delete|patch)\(' \
    "$FLUTTER_LIB_DIR" \
    --glob '!**/build/**' \
    --glob '!**/bridge/client_core_local_service.dart' \
    --glob '!**/bridge/client_ui_diagnostics.dart'; then
    echo "Flutter production code must not create direct business HTTP clients." >&2
    return 1
  fi
}

check_flutter_lib_no_business_state_plugin_reads() {
  if rg -n 'androidRuntimeState\(\)|iosRuntimeState\(\)' \
    "$FLUTTER_LIB_DIR" \
    --glob '!**/build/**' \
    --glob '!**/bridge/client_core_bridge.dart' \
    --glob '!**/bridge/client_ui_diagnostics.dart'; then
    echo "Plugin runtime reads must stay isolated to bridge/diagnostics layers." >&2
    return 1
  fi
}

check_boundary_doc_present() {
  test -s "$ROOT_DIR/client_v2/docs/client-architecture-boundary.md"
}

check_flutter_lib_no_direct_app_api
check_flutter_lib_no_direct_http_client
check_flutter_lib_no_business_state_plugin_reads
check_boundary_doc_present

echo "client boundary check passed"
