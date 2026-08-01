package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type ResourceHandler struct {
	Users         servicepkg.OpsUserUseCase
	DeviceGroups  servicepkg.DeviceGroupUseCase
	Networks      servicepkg.NetworkCoreUseCase
	NetworkInvite servicepkg.NetworkInviteUseCase
	DNS           servicepkg.NetworkDNSUseCase
	Access        servicepkg.NetworkAccessUseCase
	Audit         servicepkg.OpsAuditUseCase
}

type ownerRequest struct {
	OwnerID string `json:"ownerId"`
}
type deviceGroupsRequest struct {
	OwnerID  string   `json:"ownerId"`
	GroupIDs []string `json:"groupIds"`
}
type networkGroupRequest struct {
	OwnerID string `json:"ownerId"`
	GroupID string `json:"groupId"`
}
type networkRequest struct {
	OwnerID, Name, CIDR, IntraGroupPolicy, Status string
	Default                                       *bool `json:"default,omitempty"`
}
type groupRequest struct{ OwnerID, Name, Description string }
type securityGroupRequest struct{ OwnerID, Name, Description string }
type securityRuleRequest struct {
	OwnerID, Direction, Protocol, PortRange, PeerType, PeerValue, Action, Description string
	Priority                                                                          int
	Enabled                                                                           bool
}
type dnsZoneRequest struct{ OwnerID, Name, Status string }
type dnsRecordRequest struct {
	OwnerID, ZoneID, Name, Type, Value, Port string
	TTL                                      int
}

func (h ResourceHandler) Routes() []serviceapi.Route {
	return withOptAliases(withResourceAudit([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/device-groups", h.listGroups),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/device-groups", h.createGroup),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/device-groups/{groupId}", h.updateGroup),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/device-groups/{groupId}", h.deleteGroup),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/device-groups/{groupId}/members", h.listGroupMembers),
		serviceapi.NewRoute(http.MethodPut, "/api/ops/devices/{deviceId}/groups", h.setDeviceGroups),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks", h.listNetworks),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks", h.createNetwork),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/networks/{networkId}", h.updateNetwork),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/networks/{networkId}", h.deleteNetwork),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks/{networkId}/devices", h.listNetworkDevices),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks/{networkId}/devices/{deviceId}", h.addNetworkDevice),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/networks/{networkId}/devices/{deviceId}", h.removeNetworkDevice),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks/{networkId}/device-groups", h.listNetworkGroups),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks/{networkId}/device-groups", h.addNetworkGroup),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/networks/{networkId}/device-groups/{groupId}", h.removeNetworkGroup),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks/{networkId}/security-groups", h.listSecurityGroups),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks/{networkId}/security-groups", h.createSecurityGroup),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/security-groups/{securityGroupId}", h.updateSecurityGroup),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/security-groups/{securityGroupId}", h.deleteSecurityGroup),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/security-groups/{securityGroupId}/rules", h.listSecurityRules),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/security-groups/{securityGroupId}/rules", h.createSecurityRule),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/security-rules/{ruleId}", h.updateSecurityRule),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/security-rules/{ruleId}", h.deleteSecurityRule),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks/{networkId}/dns/zones", h.listDNSZones),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks/{networkId}/dns/zones", h.createDNSZone),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/dns/zones/{zoneId}", h.updateDNSZone),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/dns/zones/{zoneId}", h.deleteDNSZone),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks/{networkId}/dns/records", h.listDNSRecords),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks/{networkId}/dns/records", h.createDNSRecord),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/dns/records/{recordId}", h.updateDNSRecord),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/dns/records/{recordId}", h.deleteDNSRecord),
	}, h.Audit))
}

func decode[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	return v, serviceapi.DecodeJSONOrError(w, r, &v)
}
func fail(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	serviceapi.WriteError(w, err)
	return true
}

func (h ResourceHandler) owners(r *http.Request) ([]servicepkg.OpsUserView, error) {
	return h.Users.ListUsers(r.Context())
}
func (h ResourceHandler) listGroups(w http.ResponseWriter, r *http.Request) {
	owners, err := h.owners(r)
	if fail(w, err) {
		return
	}
	out := []servicepkg.DeviceGroupView{}
	members := []servicepkg.DeviceGroupMemberView{}
	for _, o := range owners {
		v, e := h.DeviceGroups.ListDeviceGroups(r.Context(), o.User.UserID)
		if fail(w, e) {
			return
		}
		out = append(out, v.Items...)
		members = append(members, v.Members...)
	}
	serviceapi.WriteItemsWithMembers(w, out, members)
}
func (h ResourceHandler) createGroup(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[groupRequest](w, r)
	if !ok {
		return
	}
	v, e := h.DeviceGroups.CreateDeviceGroup(r.Context(), servicepkg.CreateDeviceGroupInput{UserID: q.OwnerID, ActorUserID: q.OwnerID, Name: q.Name, Description: q.Description})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, v)
}
func (h ResourceHandler) updateGroup(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[groupRequest](w, r)
	if !ok {
		return
	}
	v, e := h.DeviceGroups.UpdateDeviceGroup(r.Context(), servicepkg.UpdateDeviceGroupInput{GroupID: r.PathValue("groupId"), ActorUserID: q.OwnerID, Name: q.Name, Description: q.Description})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, v)
}
func (h ResourceHandler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[ownerRequest](w, r)
	if !ok {
		return
	}
	if fail(w, h.DeviceGroups.DeleteDeviceGroup(r.Context(), servicepkg.DeleteDeviceGroupInput{GroupID: r.PathValue("groupId"), ActorUserID: q.OwnerID})) {
		return
	}
	serviceapi.WriteNoContent(w)
}
func (h ResourceHandler) listGroupMembers(w http.ResponseWriter, r *http.Request) {
	ownerID := r.URL.Query().Get("ownerId")
	items, e := h.DeviceGroups.ListDeviceGroupMembers(r.Context(), ownerID)
	if fail(w, e) {
		return
	}
	groupID := r.PathValue("groupId")
	out := []servicepkg.DeviceGroupMemberView{}
	for _, item := range items {
		if item.GroupID == groupID {
			out = append(out, item)
		}
	}
	serviceapi.WriteItems(w, out)
}
func (h ResourceHandler) setDeviceGroups(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[deviceGroupsRequest](w, r)
	if !ok {
		return
	}
	if fail(w, h.DeviceGroups.SetDeviceGroups(r.Context(), servicepkg.SetDeviceGroupsInput{UserID: q.OwnerID, ActorUserID: q.OwnerID, DeviceID: r.PathValue("deviceId"), GroupIDs: q.GroupIDs})) {
		return
	}
	serviceapi.WriteNoContent(w)
}

func (h ResourceHandler) listNetworks(w http.ResponseWriter, r *http.Request) {
	owners, e := h.owners(r)
	if fail(w, e) {
		return
	}
	out := []servicepkg.NetworkSummaryView{}
	for _, o := range owners {
		v, x := h.Networks.ListNetworks(r.Context(), o.User.UserID)
		if fail(w, x) {
			return
		}
		out = append(out, v...)
	}
	serviceapi.WriteItems(w, out)
}
func (h ResourceHandler) createNetwork(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[networkRequest](w, r)
	if !ok {
		return
	}
	v, e := h.Networks.CreateNetwork(r.Context(), servicepkg.CreateNetworkInput{OwnerID: q.OwnerID, ActorUserID: q.OwnerID, Name: q.Name, CIDR: q.CIDR, IntraGroupPolicy: q.IntraGroupPolicy})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, v)
}
func (h ResourceHandler) updateNetwork(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[networkRequest](w, r)
	if !ok {
		return
	}
	v, e := h.Networks.UpdateNetwork(r.Context(), servicepkg.UpdateNetworkInput{NetworkID: r.PathValue("networkId"), ActorUserID: q.OwnerID, Name: q.Name, CIDR: q.CIDR, IntraGroupPolicy: q.IntraGroupPolicy, Default: q.Default, Status: q.Status})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, v)
}
func (h ResourceHandler) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[ownerRequest](w, r)
	if !ok {
		return
	}
	if fail(w, h.Networks.DeleteNetwork(r.Context(), servicepkg.DeleteNetworkInput{NetworkID: r.PathValue("networkId"), ActorUserID: q.OwnerID})) {
		return
	}
	serviceapi.WriteNoContent(w)
}
func (h ResourceHandler) listNetworkDevices(w http.ResponseWriter, r *http.Request) {
	v, e := h.NetworkInvite.ListNetworkDevices(r.Context(), r.PathValue("networkId"))
	if fail(w, e) {
		return
	}
	serviceapi.WriteItems(w, v)
}
func (h ResourceHandler) addNetworkDevice(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[ownerRequest](w, r)
	if !ok {
		return
	}
	v, e := h.NetworkInvite.AddNetworkDevice(r.Context(), servicepkg.AddNetworkDeviceInput{NetworkID: r.PathValue("networkId"), DeviceID: r.PathValue("deviceId"), ActorUserID: q.OwnerID})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, v)
}
func (h ResourceHandler) removeNetworkDevice(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[ownerRequest](w, r)
	if !ok {
		return
	}
	if fail(w, h.NetworkInvite.RemoveNetworkDevice(r.Context(), servicepkg.RemoveNetworkDeviceInput{NetworkID: r.PathValue("networkId"), DeviceID: r.PathValue("deviceId"), ActorUserID: q.OwnerID})) {
		return
	}
	serviceapi.WriteNoContent(w)
}
func (h ResourceHandler) listNetworkGroups(w http.ResponseWriter, r *http.Request) {
	v, e := h.DeviceGroups.ListNetworkDeviceGroups(r.Context(), r.PathValue("networkId"))
	if fail(w, e) {
		return
	}
	serviceapi.WriteItemsWithMembers(w, v.Items, v.Members)
}
func (h ResourceHandler) addNetworkGroup(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[networkGroupRequest](w, r)
	if !ok {
		return
	}
	v, e := h.DeviceGroups.AddNetworkDeviceGroup(r.Context(), servicepkg.AddNetworkDeviceGroupInput{NetworkID: r.PathValue("networkId"), GroupID: q.GroupID, ActorUserID: q.OwnerID})
	if fail(w, e) {
		return
	}
	serviceapi.WriteItemsWithMembers(w, v.Items, v.Members)
}
func (h ResourceHandler) removeNetworkGroup(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[ownerRequest](w, r)
	if !ok {
		return
	}
	v, e := h.DeviceGroups.RemoveNetworkDeviceGroup(r.Context(), servicepkg.RemoveNetworkDeviceGroupInput{NetworkID: r.PathValue("networkId"), GroupID: r.PathValue("groupId"), ActorUserID: q.OwnerID})
	if fail(w, e) {
		return
	}
	serviceapi.WriteItemsWithMembers(w, v.Items, v.Members)
}

func (h ResourceHandler) listSecurityGroups(w http.ResponseWriter, r *http.Request) {
	v, e := h.Access.ListSecurityGroups(r.Context(), r.PathValue("networkId"))
	if fail(w, e) {
		return
	}
	serviceapi.WriteItems(w, v)
}
func (h ResourceHandler) createSecurityGroup(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[securityGroupRequest](w, r)
	if !ok {
		return
	}
	v, e := h.Access.CreateSecurityGroup(r.Context(), servicepkg.CreateSecurityGroupInput{NetworkID: r.PathValue("networkId"), ActorUserID: q.OwnerID, Name: q.Name, Description: q.Description})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, v)
}
func (h ResourceHandler) updateSecurityGroup(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[securityGroupRequest](w, r)
	if !ok {
		return
	}
	v, e := h.Access.UpdateSecurityGroup(r.Context(), servicepkg.UpdateSecurityGroupInput{SecurityGroupID: r.PathValue("securityGroupId"), ActorUserID: q.OwnerID, Name: q.Name, Description: q.Description})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, v)
}
func (h ResourceHandler) deleteSecurityGroup(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[ownerRequest](w, r)
	if !ok {
		return
	}
	if fail(w, h.Access.DeleteSecurityGroup(r.Context(), servicepkg.DeleteSecurityGroupInput{SecurityGroupID: r.PathValue("securityGroupId"), ActorUserID: q.OwnerID})) {
		return
	}
	serviceapi.WriteNoContent(w)
}
func (h ResourceHandler) listSecurityRules(w http.ResponseWriter, r *http.Request) {
	v, e := h.Access.ListSecurityRules(r.Context(), r.PathValue("securityGroupId"))
	if fail(w, e) {
		return
	}
	serviceapi.WriteItems(w, v)
}
func (h ResourceHandler) createSecurityRule(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[securityRuleRequest](w, r)
	if !ok {
		return
	}
	v, e := h.Access.AddSecurityRule(r.Context(), servicepkg.CreateSecurityRuleInput{SecurityGroupID: r.PathValue("securityGroupId"), ActorUserID: q.OwnerID, Direction: q.Direction, Protocol: q.Protocol, PortRange: q.PortRange, PeerType: q.PeerType, PeerValue: q.PeerValue, Action: q.Action, Priority: q.Priority, Description: q.Description, Enabled: q.Enabled})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, v)
}
func (h ResourceHandler) updateSecurityRule(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[securityRuleRequest](w, r)
	if !ok {
		return
	}
	priority, enabled := q.Priority, q.Enabled
	v, e := h.Access.UpdateSecurityRule(r.Context(), servicepkg.UpdateSecurityRuleInput{RuleID: r.PathValue("ruleId"), ActorUserID: q.OwnerID, Direction: q.Direction, Protocol: q.Protocol, PortRange: q.PortRange, PeerType: q.PeerType, PeerValue: q.PeerValue, Action: q.Action, Priority: &priority, Description: q.Description, Enabled: &enabled})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, v)
}
func (h ResourceHandler) deleteSecurityRule(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[ownerRequest](w, r)
	if !ok {
		return
	}
	if fail(w, h.Access.DeleteSecurityRule(r.Context(), servicepkg.DeleteSecurityRuleInput{RuleID: r.PathValue("ruleId"), ActorUserID: q.OwnerID})) {
		return
	}
	serviceapi.WriteNoContent(w)
}

func (h ResourceHandler) listDNSZones(w http.ResponseWriter, r *http.Request) {
	v, e := h.DNS.ListDNSZones(r.Context(), r.PathValue("networkId"))
	if fail(w, e) {
		return
	}
	serviceapi.WriteItems(w, v)
}
func (h ResourceHandler) createDNSZone(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[dnsZoneRequest](w, r)
	if !ok {
		return
	}
	v, e := h.DNS.AddDNSZone(r.Context(), servicepkg.CreateDNSZoneInput{NetworkID: r.PathValue("networkId"), ActorUserID: q.OwnerID, Name: q.Name})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, v)
}
func (h ResourceHandler) updateDNSZone(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[dnsZoneRequest](w, r)
	if !ok {
		return
	}
	v, e := h.DNS.UpdateDNSZone(r.Context(), servicepkg.UpdateDNSZoneInput{ZoneID: r.PathValue("zoneId"), ActorUserID: q.OwnerID, Name: q.Name, Status: q.Status})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, v)
}
func (h ResourceHandler) deleteDNSZone(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[ownerRequest](w, r)
	if !ok {
		return
	}
	if fail(w, h.DNS.DeleteDNSZone(r.Context(), servicepkg.DeleteDNSZoneInput{ZoneID: r.PathValue("zoneId"), ActorUserID: q.OwnerID})) {
		return
	}
	serviceapi.WriteNoContent(w)
}
func (h ResourceHandler) listDNSRecords(w http.ResponseWriter, r *http.Request) {
	v, e := h.DNS.ListDNSRecords(r.Context(), r.PathValue("networkId"))
	if fail(w, e) {
		return
	}
	serviceapi.WriteItems(w, v)
}
func (h ResourceHandler) createDNSRecord(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[dnsRecordRequest](w, r)
	if !ok {
		return
	}
	v, e := h.DNS.AddDNSRecord(r.Context(), servicepkg.CreateDNSRecordInput{NetworkID: r.PathValue("networkId"), ActorUserID: q.OwnerID, ZoneID: q.ZoneID, Name: q.Name, Type: q.Type, Value: q.Value, Port: q.Port, TTL: q.TTL})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, v)
}
func (h ResourceHandler) updateDNSRecord(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[dnsRecordRequest](w, r)
	if !ok {
		return
	}
	v, e := h.DNS.UpdateDNSRecord(r.Context(), servicepkg.UpdateDNSRecordInput{RecordID: r.PathValue("recordId"), ActorUserID: q.OwnerID, ZoneID: q.ZoneID, Name: q.Name, Type: q.Type, Value: q.Value, Port: q.Port, TTL: q.TTL})
	if fail(w, e) {
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, v)
}
func (h ResourceHandler) deleteDNSRecord(w http.ResponseWriter, r *http.Request) {
	q, ok := decode[ownerRequest](w, r)
	if !ok {
		return
	}
	if fail(w, h.DNS.DeleteDNSRecord(r.Context(), servicepkg.DeleteDNSRecordInput{RecordID: r.PathValue("recordId"), ActorUserID: q.OwnerID})) {
		return
	}
	serviceapi.WriteNoContent(w)
}
