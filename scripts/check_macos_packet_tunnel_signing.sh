#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNNER_ENTITLEMENTS="$ROOT_DIR/client/app/macos/Runner/DebugProfile.entitlements"
RELEASE_ENTITLEMENTS="$ROOT_DIR/client/app/macos/Runner/Release.entitlements"
PACKET_TUNNEL_ENTITLEMENTS="$ROOT_DIR/client/app/macos/PacketTunnel/PacketTunnel.entitlements"

require_packet_tunnel_entitlement() {
  local file="$1"

  if ! /usr/libexec/PlistBuddy -c "Print :com.apple.developer.networking.networkextension" "$file" 2>/dev/null \
    | rg -q "packet-tunnel-provider"; then
    echo "missing packet-tunnel-provider entitlement in: $file" >&2
    return 1
  fi
}

echo "==> checking PacketTunnel entitlement files"
require_packet_tunnel_entitlement "$RUNNER_ENTITLEMENTS"
require_packet_tunnel_entitlement "$RELEASE_ENTITLEMENTS"
require_packet_tunnel_entitlement "$PACKET_TUNNEL_ENTITLEMENTS"

echo "==> checking local code-signing identities"
IDENTITIES="$(security find-identity -v -p codesigning 2>/dev/null || true)"
echo "$IDENTITIES"

if ! echo "$IDENTITIES" | rg -q "Apple Development|Mac Development|Developer ID Application"; then
  cat >&2 <<'EOF'

PacketTunnel signing preflight failed.

Runner + PacketTunnel now carry Network Extension entitlements, so full macOS app
builds and Flutter integration tests need a local development signing identity.

What to do next:
  1. Open Xcode and sign in with an Apple ID that has development certificates.
  2. Ensure an "Apple Development" identity is installed in your login keychain.
  3. Select a Team for both the Runner and PacketTunnel targets.
  4. Re-run:
       make macos-packet-tunnel-signing-check
       make devices-integration

Unsigned native validation still works:
  - make macos-tunnel-control-test
  - make macos-packet-tunnel-build-check
EOF
  exit 1
fi

cat <<'EOF'

PacketTunnel signing preflight passed.

You have at least one local code-signing identity that can satisfy development
signing checks. The remaining target-specific Team / bundle configuration is
expected to be handled inside Xcode.
EOF
