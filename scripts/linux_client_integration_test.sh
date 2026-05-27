#!/usr/bin/env bash
set -euo pipefail

service_host="${SLAN_CLIENT_CORE_SERVICE_HOST:-127.0.0.1}"
service_port="${SLAN_CLIENT_CORE_SERVICE_PORT:-46392}"
api_url="${SLAN_TEST_API_URL:-http://api.dev.staticlss.com}"
web_url="${SLAN_TEST_WEB_URL:-http://web.dev.staticlss.com}"
ops_url="${SLAN_TEST_OPS_URL:-http://ops.dev.staticlss.com}"
mqtt_host="${SLAN_TEST_MQTT_HOST:-47.245.40.231}"
mqtt_port="${SLAN_TEST_MQTT_PORT:-1883}"
wire_host="${SLAN_TEST_WIRE_HOST:-wire.dev.staticlss.com}"
wire_port="${SLAN_TEST_WIRE_PORT:-29100}"
derp_host="${SLAN_TEST_DERP_HOST:-derp.dev.staticlss.com}"
derp_port="${SLAN_TEST_DERP_PORT:-29120}"
relay_host="${SLAN_TEST_RELAY_HOST:-relay.dev.staticlss.com}"
relay_port="${SLAN_TEST_RELAY_PORT:-29110}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

pass() {
  printf 'ok - %s\n' "$1"
}

fail() {
  printf 'not ok - %s\n' "$1" >&2
  exit 1
}

request() {
  local method="$1"
  local args="${2:-{}}"
  printf '{"method":"%s","args":%s}\n' "$method" "$args" | nc -w 3 "$service_host" "$service_port"
}

assert_jq() {
  local json="$1"
  local filter="$2"
  local label="$3"
  if jq -e "$filter" >/dev/null <<<"$json"; then
    pass "$label"
  else
    echo "$json" >&2
    fail "$label"
  fi
}

assert_tcp() {
  local host="$1"
  local port="$2"
  local label="$3"
  if timeout 5 bash -c "</dev/tcp/$host/$port" >/dev/null 2>&1; then
    pass "$label"
  else
    fail "$label"
  fi
}

need bash
need curl
need jq
need nc
need timeout

if command -v systemctl >/dev/null 2>&1; then
  systemctl is-active --quiet slan-client-v2.service && pass "systemd service active" || fail "systemd service active"
fi

test -x /opt/slan-client-v2/bin/client-core-service && pass "service binary installed" || fail "service binary installed"
test -x /opt/slan-client-v2/gui/slan_client_v2 && pass "GUI binary installed" || fail "GUI binary installed"
test -x /usr/bin/slan-client-v2-console && pass "console command installed" || fail "console command installed"
grep -q 'SLAN_CONTROL_BASE_URL=http://api.dev.staticlss.com' /etc/slan/client-v2.env.example \
  && pass "default API points to api.dev.staticlss.com" \
  || fail "default API points to api.dev.staticlss.com"

assert_tcp "$service_host" "$service_port" "local service TCP ${service_host}:${service_port}"

state="$(request localState)"
assert_jq "$state" '.signedIn == false and .networkEnabled == false and .error == null' "localState unsigned clean state"

status="$(request localStatus)"
assert_jq "$status" '.service == "client-core-service" and .signedIn == false and .networkEnabled == false' "localStatus unsigned service state"

session="$(request localSession)"
assert_jq "$session" '.signedIn == false and .expired == false and .deviceId == null' "localSession unsigned clean state"

network_module="$(request localNetworkModule)"
assert_jq "$network_module" '.networkCount == 0 and .peerCount == 0 and (.configs | length) == 0' "localNetworkModule unsigned clean state"

deactivate="$(request localNetworkDeactivate)"
assert_jq "$deactivate" '.networkEnabled == false and .error == null' "localNetworkDeactivate is idempotent"

activate="$(request localNetworkActivate)"
assert_jq "$activate" '.networkEnabled == false and (.error | type == "string")' "localNetworkActivate requires login"

platform_config="$(request localPlatformNetworkConfig)"
assert_jq "$platform_config" '(.error | type == "string")' "localPlatformNetworkConfig requires login"

relay_prepare="$(request localRelayPrepare)"
assert_jq "$relay_prepare" '(.error | type == "string")' "localRelayPrepare requires login"

diag="$(request localDiagnosticsExport)"
diag_path="$(jq -r '.path // empty' <<<"$diag")"
if [[ "$diag_path" == /var/lib/SLAN/diagnostics/client-v2-diagnostics-*.json && -f "$diag_path" ]]; then
  pass "localDiagnosticsExport writes under /var/lib/SLAN/diagnostics"
else
  echo "$diag" >&2
  fail "localDiagnosticsExport writes under /var/lib/SLAN/diagnostics"
fi

if ip link show dev slan0 >/dev/null 2>&1; then
  pass "slan0 interface exists"
else
  fail "slan0 interface exists"
fi

curl -fsS --max-time 10 "$api_url/healthz" | jq -e '.status == "ok"' >/dev/null \
  && pass "remote API healthz" \
  || fail "remote API healthz"
curl -fsSI --max-time 10 "$web_url" >/dev/null && pass "remote web console HTTP" || fail "remote web console HTTP"
curl -fsSI --max-time 10 "$ops_url" >/dev/null && pass "remote ops console HTTP" || fail "remote ops console HTTP"
assert_tcp "$mqtt_host" "$mqtt_port" "remote MQTT TCP"
assert_tcp "$wire_host" "$wire_port" "remote wire TCP"
assert_tcp "$derp_host" "$derp_port" "remote DERP TCP"

if timeout 5 bash -c "printf slan-linux-integration-test >/dev/udp/$relay_host/$relay_port" >/dev/null 2>&1; then
  pass "remote relay UDP send path"
else
  fail "remote relay UDP send path"
fi
