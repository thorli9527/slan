.PHONY: help cleanup-devices-integration devices-integration client-desktop-ui-test flutter-analyze-safe protocol-contract-check local-stack-smoke

help:
	@echo "Available targets:"
	@echo ""
	@echo "  Flutter UI"
	@echo "    make client-desktop-ui-test       # run widget tests covering the desktop client shell"
	@echo "    make flutter-analyze-safe         # run Flutter analyze with stale Dart language-server cleanup"
	@echo "    make protocol-contract-check      # run web + Flutter + Rust + Go + OpenAPI + protobuf + route drift checks"
	@echo ""
	@echo "  Devices Integration"
	@echo "    make cleanup-devices-integration  # kill lingering Flutter integration and slan_app processes"
	@echo "    make devices-integration          # run the devices integration suites with external cleanup"
	@echo "    make local-stack-smoke           # run the local Docker control-plane + relay smoke"

cleanup-devices-integration:
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File .\scripts\cleanup_devices_integration.ps1
else
	./scripts/cleanup_devices_integration.sh
endif

devices-integration:
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File .\scripts\run_devices_integration.ps1
else
	./scripts/run_devices_integration.sh
endif

client-desktop-ui-test:
	./scripts/test_client_desktop_ui.sh

flutter-analyze-safe:
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File .\scripts\flutter_analyze_safe.ps1
else
	cd client_v2/app_flutter && flutter analyze
endif

protocol-contract-check:
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File .\scripts\check_protocol_contracts.ps1
else
	./scripts/check_protocol_contracts.sh
endif

local-stack-smoke:
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File .\scripts\local_stack_smoke.ps1
else
	./scripts/local_stack_smoke.sh
endif
