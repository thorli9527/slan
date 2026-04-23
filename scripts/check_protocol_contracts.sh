#!/usr/bin/env sh

set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

echo "[protocol-contracts] checking web transport contracts"
go run "$ROOT_DIR/scripts/check_protocol_contracts.go" --root "$ROOT_DIR" --target web

echo "[protocol-contracts] checking flutter transport contracts"
go run "$ROOT_DIR/scripts/check_protocol_contracts.go" --root "$ROOT_DIR" --target flutter

echo "[protocol-contracts] all checks passed"
