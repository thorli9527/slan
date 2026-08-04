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
REQUIRE_SYSTEM_DNS_LOCALHOST="${SLAN_LOCAL_DNS_REQUIRE_SYSTEM_RESOLVER:-0}"

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
  local args="${2:-}"
  if [[ -z "$args" ]]; then
    args='{}'
  fi
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
    dns_state="$(request localResolverState)"
    if jq -e '.activated == true and .networkEnabled == true' >/dev/null <<<"$status" &&
       jq -e '.serverEnabled == true and .serverListening == true and (.serverBindAddr | type == "string") and (.serverBindAddr | test(":[0-9]+$"))' >/dev/null <<<"$dns_state"; then
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
    | map(.resolverRecords // [])
    | (add // [])
    | map(select((.enabled // true) == true))
    | map(select(((.fqdn // "") | length) > 0))
    | map(select(
        ((.recordType // "A") | ascii_upcase) == "A" or
        ((.recordType // "") | ascii_upcase) == "AAAA" or
        ((.recordType // "") | ascii_upcase) == "CNAME" or
        ((.recordType // "") | ascii_upcase) == "TXT" or
        ((.recordType // "") | ascii_upcase) == "PTR" or
        ((.recordType // "") | ascii_upcase) == "SRV"
      ))
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
  echo "localResolverState=$(request localResolverState)" >&2
  fail "local dns server reaches activated/listening state"
fi

status_json="$(printf '%s\n' "$status_and_dns" | sed -n '1p')"
dns_state_before="$(printf '%s\n' "$status_and_dns" | sed -n '2p')"
assert_jq "$status_json" '.activated == true and .networkEnabled == true' "desktop service signed in and network enabled"
assert_jq "$dns_state_before" '.serverEnabled == true and .serverListening == true' "local dns server listening"

module_json="$(request localNetworkModule)"
record_json="$(pick_dns_record "$module_json")"
[[ -n "$record_json" ]] || fail "managed dns record exists in localNetworkModule"

fqdn="$(jq -r '.fqdn' <<<"$record_json")"
record_type="$(jq -r '(.recordType // "A") | ascii_upcase' <<<"$record_json")"
requester_device_id="$(jq -r '.serverRequesterDeviceId // empty' <<<"$dns_state_before")"
[[ -n "$requester_device_id" ]] || requester_device_id="$(jq -r '.deviceId // empty' <<<"$status_json")"
[[ -n "$requester_device_id" ]] || fail "requester device id available for dns resolve"

resolve_args="$(jq -nc \
  --arg requesterDeviceId "$requester_device_id" \
  --arg qname "$fqdn" \
  --arg qtype "$record_type" \
  '{requesterDeviceId: $requesterDeviceId, qname: $qname, qtype: $qtype}')"
resolve_json="$(request localResolverResolve "$resolve_args")"
resolve_result="$(jq -r '.result // empty' <<<"$resolve_json")"
[[ -n "$resolve_result" ]] || fail "localResolverResolve returned result"

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
qtype = {"A": 1, "CNAME": 5, "PTR": 12, "TXT": 16, "AAAA": 28, "SRV": 33}.get(qtype_name, 1)

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
    elif answer_type == 28 and rdlength == 16:
        answers.append({"type": "AAAA", "value": str(ipaddress.IPv6Address(response[rdata_start:rdata_end]))})
    elif answer_type == 5:
        cname, _ = parse_name(response, rdata_start)
        answers.append({"type": "CNAME", "value": cname})
    elif answer_type == 12:
        ptr, _ = parse_name(response, rdata_start)
        answers.append({"type": "PTR", "value": ptr})
    elif answer_type == 16 and rdlength >= 1:
        texts = []
        cursor = rdata_start
        while cursor < rdata_end:
            txt_len = response[cursor]
            cursor += 1
            txt_end = min(cursor + txt_len, rdata_end)
            texts.append(response[cursor:txt_end].decode())
            cursor = txt_end
        answers.append({"type": "TXT", "value": "".join(texts)})
    elif answer_type == 33 and rdlength >= 7:
        priority, weight, port_value = struct.unpack("!HHH", response[rdata_start:rdata_start + 6])
        target, _ = parse_name(response, rdata_start + 6)
        answers.append({
            "type": "SRV",
            "priority": priority,
            "weight": weight,
            "port": port_value,
            "value": target,
        })
    offset = rdata_end

print(json.dumps({"rcode": rcode, "answerCount": ancount, "answers": answers}))
PY
)"

dns_state_after="$(request localResolverState)"
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
    assert_jq "$resolve_json" '.ips | type == "array" and length > 0' "localResolverResolve returns A answer list"
    expected_ips_json="$(jq -c '.ips' <<<"$resolve_json")"
    assert_jq "$dns_query_json" '
      [.answers[]? | select(.type == "A") | .value] as $actual_ips
      | .rcode == 0
      and ($expected_ips | all(.[]; $actual_ips | index(.) != null))
    ' "udp dns answer matches authoritative A record set" --argjson expected_ips "$expected_ips_json"
    ;;
  answer_cname)
    expected_cname="$(jq -r '.cname' <<<"$resolve_json")"
    assert_jq "$dns_query_json" '.rcode == 0 and (.answers | any(.type == "CNAME" and .value == $cname))' "udp dns answer matches authoritative CNAME record" --arg cname "$expected_cname"
    ;;
  answer_aaaa)
    assert_jq "$resolve_json" '.ips | type == "array" and length > 0' "localResolverResolve returns AAAA answer list"
    expected_ips_json="$(jq -c '.ips' <<<"$resolve_json")"
    assert_jq "$dns_query_json" '
      [.answers[]? | select(.type == "AAAA") | .value] as $actual_ips
      | .rcode == 0
      and ($expected_ips | all(.[]; $actual_ips | index(.) != null))
    ' "udp dns answer matches authoritative AAAA record set" --argjson expected_ips "$expected_ips_json"
    ;;
  answer_txt)
    assert_jq "$resolve_json" '.texts | type == "array" and length > 0' "localResolverResolve returns TXT answer list"
    expected_texts_json="$(jq -c '.texts' <<<"$resolve_json")"
    assert_jq "$dns_query_json" '
      [.answers[]? | select(.type == "TXT") | .value] as $actual_texts
      | .rcode == 0
      and ($expected_texts | all(.[]; $actual_texts | index(.) != null))
    ' "udp dns answer matches authoritative TXT record set" --argjson expected_texts "$expected_texts_json"
    ;;
  answer_ptr)
    expected_name="$(jq -r '.name' <<<"$resolve_json")"
    assert_jq "$dns_query_json" '.rcode == 0 and (.answers | any(.type == "PTR" and .value == $name))' "udp dns answer matches authoritative PTR record" --arg name "$expected_name"
    ;;
  answer_srv)
    expected_target="$(jq -r '.target' <<<"$resolve_json")"
    expected_port="$(jq -r '.port' <<<"$resolve_json")"
    assert_jq "$dns_query_json" '
      .rcode == 0 and
      (.answers | any(.type == "SRV" and .value == $target and ((.port | tostring) == $port)))
    ' "udp dns answer matches authoritative SRV record" --arg target "$expected_target" --arg port "$expected_port"
    ;;
  nxdomain)
    assert_jq "$dns_query_json" '.rcode == 3' "udp dns returns nxdomain"
    ;;
  *)
    echo "$resolve_json" >&2
    fail "localResolverResolve returned authoritative answer"
    ;;
esac

if system_dns_points_localhost; then
  pass "system dns points to localhost"
elif [[ "$REQUIRE_SYSTEM_DNS_LOCALHOST" == "1" ]]; then
  fail "system dns points to localhost"
else
  pass "system dns localhost routing not required for this smoke"
fi

printf 'desktopLocalDnsSmoke: ok service=%s:%s qname=%s qtype=%s bind=%s\n' \
  "$SERVICE_HOST" "$SERVICE_PORT" "$fqdn" "$record_type" "$bind_addr"
