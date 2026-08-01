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

check_required_contracts() {
	test -s "$ROOT_DIR/protocol/openapi/phase1.yaml"
	test -s "$ROOT_DIR/protocol/contracts/auth-registration.yaml"
	test -s "$ROOT_DIR/protocol/contracts/control-plane.yaml"
	test -s "$ROOT_DIR/protocol/contracts/network.yaml"
	test -s "$ROOT_DIR/protocol/contracts/system.yaml"
	rg -q 'option go_package = "github.com/slan/protocol/protobuf/control;control";' "$ROOT_DIR/protocol/protobuf/control.proto"
}

check_no_retired_public_api_prefixes() {
  if rg -n '/api/(v2|web)(/|["`])' \
    "$ROOT_DIR/client_v2" \
    "$ROOT_DIR/protocol" \
    "$ROOT_DIR/docs" \
    --glob '!**/*_test.go' \
    --glob '!**/target/**' \
    --glob '!**/build/**' \
    --glob '!**/.dart_tool/**'; then
    echo "retired /api/v2 or /api/web contract remains" >&2
    return 1
  fi
}

check_no_retired_plan_contracts() {
  if rg -n 'assign-plan|/plans(:|/)|/plan-config:|/users/\{userId\}/plan:|/plan:|PlanStatus:|PlanConfig:|UserPlanOverride:|planOverride:|planCode:|freeDeviceLimit:' \
    "$ROOT_DIR/protocol/openapi"; then
    echo "retired plan contract remains in OpenAPI" >&2
    return 1
  fi
}

check_no_retired_ops_customer_contracts() {
  if rg -n 'OpsCustomer|CustomerID|CustomerDirectory|/api/ops/customers' \
    "$ROOT_DIR/server/service-biz/internal" \
    "$ROOT_DIR/server/opt-ui/src" \
    --glob '!**/*_test.go'; then
    echo "retired ops customer contract remains" >&2
    return 1
  fi
}

check_no_retired_download_contracts() {
  if rg -n 'ClientDownload|client-downloads|downloads/clients|ClientDownloadUpload' \
    "$ROOT_DIR/protocol/openapi" \
    "$ROOT_DIR/server/service-biz/internal" \
    --glob '!**/*_test.go' \
    --glob '!**/gorm_migrate.go'; then
    echo "retired client upload/download contract remains" >&2
    return 1
  fi
}

check_node_repository_naming() {
  if rg -n '\bCatalog\b|\bcatalog\b' \
    "$ROOT_DIR/server/service-biz/internal/service/ops_node_usecase.go" \
    "$ROOT_DIR/server/service-biz/internal/service/ops_node_validation.go" \
    "$ROOT_DIR/server/service-biz/internal/service/wire_usecase.go" \
    "$ROOT_DIR/server/service-biz/internal/service/wire_node_require.go"; then
    echo "node repository still uses retired catalog naming" >&2
    return 1
  fi
}

check_no_retired_web_prefix() {
	if [ -d "$ROOT_DIR/server/service-biz/internal/api/web" ]; then
		echo "retired internal/api/web package remains" >&2
		return 1
	fi
  if rg -n '"/api/web(?:/|")' \
    "$ROOT_DIR/server/service-biz" \
    "$ROOT_DIR/server/opt-ui/src" \
    --glob '!**/*_test.go'; then
    echo "retired /api/web route remains" >&2
    return 1
  fi
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
  test -s "$ROOT_DIR/server/opt-ui/src/app/app.component.ts"
  test -s "$ROOT_DIR/server/opt-ui/src/app/api-paths.ts"
}

check_client_boundary_guard() {
  bash "$ROOT_DIR/scripts/check_client_boundary.sh"
}

check_client_api_surface_audit() {
  bash "$ROOT_DIR/scripts/audit_client_api_surface.sh"
}

check_no_legacy_new_names
check_required_contracts
check_no_retired_public_api_prefixes
check_no_retired_plan_contracts
check_no_retired_ops_customer_contracts
check_no_retired_download_contracts
check_node_repository_naming
check_no_retired_web_prefix
check_server_contract_compile
check_client_contract_compile
check_web_contract_compile
check_client_boundary_guard
check_client_api_surface_audit

echo "protocol contract check passed"
