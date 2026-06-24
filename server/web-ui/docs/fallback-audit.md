# web-ui Fallback Audit

## Scope

This audit covers `server/web-ui/src/ui` and separates:

- demo fallback: local-only behavior used when `isDemoMode == true`
- backend gap: behavior that still depends on `service-biz api/web` being completed or made more stable

The current boundary helpers are:

- `isDemoMode`
- `effectiveUserId`

Defined in:

- `server/web-ui/src/ui/app.component.state.ts`

## 1. Demo fallback only

These are intentional local fallbacks for unauthenticated/demo mode and do not imply an API gap by themselves.

### Devices

File:

- `server/web-ui/src/ui/device/app.component.devices.ts`

Cases:

- add device writes local `this.devices`
- toggle / bind device groups keeps local `deviceGroupIdsByDevice`
- create / update / delete device groups uses local group arrays
- update device alias writes local device row
- create device invite generates local `JOIN-*` code
- create bootstrap key generates local `sk_*` key
- revoke bootstrap key keeps local revoked state
- accept invite updates local invite state via `markInviteAccepted`
- delete device removes local row

### Networks

File:

- `server/web-ui/src/ui/network/app.component.networks.ts`

Cases:

- create network appends local workspace
- update network / name / policy mutates local workspace row
- delete network removes local workspace resources
- add / remove / rename network device mutates local device/workspace state

### DNS / public mapping

File:

- `server/web-ui/src/ui/network/app.component.dns.ts`

Cases:

- create / update / delete DNS zone mutates local `dnsZones`
- create / update / delete DNS record mutates local `dnsRecords`
- create / update / delete public mapping mutates local `publicMappings`

### Security

File:

- `server/web-ui/src/ui/network/app.component.security.ts`

Cases:

- create / update / delete security group mutates local `securityGroups`
- create / update / delete security rule mutates local `securityRules`

## 2. Backend-dependent paths already wired

These are not gaps in route shape anymore. They already call `api/web` and only fall back in demo mode.

### Device / invite / bootstrap

- `POST /api/web/device-bootstrap-keys`
- `POST /api/web/device-bootstrap-keys/{keyId}/revoke`
- `POST /api/web/device-invites`
- `POST /api/web/device-invites/accept`
- device group CRUD / binding routes

### Network

- `GET/POST/PATCH/DELETE /api/web/networks...`
- `GET/POST/PATCH/DELETE /api/web/networks/{networkId}/devices...`

### DNS / public mapping

- `GET/POST/PATCH/DELETE /api/web/networks/{networkId}/dns/zones...`
- `GET/POST/PATCH/DELETE /api/web/networks/{networkId}/dns/records...`
- `GET/POST/PATCH/DELETE /api/web/networks/{networkId}/public-mappings...`

### Security

- `GET/POST/PATCH/DELETE /api/web/networks/{networkId}/security-groups...`
- `GET/POST/PATCH/DELETE /api/web/security-groups/.../rules...`

## 3. Remaining likely backend gaps

These are the places worth checking against `service-biz api/web` next.

### A. Load-time fallback in dashboard hydration

File:

- `server/web-ui/src/ui/overview/app.component.data.ts`

Why it matters:

- this file still has many `catch` branches during initial data loading
- some of them silently keep seed/local state instead of proving the backend is complete

Check next:

- users
- visible devices
- networks
- bootstrap keys
- workspace devices
- DNS zones
- DNS records
- public mappings
- security groups
- security rules

### B. Device invite accept semantics

Files:

- `server/web-ui/src/ui/device/app.component.devices.ts`
- backend route expected by `WEB_API.deviceInviteAccept`

Why it matters:

- UI currently only requires `{ invite?: WorkspaceDeviceInviteRow }`
- need to verify backend returns the final accepted invite state consistently
- need to verify network membership is persisted and reflected on next dashboard reload

### C. Device bootstrap revoke/list consistency

Files:

- `server/web-ui/src/ui/device/app.component.devices.ts`
- `server/service-biz/internal/api/web/device_bootstrap_handler.go`

Why it matters:

- create and revoke routes exist
- still need to verify list payload always reflects `status`, `revokedAt`, `usedAt`, `usedByDeviceId`, `networkId`

### D. Network device alias / actor semantics

Files:

- `server/web-ui/src/ui/network/app.component.networks.ts`

Why it matters:

- some calls still deliberately use device owner as actor fallback
- need to verify backend authorization rules match UI assumptions for shared or delegated ownership cases

### E. User alias and password flows

Files:

- `server/web-ui/src/ui/user-alias/app.component.user-alias.ts`
- `server/web-ui/src/ui/app.component.auth.ts`

Why it matters:

- these files still use older `currentUserId || DEFAULT_USER_ID` style
- not part of network core, but they should be aligned to the same boundary model

## 4. Recommended next execution order

1. Audit `overview/app.component.data.ts` load failures against real `api/web` responses.
2. Verify bootstrap-key create/list/revoke payload completeness end to end.
3. Verify invite accept updates membership and reload state correctly.
4. Verify security rule and DNS payloads match backend fields exactly.
5. Normalize user-alias/password flows onto `isDemoMode` / `effectiveUserId`.
