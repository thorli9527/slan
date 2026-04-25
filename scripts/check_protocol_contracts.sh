#!/usr/bin/env sh

set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

echo "[protocol-contracts] checking web transport contracts"
go run "$ROOT_DIR/scripts/check_protocol_contracts.go" --root "$ROOT_DIR" --target web

echo "[protocol-contracts] checking flutter transport contracts"
go run "$ROOT_DIR/scripts/check_protocol_contracts.go" --root "$ROOT_DIR" --target flutter

echo "[protocol-contracts] checking rust controller transport contracts"
go run "$ROOT_DIR/scripts/check_protocol_contracts.go" --root "$ROOT_DIR" --target rust-controller

echo "[protocol-contracts] checking go server transport contracts"
go run "$ROOT_DIR/scripts/check_protocol_contracts.go" --root "$ROOT_DIR" --target go-server

echo "[protocol-contracts] checking OpenAPI transport contracts"
go run "$ROOT_DIR/scripts/check_protocol_contracts.go" --root "$ROOT_DIR" --target openapi

echo "[protocol-contracts] checking protobuf control contracts"
go run "$ROOT_DIR/scripts/check_protocol_contracts.go" --root "$ROOT_DIR" --target protobuf

echo "[protocol-contracts] checking public HTTP routes"
go run "$ROOT_DIR/scripts/check_protocol_contracts.go" --root "$ROOT_DIR" --target http-routes

echo "[protocol-contracts] all checks passed"
