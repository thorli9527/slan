#!/usr/bin/env sh
set -eu

base_url="${SLAN_CONTROL_BASE_URL:-http://host.docker.internal:28080}"
stamp="$(date +%s)"
email="${SLAN_LINUX_SMOKE_EMAIL:-linux-docker-$stamp@local.slan}"
password="${SLAN_LINUX_SMOKE_PASSWORD:-Verify-2026!}"
machine_id="${SLAN_LINUX_SMOKE_MACHINE_ID:-linux-docker-$stamp}"
node_id="${SLAN_LINUX_SMOKE_NODE_ID:-node-linux-docker-$stamp}"

/app/app-core-cli --control-base-url "$base_url" --json auth register \
  --email "$email" \
  --password "$password"

/app/app-core-cli --control-base-url "$base_url" --json device register \
  --name linux-docker \
  --platform linux \
  --machine-id "$machine_id" \
  --public-key linux-docker-pub

/app/app-core-cli --control-base-url "$base_url" --json node register \
  --node-id "$node_id" \
  --node-public-key linux-docker-node-pub \
  --capability client

/app/app-core-cli --control-base-url "$base_url" --json network list
/app/app-core-cli --json status

echo "linux-client-control-smoke ok: $email $machine_id $node_id"
