# Remote Docker Only

SLAN development and deployment now use the remote Docker host only.

## Active Runtime

- Remote host: `47.245.40.231`
- Docker context: `slan-remote`
- Docker endpoint: `ssh://root@47.245.40.231`
- Remote project directory: `/opt/slan`
- Deploy script: `.tmp/remote-deploy/deploy_to_47.245.40.231.sh`
- Remote compose file: `/opt/slan/docker-compose.local.yml`
- Remote environment file: `/opt/slan/.env.prod`

## Public IP Endpoints

- API: `http://47.245.40.231:28080`
- Web Console: `http://47.245.40.231:24200`
- Ops Console: `http://47.245.40.231:24201`
- MQTT: `47.245.40.231`
- Wire: `47.245.40.231:29100`
- Relay: `47.245.40.231:29110`
- DERP: `47.245.40.231:29120`

## Required Workflow

Before running Docker commands on the Mac:

```bash
sh scripts/setup_remote_docker_context.sh
docker context show
```

`docker context show` must print:

```text
slan-remote
```

Deploy or redeploy with:

```bash
.tmp/remote-deploy/deploy_to_47.245.40.231.sh
```

## Publish Validation

Default publish validation already includes:

- Remote health checks
- `app/web/ops` lightweight smoke
- `punch` smoke

Run the default publish flow with:

```bash
.tmp/remote-deploy/deploy_to_47.245.40.231.sh
```

Enable app client DNS/ACL/message validation after deploy:

```bash
RUN_REMOTE_APP_DNS_ACL_SMOKE=1 .tmp/remote-deploy/deploy_to_47.245.40.231.sh
```

Enable heavy UI/OPS end-to-end validation after deploy:

```bash
RUN_REMOTE_UI_OPS_SMOKE=1 .tmp/remote-deploy/deploy_to_47.245.40.231.sh
```

Enable both heavy validations together:

```bash
RUN_REMOTE_APP_DNS_ACL_SMOKE=1 RUN_REMOTE_UI_OPS_SMOKE=1 .tmp/remote-deploy/deploy_to_47.245.40.231.sh
```

Enable post-publish Linux/iOS client validation from this Mac / configured VM:

```bash
RUN_POST_PUBLISH_CLIENT_VALIDATION=1 .tmp/remote-deploy/deploy_to_47.245.40.231.sh
```

Relevant switches:

- `RUN_REMOTE_SMOKE=1` keeps the lightweight `app/web/ops` smoke enabled.
- `RUN_REMOTE_PUNCH_SMOKE=1` keeps the punch smoke enabled.
- `RUN_REMOTE_APP_DNS_ACL_SMOKE=1` adds app-side DNS/ACL/message validation.
- `RUN_REMOTE_UI_OPS_SMOKE=1` adds the heavier UI/OPS workflow validation.
- `RUN_POST_PUBLISH_CLIENT_VALIDATION=1` runs Linux VM and iOS follow-up client validation after publish.
- `REMOTE_SMOKE_SEED_WIRE_NODES=1` is only needed when the target environment does not already have relay / DERP node data.

## Linux And iOS Follow-up Validation

After publish succeeds, Linux and iOS client-side validation should use the same isolated endpoints:

- `api/app` traffic goes to `http://47.245.40.231:28080`
- `api/web` traffic goes to `http://47.245.40.231:24200`

Linux client validation:

```bash
SLAN_TEST_API_URL=http://47.245.40.231:28080 \
SLAN_TEST_WEB_URL=http://47.245.40.231:24200 \
SLAN_TEST_OPS_URL=http://47.245.40.231:24201 \
scripts/linux_client_integration_test.sh
```

iOS DNS/ACL validation:

```bash
SLAN_BIZ_URL=http://47.245.40.231:28080 \
SLAN_WEB_BASE_URL=http://47.245.40.231:24200 \
scripts/ios_app_dns_acl_smoke.sh
```

Mac + iOS integration validation:

```bash
SLAN_BIZ_URL=http://47.245.40.231:28080 \
SLAN_WEB_BASE_URL=http://47.245.40.231:24200 \
scripts/mac_ios_integration_check.sh
```

## Special Notes

- Do not start the local Docker Compose stack for normal development.
- `scripts/local_docker_up.sh`, `scripts/local_docker_down.sh`, and `scripts/local_docker_check.sh` refuse to run by default.
- If Docker Desktop was previously used, local cleanup must be explicit:

```bash
SLAN_ALLOW_LOCAL_DOCKER=1 sh scripts/local_docker_down.sh
```

- The local cleanup command targets `desktop-linux` by default and will not use the active remote context.
- SSH credentials and passwords must stay outside git. Use local-only env files under `.tmp/remote-deploy/`.
