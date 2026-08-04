#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

check_no_legacy_new_names() {
  if rg -n 'service-biz-new|service-ui-new|server-main-new|SLAN_BIZ_NEW|slan-biz-new|slan_biz_new|service_biz_new|docker-compose\.biz-new|container_new_stack|new-stack' \
    "$ROOT_DIR" \
    --glob '!**/.git/**' \
    --glob '!**/target/**' \
    --glob '!**/node_modules/**' \
    --glob '!scripts/check_protocol_contracts.sh' \
    --glob '!scripts/tests/guard/check_protocol_contracts.sh'; then
    echo "legacy -new project names remain" >&2
    return 1
  fi
}

check_no_client_user_contracts() {
  local contract_roots=(
    "$ROOT_DIR/protocol/openapi/service-biz-external.yaml"
    "$ROOT_DIR/server/service-biz/internal/api/app"
    "$ROOT_DIR/server/service-biz/internal/model"
    "$ROOT_DIR/server/service-biz/internal/service"
  )
  if rg -n '/api/auth/(register|login|renew|logout)|/api/users|/api/user-aliases|/api/device-invites|ownerUserId|owner_user_id|listVisibleManagedDevices' \
    "${contract_roots[@]}" \
    --glob '!**/*_test.go'; then
    echo "client user contracts or ownership fields were reintroduced" >&2
    return 1
  fi
}

check_no_sensitive_payload_logs() {
  local server_roots=(
    "$ROOT_DIR/server/service-biz"
    "$ROOT_DIR/server/server-wire"
    "$ROOT_DIR/server/server-wire-punch"
    "$ROOT_DIR/server/server-wire-relay"
    "$ROOT_DIR/server/server-wire-derp"
  )
  if rg -n 'payload=%[sqv]|head=%s|string\([^[:space:]]*\.Payload\(\)\)' \
    "${server_roots[@]}" \
    --glob '*.go' \
    --glob '!**/*_test.go'; then
    echo "raw payload or packet bytes must not be written to production logs" >&2
    return 1
  fi
}

check_required_contracts() {
		test -s "$ROOT_DIR/protocol/openapi/service-biz-external.yaml"
		test -s "$ROOT_DIR/protocol/contracts/device-authorization.yaml"
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
    zsh -lc "cd '$ROOT_DIR/client/rust' && cargo check -p client-core-service"
  else
    (cd "$ROOT_DIR/client/rust" && cargo check -p client-core-service)
  fi
  test -s "$ROOT_DIR/client/app_flutter/lib/bridge/client_core_bridge.dart"
  test -s "$ROOT_DIR/client/app_flutter/lib/bridge/client_commands.dart"
  test -s "$ROOT_DIR/client/app_flutter/lib/bridge/client_view_state.dart"
}

	check_web_contract_compile() {
  test -s "$ROOT_DIR/server/opt-ui/src/app/app.component.ts"
}

check_client_boundary_guard() {
  bash "$ROOT_DIR/scripts/check_client_boundary.sh"
}

check_client_api_surface_audit() {
  bash "$ROOT_DIR/scripts/audit_client_api_surface.sh"
}

check_no_legacy_new_names
check_no_client_user_contracts
check_no_sensitive_payload_logs
check_required_contracts
check_server_contract_compile
check_client_contract_compile
check_web_contract_compile
check_client_boundary_guard
check_client_api_surface_audit

echo "protocol contract check passed"
