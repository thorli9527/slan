# Phase 1 MVP

## Goal

Phase 1 should prove the shortest usable loop for SLAN:

1. User registers or logs in.
2. Client registers its device and node identity.
3. User creates one owned network or joins another user's network.
4. Server assigns virtual IP and default subnet access.
5. Client fetches bootstrap and NetworkMap data.
6. Client connects to the MQTT control channel.
7. Client attempts P2P first and falls back to relay when needed.
8. UI shows connection, network, member, and address binding status.

## Scope

### `server-biz`

- User registration and login.
- JWT authentication.
- Device and node registration.
- One owned network per account.
- Join by owner email or Join Key.
- Join approval.
- Default subnet and virtual IP allocation.
- MQTT control channel.
- NetworkMap, bootstrap, and relay ticket APIs.

### `client/app_core`

- Persist login credentials.
- Fetch network and device configuration.
- Establish MQTT control channel.
- Run NAT detection.
- Attempt P2P.
- Fall back to relay.
- Maintain encrypted tunnel state.
- Report runtime network status.

### `client/app`

- Login/register screens.
- Network list and active network view.
- Create network dialog with DHCP options.
- Join network dialog.
- Address binding, network status, and remark display.
- Basic error and state feedback.

### Out of Scope for Phase 1

- Full DNS management.
- Traceroute and advanced diagnostics.
- Fine-grained ACL policies.
- Billing.
- Operations dashboards.
- Manual subnet creation in the console.
- Dedicated device management page.

## Design Notes

- The console no longer creates extra subnets manually. It displays the default subnet and DHCP plan generated during network creation.
- Device-specific actions are folded into network management views through address binding, status, and remarks.
- Protocol contracts live under `protocol/` and are checked against web, Flutter, Rust controller, Go DTOs, OpenAPI, protobuf, and public HTTP routes.

## Acceptance Criteria

1. User can register and log in.
2. User can create one owned network.
3. User can join another network by owner email or Join Key.
4. Owner can approve or reject join requests.
5. Client can fetch network members and assigned virtual IPs.
6. Client can establish the MQTT control channel.
7. Client can prefer P2P and fall back to relay.
8. UI can show network status, address binding, remarks, and basic errors.
