#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

SERVICE_HOST="${SLAN_CLIENT_CORE_SERVICE_HOST:-127.0.0.1}"
SERVICE_PORT="${SLAN_CLIENT_CORE_SERVICE_PORT:-46392}"
DNS_STATE_TIMEOUT_SECONDS="${SLAN_LOCAL_DNS_SMOKE_TIMEOUT_SECONDS:-20}"

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
  python3 - "$SERVICE_HOST" "$SERVICE_PORT" "$method" "$args" <<'PY'
import json
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
method = sys.argv[3]
args = json.loads(sys.argv[4])
payload = json.dumps({"method": method, "args": args}) + "\n"

with socket.create_connection((host, port), timeout=5) as sock:
    sock.sendall(payload.encode())
    sock.shutdown(socket.SHUT_WR)
    print(sock.recv(65535).decode())
PY
}

json_field() {
  local json="$1"
  local filter="$2"
  jq -r "$filter" <<<"$json"
}

assert_jq() {
  local json="$1"
  local filter="$2"
  local label="$3"
  shift 3
  if jq -e "$@" "$filter" >/dev/null <<<"$json"; then
    pass "$label"
  else
    echo "$json" >&2
    fail "$label"
  fi
}

wait_for_dns_ready() {
  local deadline=$(( $(date +%s) + DNS_STATE_TIMEOUT_SECONDS ))
  while (( $(date +%s) < deadline )); do
    local status dns_state
    status="$(request localStatus)"
    dns_state="$(request localDnsState)"
    if jq -e '.signedIn == true and .networkEnabled == true' >/dev/null <<<"$status" &&
       jq -e '.serverEnabled == true and .serverListening == true and (.serverBindAddr | type == "string") and (.serverBindAddr | endswith(":53"))' >/dev/null <<<"$dns_state"; then
      printf '%s\n%s\n' "$status" "$dns_state"
      return 0
    fi
    sleep 1
  done
  return 1
}

pick_dns_record() {
  local module_json="$1"
  jq -c '
    .configs
    | map(.dnsRecords // [])
    | (add // [])
    | map(select((.enabled // true) == true))
    | map(select(((.fqdn // "") | length) > 0))
    | map(select(((.recordType // "A") | ascii_upcase) == "A" or ((.recordType // "") | ascii_upcase) == "CNAME"))
    | .[0] // empty
  ' <<<"$module_json"
}

system_dns_points_localhost() {
  case "$(uname -s)" in
    Darwin)
      scutil --dns 2>/dev/null | grep -q '127\.0\.0\.1'
      ;;
    Linux)
      if command -v resolvectl >/dev/null 2>&1; then
        resolvectl dns 2>/dev/null | grep -q '127\.0\.0\.1'
      else
        grep -Eq '(^|\s)nameserver\s+127\.0\.0\.1($|\s)' /etc/resolv.conf 2>/dev/null
      fi
      ;;
    *)
      return 1
      ;;
  esac
}

need jq
need python3

status_and_dns="$(wait_for_dns_ready || true)"
if [[ -z "$status_and_dns" ]]; then
  echo "localStatus=$(request localStatus)" >&2
  echo "localDnsState=$(request localDnsState)" >&2
  fail "local dns server reaches signed-in/listening state"
fi

status_json="$(printf '%s\n' "$status_and_dns" | sed -n '1p')"
dns_state_before="$(printf '%s\n' "$status_and_dns" | sed -n '2p')"
assert_jq "$status_json" '.signedIn == true and .networkEnabled == true' "desktop service signed in and network enabled"
assert_jq "$dns_state_before" '.serverEnabled == true and .serverListening == true' "local dns server listening"

module_json="$(request localNetworkModule)"
record_json="$(pick_dns_record "$module_json")"
[[ -n "$record_json" ]] || fail "managed dns record exists in localNetworkModule"

fqdn="$(jq -r '.fqdn' <<<"$record_json")"
record_type="$(jq -r '(.recordType // "A") | ascii_upcase' <<<"$record_json")"
requester_device_id="$(jq -r '.serverRequesterDeviceId // empty' <<<"$dns_state_before")"
[[ -n "$requester_device_id" ]] || requester_device_id="$(jq -r '.deviceId // empty' <<<"$status_json")"
[[ -n "$requester_device_id" ]] || fail "requester device id available for dns resolve"

resolve_json="$(request localDnsResolve "{\"requesterDeviceId\":\"${requester_device_id}\",\"qname\":\"${fqdn}\",\"qtype\":\"${record_type}\"}")"
resolve_result="$(jq -r '.result // empty' <<<"$resolve_json")"
[[ -n "$resolve_result" ]] || fail "localDnsResolve returned result"

bind_addr="$(jq -r '.serverBindAddr' <<<"$dns_state_before")"
last_query_before="$(jq -r '.serverLastQueryAtMs // 0' <<<"$dns_state_before")"
dns_query_json="$(python3 - "$bind_addr" "$fqdn" "$record_type" <<'PY'
import ipaddress
import json
import socket
import struct
import sys

bind_addr = sys.argv[1]
fqdn = sys.argv[2].strip().rstrip(".")
qtype_name = sys.argv[3].strip().upper()
host, port = bind_addr.rsplit(":", 1)
port = int(port)
qtype = {"A": 1, "CNAME": 5}.get(qtype_name, 1)

def encode_name(name: str) -> bytes:
    parts = [part for part in name.split(".") if part]
    out = bytearray()
    for part in parts:
        data = part.encode()
        out.append(len(data))
        out.extend(data)
    out.append(0)
    return bytes(out)

packet = bytearray()
packet.extend(struct.pack("!H", 0x4242))
packet.extend(struct.pack("!H", 0x0100))
packet.extend(struct.pack("!HHHH", 1, 0, 0, 0))
packet.extend(encode_name(fqdn))
packet.extend(struct.pack("!HH", qtype, 1))

sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.settimeout(5)
sock.sendto(packet, (host, port))
response, _ = sock.recvfrom(4096)
sock.close()

ancount = struct.unpack("!H", response[6:8])[0]
rcode = response[3] & 0x0F

def parse_name(payload: bytes, offset: int):
    labels = []
    jumped = False
    start = offset
    while True:
        length = payload[offset]
        if length & 0xC0 == 0xC0:
            pointer = ((length & 0x3F) << 8) | payload[offset + 1]
            if not jumped:
                start = offset + 2
                jumped = True
            offset = pointer
            continue
        if length == 0:
            offset += 1
            if not jumped:
                start = offset
            return ".".join(labels), start
        offset += 1
        labels.append(payload[offset:offset + length].decode())
        offset += length

offset = 12
_, offset = parse_name(response, offset)
offset += 4

answers = []
for _ in range(ancount):
    _, offset = parse_name(response, offset)
    answer_type, answer_class, ttl, rdlength = struct.unpack("!HHIH", response[offset:offset + 10])
    offset += 10
    rdata_start = offset
    rdata_end = offset + rdlength
    if answer_type == 1 and rdlength == 4:
        answers.append({"type": "A", "value": str(ipaddress.IPv4Address(response[rdata_start:rdata_end]))})
    elif answer_type == 5:
        cname, _ = parse_name(response, rdata_start)
        answers.append({"type": "CNAME", "value": cname})
    offset = rdata_end

print(json.dumps({"rcode": rcode, "answerCount": ancount, "answers": answers}))
PY
)"

dns_state_after="$(request localDnsState)"
last_query_after="$(jq -r '.serverLastQueryAtMs // 0' <<<"$dns_state_after")"
if [[ "$last_query_after" -gt "$last_query_before" ]]; then
  pass "udp dns query reached local dns server"
else
  echo "$dns_state_before" >&2
  echo "$dns_state_after" >&2
  fail "udp dns query reached local dns server"
fi

case "$resolve_result" in
  answer_a)
    expected_ip="$(jq -r '.ip' <<<"$resolve_json")"
    assert_jq "$dns_query_json" '.rcode == 0 and (.answers | any(.type == "A" and .value == $ip))' "udp dns answer matches authoritative A record" --arg ip "$expected_ip"
    ;;
  answer_cname)
    expected_cname="$(jq -r '.cname' <<<"$resolve_json")"
    assert_jq "$dns_query_json" '.rcode == 0 and (.answers | any(.type == "CNAME" and .value == $cname))' "udp dns answer matches authoritative CNAME record" --arg cname "$expected_cname"
    ;;
  nxdomain)
    assert_jq "$dns_query_json" '.rcode == 3' "udp dns returns nxdomain"
    ;;
  *)
    echo "$resolve_json" >&2
    fail "localDnsResolve returned authoritative answer"
    ;;
esac

if system_dns_points_localhost; then
  pass "system dns points to localhost"
else
  fail "system dns points to localhost"
fi

printf 'desktopLocalDnsSmoke: ok service=%s:%s qname=%s qtype=%s bind=%s\n' \
  "$SERVICE_HOST" "$SERVICE_PORT" "$fqdn" "$record_type" "$bind_addr"
