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

## Dev Domains

- API: `http://api.dev.staticlss.com`
- Web Console: `http://web.dev.staticlss.com`
- Ops Console: `http://ops.dev.staticlss.com`
- MQTT: `47.245.40.231`
- Wire: `wire.dev.staticlss.com`
- Relay: `relay.dev.staticlss.com`
- DERP: `derp.dev.staticlss.com`

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

## Special Notes

- Do not start the local Docker Compose stack for normal development.
- `scripts/local_docker_up.sh`, `scripts/local_docker_down.sh`, and `scripts/local_docker_check.sh` refuse to run by default.
- If Docker Desktop was previously used, local cleanup must be explicit:

```bash
SLAN_ALLOW_LOCAL_DOCKER=1 sh scripts/local_docker_down.sh
```

- The local cleanup command targets `desktop-linux` by default and will not use the active remote context.
- SSH credentials and passwords must stay outside git. Use local-only env files under `.tmp/remote-deploy/`.
