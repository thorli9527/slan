.PHONY: help cleanup-devices-integration devices-integration macos-tunnel-control-test macos-packet-tunnel-build-check macos-packet-tunnel-signing-check client-desktop-ui-test protocol-contract-check local-stack-smoke

help:
	@echo "Available targets:"
	@echo ""
	@echo "  Flutter UI"
	@echo "    make client-desktop-ui-test       # run widget tests covering the desktop client shell"
	@echo "    make protocol-contract-check      # run web + Flutter protocol drift checks"
	@echo ""
	@echo "  Devices Integration"
	@echo "    make cleanup-devices-integration  # kill lingering Flutter integration and slan_app processes"
	@echo "    make devices-integration          # run the devices integration suites with external cleanup"
	@echo "    make local-stack-smoke           # run the local Docker control-plane + relay smoke"
	@echo ""
	@echo "  macOS Native"
	@echo "    make macos-tunnel-control-test        # run the SwiftPM TunnelControl native tests"
	@echo "    make macos-packet-tunnel-build-check  # build-check the macOS PacketTunnel target without code signing"
	@echo "    make macos-packet-tunnel-signing-check  # verify local development signing prerequisites for Runner + PacketTunnel"

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

macos-tunnel-control-test:
	./scripts/test_macos_tunnel_control.sh

macos-packet-tunnel-build-check:
	./scripts/test_macos_packet_tunnel_target.sh

macos-packet-tunnel-signing-check:
	./scripts/check_macos_packet_tunnel_signing.sh

client-desktop-ui-test:
	./scripts/test_client_desktop_ui.sh

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
