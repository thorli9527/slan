#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

"$ROOT_DIR/scripts/cleanup_devices_integration.sh"
"$ROOT_DIR/scripts/test_devices_integration.sh"
