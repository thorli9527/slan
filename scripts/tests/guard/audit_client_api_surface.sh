#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
CLIENT_DIR="$ROOT_DIR/client_v2"
WEB_CONSOLE_RULE_DOC="$CLIENT_DIR/docs/web-console-url-resolution.md"
CLIENT_DEFAULT_ENDPOINTS_DOC="$CLIENT_DIR/docs/client-default-endpoints.md"
SCRIPT_DEFAULT_ENDPOINTS_LIB="$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

print_section() {
  local title="$1"
  echo
  echo "==> ${title}"
}

report_matches() {
  local title="$1"
  shift
  print_section "$title"
  if ! rg -n "$@" 2>/dev/null; then
    echo "(none)"
  fi
}

echo "Client API surface audit root: $CLIENT_DIR"

report_matches \
  "Flutter production direct /api/app references (must stay empty)" \
  '"[^"]*/api/app/|'\''[^'\'']*/api/app/'\''' \
  "$CLIENT_DIR/app_flutter/lib" \
  --glob '!**/build/**'

report_matches \
  "Flutter production direct HTTP client usage (must stay limited)" \
  'HttpClient\(|package:http|http\.(get|post|put|delete|patch)\(' \
  "$CLIENT_DIR/app_flutter/lib" \
  --glob '!**/build/**'

report_matches \
  "Rust control-plane API entrypoints (expected owner)" \
  '/api/app/' \
  "$CLIENT_DIR/rust/crates/client-core-service/src"

report_matches \
  "Install/package defaults referencing biz base URL" \
  '47\.245\.40\.231|SLAN_CONTROL_BASE_URL|DEFAULT_CONTROL_BASE_URL' \
  "$CLIENT_DIR/install" \
  "$CLIENT_DIR/app_flutter/ios/Runner/Info.plist" \
  "$CLIENT_DIR/plugins/client_core_plugin/macos" \
  "$CLIENT_DIR/plugins/client_core_plugin/ios" \
  "$CLIENT_DIR/plugins/client_core_plugin/linux" \
  "$CLIENT_DIR/plugins/client_core_plugin/windows" \
  "$CLIENT_DIR/app_flutter/lib/bridge/client_core_bridge.dart" \
  "$CLIENT_DIR/rust/crates/client-core-service/src/control_plane.rs"

print_section "Shared default endpoint coverage"
test -s "$CLIENT_DEFAULT_ENDPOINTS_DOC"
for required_pattern in \
  '47\.245\.40\.231:28080' \
  '47\.245\.40\.231:24200'
do
  if ! rg -q "$required_pattern" \
    "$CLIENT_DIR/rust/crates/client-core-service/src/control_plane.rs" \
    "$CLIENT_DIR/app_flutter/lib/bridge/client_core_bridge.dart" \
    "$CLIENT_DIR/app_flutter/ios/Runner/Info.plist" \
    "$CLIENT_DIR/install/linux/lib/slan-linux-install.sh" \
    "$CLIENT_DIR/install/windows/SlanWindowsInstall.psm1" \
    "$CLIENT_DIR/plugins/client_core_plugin/macos/Classes/ClientCorePlugin.swift" \
    "$CLIENT_DIR/plugins/client_core_plugin/linux/client_core_plugin.cc" \
    "$CLIENT_DIR/plugins/client_core_plugin/windows/client_core_plugin.cpp" \
    "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"; then
    echo "missing shared default endpoint pattern: $required_pattern" >&2
    exit 1
  fi
done
echo "shared default endpoint patterns present in main client entrypoints"

report_matches \
  "Desktop Web Console host-mapping rules still present (review intentionally)" \
  'web\.dev\.staticlss\.com|127\.0\.0\.1:24200' \
  "$CLIENT_DIR/app_flutter" \
  "$CLIENT_DIR/plugins/client_core_plugin" \
  "$CLIENT_DIR/install" \
  --glob '!**/build/**'

print_section "Shared Web Console mapping rule coverage"
test -s "$WEB_CONSOLE_RULE_DOC"
for required_pattern in \
  'api\.dev\.staticlss\.com' \
  'web\.dev\.staticlss\.com' \
  'api\.slan\.localhost' \
  'web\.slan\.localhost' \
  '47\.245\.40\.231:24200'
do
  if ! rg -q "$required_pattern" \
    "$CLIENT_DIR/app_flutter/lib/bridge/client_core_bridge.dart" \
    "$CLIENT_DIR/plugins/client_core_plugin/macos/Classes/ClientCorePlugin.swift" \
    "$CLIENT_DIR/plugins/client_core_plugin/windows/client_core_plugin.cpp" \
    "$CLIENT_DIR/plugins/client_core_plugin/linux/client_core_plugin.cc"; then
    echo "missing shared Web Console mapping pattern: $required_pattern" >&2
    exit 1
  fi
done
echo "shared mapping patterns present in Flutter/macOS/Windows/Linux"

report_matches \
  "Flutter tests and integration checks with network primitives (allowed by policy)" \
  'HttpClient\(|Socket\.connect\(|RawDatagramSocket|ServerSocket|InternetAddress\.lookup' \
  "$CLIENT_DIR/app_flutter/test" \
  "$CLIENT_DIR/app_flutter/integration_test" \
  --glob '!**/build/**'

print_section "Expected shell/default endpoint literals kept intentionally"
if ! rg -n \
  '47\.245\.40\.231:28080|47\.245\.40\.231:24200|47\.245\.40\.231:24201|47\.245\.40\.231\b' \
  "$ROOT_DIR/scripts/lib/client_default_endpoints.sh" \
  "$ROOT_DIR/scripts/ios_dual_acl_dns_integration.go" \
  "$ROOT_DIR/scripts/setup_remote_docker_context.sh" \
  "$ROOT_DIR/scripts/local_docker_up.sh" \
  "$ROOT_DIR/scripts/local_docker_down.sh" 2>/dev/null; then
  echo "(none)"
fi

print_section "Shell scripts still hardcoding default production endpoints (migration backlog)"
missing_default_literals=0
while IFS= read -r script_path; do
  [[ -n "$script_path" ]] || continue
  case "$(basename "$script_path")" in
    local_docker_up.sh|local_docker_down.sh|setup_remote_docker_context.sh|client_default_endpoints.sh|ios_dual_acl_dns_integration.go)
      continue
      ;;
  esac
  if rg -n \
    '47\.245\.40\.231:28080|47\.245\.40\.231:24200|47\.245\.40\.231:24201|47\.245\.40\.231\b' \
    "$script_path" 2>/dev/null; then
    missing_default_literals=1
  fi
done < <(find "$ROOT_DIR/scripts" -maxdepth 1 \( -name '*.sh' -o -name '*.go' \) -type f | sort)
if [[ "$missing_default_literals" == "0" ]]; then
  echo "(none)"
fi

print_section "Client-facing shell scripts not yet using shared default endpoint library"
missing_shared_defaults=0
while IFS= read -r script_path; do
  [[ -n "$script_path" ]] || continue
  if ! rg -q 'SLAN_BIZ_URL|SLAN_WEB_BASE_URL|SLAN_OPS_BASE_URL|SLAN_REMOTE_OPS_BASE|SLAN_REMOTE_WEB_BASE|SLAN_SERVER_HOST|SLAN_EXPECT_MQTT_HOST' "$script_path"; then
    continue
  fi
  if ! rg -q "client_default_endpoints\\.sh" "$script_path"; then
    echo "$script_path"
    missing_shared_defaults=1
  fi
done < <(find "$ROOT_DIR/scripts" -maxdepth 1 -type f -name '*.sh' | sort)
if [[ "$missing_shared_defaults" == "0" ]]; then
  echo "(none)"
fi

echo
echo "audit complete"
