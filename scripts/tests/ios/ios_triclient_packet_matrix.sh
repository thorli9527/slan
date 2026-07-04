#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
TIMEOUT="${SLAN_TRICLIENT_PACKET_TIMEOUT:-20s}"

cd "$ROOT_DIR"
go run scripts/ios_triclient_packet_matrix.go -platform ios -timeout "$TIMEOUT"
