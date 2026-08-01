# SLAN Product Scope

## Positioning

SLAN is a cross-platform secure networking platform for developers and teams. It supports private connectivity, device-to-device access, relay fallback, and operational network management.

## Architecture

- Client: Rust networking core plus Flutter UI.
- Control plane: Go `service-biz`, responsible for users, networks, devices, membership, policy, bootstrap, and MQTT control.
- Data plane: Rust relay and tunnel components for P2P, relay fallback, and encrypted traffic.
- Operations: Go/Python tooling for observability and administration.

## Client Scope

### Connection

- Connect and disconnect.
- Automatic reconnect.
- Network status display.
- Latency, bandwidth, and loss indicators.

### Network Management

- Create one owned network per account.
- Join another network by owner email or Join Key.
- Switch active network.
- Show network detail, status, virtual IPs, and remarks.
- Manage address binding and member approval from the network view.

### DNS and Diagnostics

- Private DNS resolution.
- Hosts import/export.
- NAT detection.
- Ping, traceroute, and troubleshooting views.

## Control Plane Scope

- User registration, login, and JWT authentication.
- Device and node registration.
- Network ownership and membership.
- Join approval.
- Default subnet and DHCP-style virtual IP allocation.
- MQTT control channel.
- ACL policy.
- Bootstrap, NetworkMap, and relay ticket APIs.

## Data Plane Scope

- STUN and UDP hole punching.
- P2P connection management.
- UDP/TCP relay fallback.
- Encrypted tunnel using Noise or WireGuard-style primitives.
- TUN/TAP integration.

## Operations Scope

- User analytics.
- Network monitoring.
- Administrative reports.

## Current Product Decisions

- Commercial plans, billing, products, orders, and renewals are retired and out of scope.
- The web console does not expose a dedicated device management page.
- The web console does not expose manual subnet creation.
- Network creation is explicit and dialog based.
- Joining another user's network is dialog based.
- DHCP options are configured during network creation.
- If an account already owns a network, creating another owned network is disabled.
