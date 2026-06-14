package biz

import "net/http"

// apiRoute 描述一个 HTTP API 入口，集中承载方法、路径和 handler 绑定。
type apiRoute struct {
	method  string
	path    string
	handler http.HandlerFunc
}

func route(method, path string, handler http.HandlerFunc) apiRoute {
	return apiRoute{method: method, path: path, handler: handler}
}

func registerAPIRoutes(mux *http.ServeMux, routes []apiRoute) {
	for _, item := range routes {
		mux.HandleFunc(item.method+" "+item.path, item.handler)
	}
}

func (s *Server) registerBaseRoutes(mux *http.ServeMux) {
	registerAPIRoutes(mux, []apiRoute{
		route(http.MethodGet, "/healthz", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		}),
		route(http.MethodGet, "/api/client-downloads", s.listClientDownloads),
		route(http.MethodGet, "/downloads/clients/{fileName}", s.downloadClientFile),
	})
}

func (s *Server) registerExternalAPIRoutes(mux *http.ServeMux) {
	registerAPIRoutes(mux, []apiRoute{
		route(http.MethodPost, "/api/auth/register", s.registerUser),
		route(http.MethodPost, "/api/auth/login", s.loginUser),
		route(http.MethodPost, "/api/auth/renew", s.renewUserSession),
		route(http.MethodPost, "/api/auth/logout", s.logoutUser),
		route(http.MethodPost, "/api/auth/console-login-keys", s.createConsoleLoginKey),
		route(http.MethodPost, "/api/auth/console-login", s.consoleLogin),
		route(http.MethodPost, "/api/auth/device-login-devices", s.prepareDeviceLoginDevice),
		route(http.MethodPost, "/api/auth/device-login-devices/{deviceId}/complete", s.completeDeviceLoginDevice),
		route(http.MethodPost, "/api/web/device-bootstrap-keys", s.createDeviceBootstrapKey),
		route(http.MethodGet, "/api/web/device-bootstrap-keys", s.listDeviceBootstrapKeys),
		route(http.MethodPost, "/api/web/device-bootstrap-keys/{keyId}/revoke", s.revokeDeviceBootstrapKey),
		route(http.MethodGet, "/api/users", s.listUsers),
		route(http.MethodGet, "/api/users/{userId}/entitlement", s.userEntitlement),
		route(http.MethodPatch, "/api/users/{userId}/password", s.changeUserPassword),
		route(http.MethodGet, "/api/users/{userId}/device-bootstrap-keys", s.listDeviceBootstrapKeys),
		route(http.MethodGet, "/api/users/{userId}/user-aliases", s.listUserAliases),
		route(http.MethodGet, "/api/users/{userId}/devices/visible", s.listVisibleDevices),
		route(http.MethodGet, "/api/users/{userId}/device-invites", s.listDeviceInvites),
		route(http.MethodGet, "/api/users/{userId}/networks", s.listNetworks),
		route(http.MethodGet, "/api/user-aliases", s.listUserAliases),
		route(http.MethodPatch, "/api/user-aliases", s.upsertUserAlias),

		route(http.MethodGet, "/api/devices", s.listDevices),
		route(http.MethodGet, "/api/devices/visible", s.listVisibleDevices),
		route(http.MethodPost, "/api/devices/register", s.registerDevice),
		route(http.MethodPost, "/api/devices/{deviceId}/renew", s.renewDevice),
		route(http.MethodPost, "/api/device/session/bootstrap", s.bootstrapDeviceSession),
		route(http.MethodPost, "/api/device/session/bind", s.bindDeviceSession),
		route(http.MethodPost, "/api/device/session/renew", s.renewDeviceSession),
		route(http.MethodGet, "/api/devices/{deviceId}/network-configs", s.deviceNetworkConfigs),
		route(http.MethodGet, "/api/devices/{deviceId}/mqtt-credential", s.deviceMQTTCredential),
		route(http.MethodPatch, "/api/devices/{deviceId}", s.updateDeviceAlias),
		route(http.MethodDelete, "/api/devices/{deviceId}", s.deleteDevice),
		route(http.MethodPost, "/api/device-invites", s.createDeviceInvite),
		route(http.MethodGet, "/api/device-invites", s.listDeviceInvites),
		route(http.MethodPost, "/api/device-invites/accept", s.acceptDeviceInvite),

		route(http.MethodGet, "/api/networks", s.listNetworks),
		route(http.MethodPost, "/api/networks", s.createNetwork),
		route(http.MethodPatch, "/api/networks/{networkId}", s.updateNetwork),
		route(http.MethodGet, "/api/networks/{networkId}/devices", s.listNetworkDevices),
		route(http.MethodPost, "/api/networks/{networkId}/devices", s.addNetworkDevice),
		route(http.MethodPatch, "/api/networks/{networkId}/devices/{deviceId}", s.updateNetworkDevice),
		route(http.MethodDelete, "/api/networks/{networkId}/devices/{deviceId}", s.removeNetworkDevice),
		route(http.MethodGet, "/api/networks/{networkId}/dns/zones", s.listDNSZones),
		route(http.MethodPost, "/api/networks/{networkId}/dns/zones", s.addDNSZone),
		route(http.MethodPatch, "/api/networks/{networkId}/dns/zones/{zoneId}", s.updateDNSZone),
		route(http.MethodDelete, "/api/networks/{networkId}/dns/zones/{zoneId}", s.deleteDNSZone),
		route(http.MethodGet, "/api/networks/{networkId}/dns/records", s.listDNSRecords),
		route(http.MethodPost, "/api/networks/{networkId}/dns/records", s.addDNSRecord),
		route(http.MethodPatch, "/api/networks/{networkId}/dns/records/{recordId}", s.updateDNSRecord),
		route(http.MethodDelete, "/api/networks/{networkId}/dns/records/{recordId}", s.deleteDNSRecord),
		route(http.MethodGet, "/api/networks/{networkId}/public-mappings", s.listPublicMappings),
		route(http.MethodPost, "/api/networks/{networkId}/public-mappings", s.createPublicMapping),
		route(http.MethodPatch, "/api/networks/{networkId}/public-mappings/{mappingId}", s.updatePublicMapping),
		route(http.MethodDelete, "/api/networks/{networkId}/public-mappings/{mappingId}", s.deletePublicMapping),
		route(http.MethodGet, "/api/networks/{networkId}/security-groups", s.listSecurityGroups),
		route(http.MethodPost, "/api/networks/{networkId}/security-groups", s.createSecurityGroup),
		route(http.MethodDelete, "/api/networks/{networkId}/security-groups/{securityGroupId}", s.deleteSecurityGroup),
		route(http.MethodGet, "/api/security-groups/{securityGroupId}/rules", s.listSecurityRules),
		route(http.MethodPost, "/api/security-groups/{securityGroupId}/rules", s.addSecurityRule),
		route(http.MethodPatch, "/api/security-groups/rules/{ruleId}", s.updateSecurityRule),
		route(http.MethodDelete, "/api/security-groups/rules/{ruleId}", s.deleteSecurityRule),
		route(http.MethodGet, "/api/networks/{networkId}/network-config", s.networkConfig),
		route(http.MethodGet, "/api/networks/{networkId}/relay-candidates", s.relayCandidates),
		route(http.MethodPost, "/api/networks/{networkId}/relay-candidates", s.relayCandidates),
		route(http.MethodPost, "/api/networks/{networkId}/punch/connect-sessions", s.createPunchConnectSession),
		route(http.MethodPost, "/api/relay/tickets", s.issueRelayTicket),
	})
}

func (s *Server) registerMQTTRoutes(mux *http.ServeMux) {
	registerAPIRoutes(mux, []apiRoute{
		route(http.MethodPost, "/mqtt/bifromq/auth", s.bifroMQAuth),
		route(http.MethodPost, "/mqtt/bifromq/check", s.bifroMQCheck),
	})
}
