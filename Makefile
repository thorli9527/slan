.PHONY: help cleanup-devices-integration devices-integration client-desktop-ui-test client-macos-build client-macos-package client-windows-package client-linux-build client-linux-package client-macos-service-smoke client-macos-service-upgrade-smoke client-multidevice-dev flutter-analyze-safe protocol-contract-check local-stack-smoke wire-stack-smoke wire-biz-e2e-smoke wire-stale-nodes-smoke wire-persistence-smoke wire-ticket-key-mismatch-smoke wire-biz-ticket-key-drift-smoke wire-control-plane-check server-docker-build server-docker-package server-docker-publish

help:
	@echo "Available targets:"
	@echo ""
	@echo "  Flutter UI"
	@echo "    make client-desktop-ui-test       # run widget tests covering the desktop client shell"
	@echo "    make client-macos-build           # build the macOS menu bar client shell"
	@echo "    make client-macos-package         # build the macOS .pkg installer"
	@echo "    make client-windows-package       # package Windows installer stage/zip and Inno Setup exe when available"
	@echo "    make client-linux-build           # build Linux Rust service and Flutter GUI"
	@echo "    make client-linux-package         # package Linux tarball and .deb when dpkg-deb is available"
	@echo "    make client-macos-service-smoke   # verify macOS app bundle and launchd service integration"
	@echo "    make client-macos-service-upgrade-smoke # install/upgrade launchd service then verify it"
	@echo "    make client-multidevice-dev       # run macOS/iOS/Android against one local client-core-service"
	@echo "    make flutter-analyze-safe         # run Flutter analyze with stale Dart language-server cleanup"
	@echo "    make protocol-contract-check      # run lightweight protocol metadata and compile checks"
	@echo ""
	@echo "  Server Docker"
	@echo "    make server-docker-build          # build each unique server image once"
	@echo "    make server-docker-package        # export Linux AMD64 server images as a verified archive"
	@echo "    make server-docker-publish        # package and publish to the configured remote server"
	@echo ""
	@echo "  Wire Stack"
	@echo "    make wire-stack-smoke             # run the isolated server-wire + relay + DERP smoke"
	@echo "    make wire-biz-e2e-smoke           # run Docker service-biz -> server-wire authorization smoke"
	@echo "    make wire-stale-nodes-smoke       # verify stale relay/DERP nodes leave scheduling"
	@echo "    make wire-persistence-smoke       # verify server-wire Postgres state survives service restart"
	@echo "    make wire-ticket-key-mismatch-smoke # verify ticket key ring drift is detected"
	@echo "    make wire-biz-ticket-key-drift-smoke # verify business service reports ticket key drift"
	@echo "    make wire-control-plane-check     # run wire authz unit tests + Docker biz/wire e2e smoke"
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

client-macos-build:
	cd client/rust && cargo build -p client-core-service --release --target aarch64-apple-darwin
	cd client/rust && cargo build -p client-core-service --release --target x86_64-apple-darwin
	cd client/app_flutter && flutter build macos
	lipo -create \
		client/rust/target/aarch64-apple-darwin/release/client-core-service \
		client/rust/target/x86_64-apple-darwin/release/client-core-service \
		-output client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service
	chmod 755 client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service
	codesign --force --deep --sign - client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app
	codesign --verify --deep --strict --verbose=2 client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app

client-macos-package: client-macos-build
	./scripts/package_macos.sh

client-windows-package:
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File .\scripts\package_windows.ps1
else
	@echo "client-windows-package must run on Windows with the Flutter Windows release bundle present."
	@echo "Run: powershell -ExecutionPolicy Bypass -File .\\scripts\\package_windows.ps1"
endif

client-linux-build:
	cd client/rust && cargo build -p client-core-service --release
	cd client/app_flutter && flutter build linux

client-linux-package:
	./scripts/package_linux.sh

client-macos-service-smoke:
	./scripts/macos_service_smoke.sh

client-macos-service-upgrade-smoke:
	./scripts/upgrade_macos_service_smoke.sh

client-multidevice-dev:
	./scripts/client_multidevice_dev.sh

flutter-analyze-safe:
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File .\scripts\flutter_analyze_safe.ps1
else
	cd client/app_flutter && flutter analyze
endif

protocol-contract-check:
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File .\scripts\check_protocol_contracts.ps1
else
	./scripts/check_protocol_contracts.sh
endif

server-docker-build:
	DOCKER_DEFAULT_PLATFORM=linux/amd64 docker compose -f docker-compose.local.yml build server-biz server-wire server-wire-relay server-wire-punch server-wire-derp opt-ui

server-docker-package:
	./scripts/package_server_docker.sh

server-docker-publish:
	./scripts/publish_server_docker.sh

local-stack-smoke:
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File .\scripts\local_stack_smoke.ps1
else
	./scripts/container_stack_smoke.sh
endif

wire-stack-smoke:
	GO111MODULE=off go run ./scripts/wire_stack_smoke.go

wire-biz-e2e-smoke:
	go run ./scripts/wire_biz_e2e_smoke.go

wire-stale-nodes-smoke:
	go run ./scripts/wire_stale_nodes_smoke.go

wire-persistence-smoke:
	go run ./scripts/wire_persistence_smoke.go seed
	@if [ "$${SLAN_ALLOW_LOCAL_DOCKER:-0}" != "1" ]; then echo "wire-persistence-smoke needs an explicit local Docker stack: SLAN_ALLOW_LOCAL_DOCKER=1 make wire-persistence-smoke"; exit 2; fi
	docker --context "$${SLAN_LOCAL_DOCKER_CONTEXT:-desktop-linux}" compose -f docker-compose.local.yml restart server-wire
	go run ./scripts/wire_persistence_smoke.go verify

wire-ticket-key-mismatch-smoke:
	go run ./scripts/wire_ticket_key_mismatch_smoke.go

wire-biz-ticket-key-drift-smoke:
	go run ./scripts/wire_biz_ticket_key_drift_smoke.go

wire-control-plane-check:
	cd server/server-wire && GOCACHE=/private/tmp/slan-go-build-cache SLAN_WIRE_POSTGRES_TEST_DSN="postgres://postgres:$${POSTGRES_PASSWORD:-change-me-postgres-password}@127.0.0.1:15432/slan?sslmode=disable" go test ./...
	cd server/server-wire-relay && GOCACHE=/private/tmp/slan-go-build-cache go test ./...
	cd server/server-wire-derp && GOCACHE=/private/tmp/slan-go-build-cache go test ./...
	cd server/service-biz && GOCACHE=/private/tmp/slan-go-build-cache go test ./...
	$(MAKE) wire-biz-e2e-smoke
	$(MAKE) wire-ticket-key-mismatch-smoke
	$(MAKE) wire-biz-ticket-key-drift-smoke
	$(MAKE) wire-stale-nodes-smoke
	$(MAKE) wire-persistence-smoke
