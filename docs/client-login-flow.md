# SLAN Client Device Authentication

This document is the source of truth for client authentication behavior.

## Device Activation

Client identity is device-scoped. The client has no user account, user session,
device claim, or browser-assisted login flow.

1. An operator creates the device in Opt.
2. An operator creates an authorization key for that device. The plaintext key is
   returned once and must be delivered to the device securely.
3. The client calls `POST /api/device-auth/token` with the authorization key and
   its stable local `deviceId` when available.
4. The server validates the key status, expiry, scope, and bound device, then
   returns the device profile, device token, MQTT credentials, and network
   configuration.
5. The client stores the device session securely and uses the device bearer token
   for `/api/app` resources.
6. The client renews its device session before expiry. Revoking the authorization
   key invalidates sessions issued from that key.

An authorization key is never a user credential and must not create a user
session. A device cannot be claimed by, assigned to, or inferred from a user.

## Open Web Console

The desktop client may open the configured Web Console URL. It does not append a
device ID, user session, authorization key, device token, or temporary console
login key. Web Console authentication is an independent browser workflow.

## Local State

The local service owns device identity and credentials. Flutter and native shell
plugins only invoke local commands and display the resulting device/network state.

Logout or deactivation clears local device credentials and runtime state. It does
not operate on a user session. Network, device, device-group, and authorization-key
management remains exclusively in Opt.
