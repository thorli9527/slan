#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

check_no_legacy_new_names() {
  if rg -n 'service-biz-new|service-ui-new|server-main-new|SLAN_BIZ_NEW|slan-biz-new|slan_biz_new|service_biz_new|docker-compose\.biz-new|container_new_stack|new-stack' \
    "$ROOT_DIR" \
    --glob '!**/.git/**' \
    --glob '!**/target/**' \
    --glob '!**/node_modules/**' \
    --glob '!scripts/check_protocol_contracts.sh'; then
    echo "legacy -new project names remain" >&2
    return 1
  fi
}

check_required_contracts() {
	test -s "$ROOT_DIR/protocol/openapi/phase1.yaml"
	test -s "$ROOT_DIR/protocol/contracts/auth-registration.yaml"
	test -s "$ROOT_DIR/protocol/contracts/control-plane.yaml"
	test -s "$ROOT_DIR/protocol/contracts/network.yaml"
	test -s "$ROOT_DIR/protocol/contracts/system.yaml"
	rg -q 'option go_package = "github.com/slan/protocol/protobuf/control;control";' "$ROOT_DIR/protocol/protobuf/control.proto"
}

check_server_contract_compile() {
  if command -v zsh >/dev/null 2>&1; then
    zsh -lc "cd '$ROOT_DIR/server/service-biz' && go test ./..."
  else
    (cd "$ROOT_DIR/server/service-biz" && go test ./...)
  fi
}

check_client_contract_compile() {
  if command -v zsh >/dev/null 2>&1; then
    zsh -lc "cd '$ROOT_DIR/client_v2/rust' && cargo check -p client-core-service"
  else
    (cd "$ROOT_DIR/client_v2/rust" && cargo check -p client-core-service)
  fi
  test -s "$ROOT_DIR/client_v2/app_flutter/lib/bridge/client_core_bridge.dart"
  test -s "$ROOT_DIR/client_v2/app_flutter/lib/bridge/client_commands.dart"
  test -s "$ROOT_DIR/client_v2/app_flutter/lib/bridge/client_view_state.dart"
}

check_web_contract_compile() {
  test -s "$ROOT_DIR/server/web-ui/src/ui/app-api.service.ts"
  test -s "$ROOT_DIR/server/web-ui/src/ui/app-auth-flow.ts"
  test -s "$ROOT_DIR/server/web-ui/src/ui/app.models.ts"
  test -s "$ROOT_DIR/server/opt-ui/src/app/app.component.ts"
}

check_no_legacy_new_names
check_required_contracts
check_server_contract_compile
check_client_contract_compile
check_web_contract_compile

echo "protocol contract check passed"
