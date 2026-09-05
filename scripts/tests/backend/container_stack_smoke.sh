#!/usr/bin/env bash
set -euo pipefail

PROJECT="slan-stack-smoke"
NETWORK="${PROJECT}-net"
BIZ_NAME="${PROJECT}-biz"
OPS_NAME="${PROJECT}-ops"
POSTGRES_NAME="${PROJECT}-postgres"
REDIS_NAME="${PROJECT}-redis"
EMQX_NAME="${PROJECT}-emqx"
MAIN_NAME="${PROJECT}-main"
CLIENT_IMAGE="${SLAN_CONTAINER_SMOKE_CLIENT_IMAGE:-alpine:3.20}"
BIZ_PORT="${SLAN_BIZ_CONTAINER_SMOKE_PORT:-39381}"
MAIN_PORT="${SLAN_MAIN_CONTAINER_SMOKE_PORT:-39383}"
WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN:-wire-token}"
POSTGRES_PASSWORD="${SLAN_CONTAINER_SMOKE_DB_PASSWORD:-slan}"

cleanup() {
  docker rm -f "${BIZ_NAME}" "${OPS_NAME}" "${POSTGRES_NAME}" "${REDIS_NAME}" "${EMQX_NAME}" "${MAIN_NAME}" >/dev/null 2>&1 || true
  docker network rm "${NETWORK}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

cleanup
docker network create "${NETWORK}" >/dev/null

smoke_wget() {
  docker run --rm --network "${NETWORK}" "${CLIENT_IMAGE}" wget -qO- "$@"
}

docker run -d --name "${REDIS_NAME}" --network "${NETWORK}" redis:7-alpine >/dev/null
docker run -d --platform linux/amd64 --name "${EMQX_NAME}" --network "${NETWORK}" \
  -e EMQX_AUTHENTICATION__1__MECHANISM=password_based \
  -e EMQX_AUTHENTICATION__1__BACKEND=http \
  -e EMQX_AUTHENTICATION__1__METHOD=post \
  -e EMQX_AUTHENTICATION__1__URL="http://${BIZ_NAME}:8080/mqtt/emqx/auth" \
  -e EMQX_AUTHORIZATION__NO_MATCH=deny \
  -e EMQX_AUTHORIZATION__SOURCES__1__TYPE=http \
  -e EMQX_AUTHORIZATION__SOURCES__1__METHOD=post \
  -e EMQX_AUTHORIZATION__SOURCES__1__URL="http://${BIZ_NAME}:8080/mqtt/emqx/check" \
  emqx/emqx:5.8.9 >/dev/null
docker run -d --name "${POSTGRES_NAME}" --network "${NETWORK}" \
  -e POSTGRES_DB=slan \
  -e POSTGRES_USER=slan \
  -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  postgres:16-alpine >/dev/null

for _ in {1..60}; do
  if docker exec "${POSTGRES_NAME}" pg_isready -U slan -d slan >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

docker exec "${POSTGRES_NAME}" pg_isready -U slan -d slan >/dev/null

for _ in {1..60}; do
  if docker exec "${EMQX_NAME}" emqx ping >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

docker exec "${EMQX_NAME}" emqx ping >/dev/null

docker run -d --name "${BIZ_NAME}" --network "${NETWORK}" --network-alias server-biz \
  -p "127.0.0.1:${BIZ_PORT}:8080" \
  -e SLAN_BIZ_ADDR=:8080 \
  -e SLAN_BIZ_ROUTE_SET=app \
  -e SLAN_SERVICE_BIZ_DB_HOST="${POSTGRES_NAME}" \
  -e SLAN_SERVICE_BIZ_DB_PORT=5432 \
  -e SLAN_SERVICE_BIZ_DB_USER=slan \
  -e SLAN_SERVICE_BIZ_DB_PASSWORD="${POSTGRES_PASSWORD}" \
  -e SLAN_SERVICE_BIZ_DB_NAME=slan \
  -e SLAN_SERVICE_BIZ_DB_SSLMODE=disable \
  -e SLAN_BIZ_REDIS_ADDR="${REDIS_NAME}:6379" \
  -e SLAN_MQTT_ENABLED=true \
  -e SLAN_MQTT_BROKER_URL="mqtt://${EMQX_NAME}:1883" \
  -e SLAN_MQTT_PUBLIC_BROKER_URL="mqtt://127.0.0.1:1883" \
  -e SLAN_MQTT_PASSWORD_SECRET=container-smoke-mqtt-secret \
  -e SLAN_INTERNAL_WIRE_TOKEN="${WIRE_TOKEN}" \
  slan-service-biz:latest >/dev/null

docker run -d --name "${OPS_NAME}" --network "${NETWORK}" --network-alias server-biz-ops \
  -e SLAN_BIZ_ADDR=:8080 \
  -e SLAN_BIZ_ROUTE_SET=ops \
  -e SLAN_SERVICE_BIZ_DB_HOST="${POSTGRES_NAME}" \
  -e SLAN_SERVICE_BIZ_DB_PORT=5432 \
  -e SLAN_SERVICE_BIZ_DB_USER=slan \
  -e SLAN_SERVICE_BIZ_DB_PASSWORD="${POSTGRES_PASSWORD}" \
  -e SLAN_SERVICE_BIZ_DB_NAME=slan \
  -e SLAN_SERVICE_BIZ_DB_SSLMODE=disable \
  -e SLAN_BIZ_REDIS_ADDR="${REDIS_NAME}:6379" \
  -e SLAN_MQTT_ENABLED=true \
  -e SLAN_MQTT_BROKER_URL="mqtt://${EMQX_NAME}:1883" \
  -e SLAN_MQTT_PUBLIC_BROKER_URL="mqtt://127.0.0.1:1883" \
  -e SLAN_MQTT_PASSWORD_SECRET=container-smoke-mqtt-secret \
  -e SLAN_INTERNAL_WIRE_TOKEN="${WIRE_TOKEN}" \
  slan-service-biz:latest >/dev/null

docker run -d --name "${MAIN_NAME}" --network "${NETWORK}" \
  -p "127.0.0.1:${MAIN_PORT}:80" \
  slan-opt-ui:latest >/dev/null

for _ in {1..60}; do
  if smoke_wget --header "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
    "http://${BIZ_NAME}:8080/internal/wire/admin/relay-nodes" >/dev/null 2>&1 &&
    smoke_wget "http://${OPS_NAME}:8080/healthz" >/dev/null 2>&1 &&
    smoke_wget "http://${MAIN_NAME}/" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

smoke_wget --header "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  "http://${BIZ_NAME}:8080/internal/wire/admin/relay-nodes" >/dev/null
smoke_wget "http://${OPS_NAME}:8080/healthz" >/dev/null
smoke_wget "http://${MAIN_NAME}/" | grep -q '<ops-root'
smoke_wget --header 'Content-Type: application/json' \
  --post-data '{"email":"admin1","password":"admin1"}' \
  "http://${MAIN_NAME}/api/ops/auth/login" >/dev/null

echo "container stack smoke passed: app=${BIZ_PORT} main=${MAIN_PORT}"
