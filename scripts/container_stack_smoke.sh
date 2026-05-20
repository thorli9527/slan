#!/usr/bin/env bash
set -euo pipefail

PROJECT="slan-stack-smoke"
NETWORK="${PROJECT}-net"
BIZ_NAME="${PROJECT}-biz"
REDIS_NAME="${PROJECT}-redis"
BIFROMQ_NAME="${PROJECT}-bifromq"
WEB_NAME="${PROJECT}-web"
MAIN_NAME="${PROJECT}-main"
CLIENT_IMAGE="${SLAN_CONTAINER_SMOKE_CLIENT_IMAGE:-alpine:3.20}"
BIZ_PORT="${SLAN_BIZ_CONTAINER_SMOKE_PORT:-39081}"
WEB_PORT="${SLAN_WEB_CONTAINER_SMOKE_PORT:-39200}"
MAIN_PORT="${SLAN_MAIN_CONTAINER_SMOKE_PORT:-39201}"
WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN:-wire-token}"

cleanup() {
  docker rm -f "${BIZ_NAME}" "${REDIS_NAME}" "${BIFROMQ_NAME}" "${WEB_NAME}" "${MAIN_NAME}" >/dev/null 2>&1 || true
  docker network rm "${NETWORK}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

cleanup
docker network create "${NETWORK}" >/dev/null

smoke_wget() {
  docker run --rm --network "${NETWORK}" "${CLIENT_IMAGE}" wget -qO- "$@"
}

docker run -d --name "${REDIS_NAME}" --network "${NETWORK}" redis:7-alpine >/dev/null
docker run -d --platform linux/amd64 --name "${BIFROMQ_NAME}" --network "${NETWORK}" apache/bifromq:4.0.0-incubating >/dev/null

docker run -d --name "${BIZ_NAME}" --network "${NETWORK}" --network-alias server-biz \
  -p "127.0.0.1:${BIZ_PORT}:8080" \
  -e SLAN_BIZ_ADDR=:8080 \
  -e SLAN_BIZ_REDIS_ADDR="${REDIS_NAME}:6379" \
  -e SLAN_MQTT_ENABLED=true \
  -e SLAN_MQTT_BROKER_URL="mqtt://${BIFROMQ_NAME}:1883" \
  -e SLAN_MQTT_PUBLIC_BROKER_URL="mqtt://127.0.0.1:1883" \
  -e SLAN_MQTT_PASSWORD_SECRET=container-smoke-mqtt-secret \
  -e SLAN_INTERNAL_WIRE_TOKEN="${WIRE_TOKEN}" \
  slan-server-biz:latest >/dev/null

docker run -d --name "${WEB_NAME}" --network "${NETWORK}" \
  -p "127.0.0.1:${WEB_PORT}:80" \
  slan-server-ui-web:latest >/dev/null

docker run -d --name "${MAIN_NAME}" --network "${NETWORK}" \
  -p "127.0.0.1:${MAIN_PORT}:80" \
  slan-server-main:latest >/dev/null

for _ in {1..60}; do
  if smoke_wget --header "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
    "http://${BIZ_NAME}:8080/internal/wire/admin/relay-nodes" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

smoke_wget --header "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  "http://${BIZ_NAME}:8080/internal/wire/admin/relay-nodes" >/dev/null
smoke_wget --header 'Content-Type: application/json' \
  --post-data '{"email":"admin@slan.local","password":"admin123456"}' \
  "http://${BIZ_NAME}:8080/api/ops/auth/login" >/dev/null
smoke_wget "http://${WEB_NAME}/" | grep -q '<app-root'
smoke_wget "http://${MAIN_NAME}/" | grep -q '<ops-root'
smoke_wget "http://${WEB_NAME}/api/devices/visible" >/dev/null

echo "container new stack smoke passed: biz=${BIZ_PORT} web=${WEB_PORT} main=${MAIN_PORT}"
