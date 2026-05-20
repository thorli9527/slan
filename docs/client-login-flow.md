# SLAN Client Login Flow

This document is the source of truth for client login behavior.

## Browser Login From Client

Client login uses device-scoped MQTT delivery. It does not wait for login completion through repeated server requests.

1. Client service resolves the stable local `deviceId`.
2. Client service calls `POST /api/auth/device-login-devices` with `deviceId`, platform metadata, and public key.
3. Server registers a pre-login device if it does not exist. If the device already exists, the server reuses it. The server then returns MQTT credentials and a login URL.
4. Client opens Web Console with only:

```text
?auth=login&deviceId=<deviceId>
```

5. If the browser already has a valid Web Console session, Web Console completes client login immediately.
6. If the browser is not signed in, Web Console signs in first, then completes client login.
7. Web Console calls `POST /api/auth/device-login-devices/{deviceId}/complete`.
8. Server verifies that the target device exists. If it is still a pre-login device, the server binds it to the browser user; if it already belongs to another user, the server rejects the login. The server then publishes `device_user_login_succeeded` to that device's MQTT topic.
9. Client consumes `device_user_login_succeeded`, persists the user session and device session, emits `session.changed`, and moves to the signed-in page.

The browser login URL must only carry the login intent and target device. It must not include:

- `clientPlatform`
- `clientName`

The complete endpoint must never create a missing device. Device creation for browser login happens only during the client-initiated prepare call.

## Open Web Console From Signed-In Client

Opening Web Console from an already signed-in client is a separate flow.

1. Client asks local service for a new `consoleLoginKey` every time the signed-in user clicks Web Console.
2. Local service requests `POST /api/auth/console-login-keys` using the current user session and the current `deviceId`.
3. Server creates a short-lived, single-use `consoleLoginKey` bound to the current user session and optional device.
4. Client opens Web Console with `consoleLoginKey` and optional `deviceId`.
5. Web Console consumes the key through `POST /api/auth/console-login`.
6. If the server accepts the key, Web Console persists the returned browser session and navigates to the user's default page.
7. If the server rejects the key, Web Console must show an invalid/expired credential prompt and must not enter the authenticated UI.

`consoleLoginKey` is only for opening Web Console from a signed-in client. It is not used for client browser login, and the client must not reuse an old key.

## Angular Web Console Responsibilities

Angular keeps these concerns separated:

- `app-auth-flow.ts`: URL parsing, browser auth storage, and URL cleanup.
- `app.component.auth.ts`: login/register, browser session restore, console login key consumption, and client login completion.
- `app.component.ts`: startup sequencing only.

Startup order:

1. If `auth=login&deviceId=...` is present and browser auth already exists, complete client login immediately. This calls the server complete endpoint, and the server publishes MQTT login success to that device.
2. Consume `consoleLoginKey` if present. This is only for opening Web Console from an already signed-in client. A valid key goes to the default signed-in page; an invalid or expired key shows an illegal credential message and returns to the login screen.
3. Restore browser auth from local storage for normal Web Console navigation.
4. If client login completion fails, stay on the login screen and clear stale browser auth.

## Logout

Logout clears both sides of local identity:

1. Client calls the local logout command.
2. Local service clears persisted user session and device session.
3. Server logout can clear the user session and device session when tokens are supplied.
4. Client emits `session.changed` with signed-out state.

## Renewal

User session and device session are renewed independently:

- User session: `POST /api/auth/renew`.
- Device session: `POST /api/device/session/renew`.
- MQTT credentials are refreshed through the device/session responses.

## Removed Legacy Behavior

Client login completion is no longer modeled as browser-side waiting or repeated HTTP requests. The only supported completion path is the Web Console complete endpoint followed by device-scoped MQTT notification.
