# server-ui/web

Angular network control console.

Current capabilities:

- User login and registration.
- Create an owned network from a dialog.
- Disable network creation when the account already owns a network.
- Join another network from a dialog by owner email or Join Key.
- Configure DHCP options when creating a network.
- Switch back to the owned network when viewing a joined network.
- View the active network, default subnet, DHCP range, Join Key state, pending join requests, address bindings, network status, and remarks.
- Maintain member approval, virtual IP binding, and attachment remarks.
- Display readable backend validation errors in the page.

Removed from the console:

- Dedicated device management page.
- Manual subnet creation.
- Automatic network creation flow.

Default backend:

- Same-origin `/api/*`.
- Local Docker routes requests through `nginx` to `server-biz:8080`.

Local Docker / Caddy entry:

- `https://web.slan.localhost:18443`

Development:

```bash
npm install
npm start
```
