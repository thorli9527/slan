#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/../../.." && pwd)
PLUGIN="$ROOT_DIR/client/plugins/client_core_plugin/macos/Classes/ClientCorePlugin.swift"
FLUTTER_CLIENT="$ROOT_DIR/client/app_flutter/lib/bridge/client_core_local_service.dart"
INSTALLER="$ROOT_DIR/client/install/macos/scripts/postinstall"

expected_host="127.0.0.1:46392"

for file in "$PLUGIN" "$FLUTTER_CLIENT" "$INSTALLER"; do
  if ! grep -Fq "$expected_host" "$file"; then
    echo "macOS service host mismatch: $file must use $expected_host" >&2
    exit 1
  fi
done

if grep -Fq "127.0.0.1:46394" "$PLUGIN"; then
  echo "macOS plugin must not start a second client-core-service on port 46394" >&2
  exit 1
fi

echo "macOS service host contract passed: $expected_host"
