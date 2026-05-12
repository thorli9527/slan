#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
ENV_FILE="$ROOT_DIR/.env.local"
COMPOSE_FILE="$ROOT_DIR/docker-compose.local.yml"

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps
echo
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T server-biz /bin/sh -lc \
  "wget -qO- http://127.0.0.1:8080/healthz && echo"
echo
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T caddy /bin/sh -lc \
  "grep -q ' slan.localhost' /etc/hosts || echo '127.0.0.1 slan.localhost' >> /etc/hosts; \
   grep -q ' ops.slan.localhost' /etc/hosts || echo '127.0.0.1 ops.slan.localhost' >> /etc/hosts; \
   grep -q ' web.slan.localhost' /etc/hosts || echo '127.0.0.1 web.slan.localhost' >> /etc/hosts; \
   grep -q ' main.slan.localhost' /etc/hosts || echo '127.0.0.1 main.slan.localhost' >> /etc/hosts; \
   wget -qO- --no-check-certificate https://slan.localhost/healthz && echo && \
   wget -qO- --no-check-certificate https://web.slan.localhost/api/devices/visible >/dev/null && echo web-api-ok && \
   wget -qO- --no-check-certificate https://main.slan.localhost/ | grep -q '<ops-root' && echo main-ui-ok && \
   wget -qO- --no-check-certificate https://ops.slan.localhost/ | grep -q '<ops-root' && echo ops-ui-ok && \
   wget -qO- --no-check-certificate https://web.slan.localhost/ | grep -q '<app-root' && echo web-ui-ok && \
   wget -qO- --no-check-certificate https://127.0.0.1/ | grep -q '<ops-root' && echo loopback-ui-ok && \
   wget -qO- http://127.0.0.1/ | grep -q '<ops-root' && echo loopback-http-ui-ok"
