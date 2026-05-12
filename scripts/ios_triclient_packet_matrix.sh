#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TIMEOUT="${SLAN_TRICLIENT_PACKET_TIMEOUT:-20s}"

cd "$ROOT_DIR"
go run scripts/ios_triclient_packet_matrix.go -platform ios -timeout "$TIMEOUT"
