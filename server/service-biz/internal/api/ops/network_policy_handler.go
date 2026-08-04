package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkPolicyHandler struct {
	DNS    servicepkg.NetworkDNSUseCase
	Access servicepkg.NetworkAccessUseCase
}

type networkPolicyDetailResponse struct {
	NetworkID string                        `json:"networkId"`
	DNS       networkDNSDetailResponse      `json:"dns"`
	Security  networkSecurityDetailResponse `json:"security"`
	Summary   networkPolicySummaryResponse  `json:"summary"`
}

type networkDNSDetailResponse struct {
	Zones   []servicepkg.DNSZoneView   `json:"zones"`
	Records []servicepkg.DNSRecordView `json:"records"`
}

type networkSecurityDetailResponse struct {
	Groups []servicepkg.SecurityGroupView `json:"groups"`
	Rules  []servicepkg.SecurityRuleView  `json:"rules"`
}

type networkPolicySummaryResponse struct {
	DNSZoneCount       int `json:"dnsZoneCount"`
	DNSRecordCount     int `json:"dnsRecordCount"`
	SecurityGroupCount int `json:"securityGroupCount"`
	SecurityRuleCount  int `json:"securityRuleCount"`
}

func (h NetworkPolicyHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks/{networkId}/policy", h.GetNetworkPolicyDetail),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks/{networkId}/dns/zones", h.ListDNSZones),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks/{networkId}/dns/zones", h.CreateDNSZone),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/dns/zones/{zoneId}", h.UpdateDNSZone),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/dns/zones/{zoneId}", h.DeleteDNSZone),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks/{networkId}/dns/records", h.ListDNSRecords),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks/{networkId}/dns/records", h.CreateDNSRecord),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/dns/records/{recordId}", h.UpdateDNSRecord),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/dns/records/{recordId}", h.DeleteDNSRecord),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks/{networkId}/security-groups", h.ListSecurityGroups),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks/{networkId}/security-groups", h.CreateSecurityGroup),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/security-groups/{securityGroupId}", h.UpdateSecurityGroup),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/security-groups/{securityGroupId}", h.DeleteSecurityGroup),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/security-groups/{securityGroupId}/rules", h.ListSecurityRules),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/security-groups/{securityGroupId}/rules", h.CreateSecurityRule),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/security-rules/{ruleId}", h.UpdateSecurityRule),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/security-rules/{ruleId}", h.DeleteSecurityRule),
	})
}

func (h NetworkPolicyHandler) GetNetworkPolicyDetail(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	zones, err := h.DNS.ListDNSZones(r.Context(), networkID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	records, err := h.DNS.ListDNSRecords(r.Context(), networkID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	groups, err := h.Access.ListSecurityGroups(r.Context(), networkID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	rules := make([]servicepkg.SecurityRuleView, 0)
	for _, group := range groups {
		items, listErr := h.Access.ListSecurityRules(r.Context(), group.SecurityGroupID)
		if listErr != nil {
			serviceapi.WriteError(w, listErr)
			return
		}
		rules = append(rules, items...)
	}
	serviceapi.WriteJSON(w, http.StatusOK, networkPolicyDetailResponse{
		NetworkID: networkID,
		DNS:       networkDNSDetailResponse{Zones: zones, Records: records},
		Security:  networkSecurityDetailResponse{Groups: groups, Rules: rules},
		Summary: networkPolicySummaryResponse{
			DNSZoneCount:       len(zones),
			DNSRecordCount:     len(records),
			SecurityGroupCount: len(groups),
			SecurityRuleCount:  len(rules),
		},
	})
}

func writePolicyResult[T any](w http.ResponseWriter, item T, err error, status int) {
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, status, item)
}

func (h NetworkPolicyHandler) ListDNSZones(w http.ResponseWriter, r *http.Request) {
	items, err := h.DNS.ListDNSZones(r.Context(), r.PathValue("networkId"))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, items)
}
func (h NetworkPolicyHandler) CreateDNSZone(w http.ResponseWriter, r *http.Request) {
	var in servicepkg.CreateDNSZoneInput
	if !serviceapi.DecodeJSONOrError(w, r, &in) {
		return
	}
	in.NetworkID = r.PathValue("networkId")
	item, err := h.DNS.AddDNSZone(r.Context(), in)
	writePolicyResult(w, item, err, http.StatusCreated)
}
func (h NetworkPolicyHandler) UpdateDNSZone(w http.ResponseWriter, r *http.Request) {
	var in servicepkg.UpdateDNSZoneInput
	if !serviceapi.DecodeJSONOrError(w, r, &in) {
		return
	}
	in.ZoneID = r.PathValue("zoneId")
	item, err := h.DNS.UpdateDNSZone(r.Context(), in)
	writePolicyResult(w, item, err, http.StatusOK)
}
func (h NetworkPolicyHandler) DeleteDNSZone(w http.ResponseWriter, r *http.Request) {
	if err := h.DNS.DeleteDNSZone(r.Context(), servicepkg.DeleteDNSZoneInput{ZoneID: r.PathValue("zoneId")}); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
func (h NetworkPolicyHandler) ListDNSRecords(w http.ResponseWriter, r *http.Request) {
	items, err := h.DNS.ListDNSRecords(r.Context(), r.PathValue("networkId"))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, items)
}
func (h NetworkPolicyHandler) CreateDNSRecord(w http.ResponseWriter, r *http.Request) {
	var in servicepkg.CreateDNSRecordInput
	if !serviceapi.DecodeJSONOrError(w, r, &in) {
		return
	}
	in.NetworkID = r.PathValue("networkId")
	item, err := h.DNS.AddDNSRecord(r.Context(), in)
	writePolicyResult(w, item, err, http.StatusCreated)
}
func (h NetworkPolicyHandler) UpdateDNSRecord(w http.ResponseWriter, r *http.Request) {
	var in servicepkg.UpdateDNSRecordInput
	if !serviceapi.DecodeJSONOrError(w, r, &in) {
		return
	}
	in.RecordID = r.PathValue("recordId")
	item, err := h.DNS.UpdateDNSRecord(r.Context(), in)
	writePolicyResult(w, item, err, http.StatusOK)
}
func (h NetworkPolicyHandler) DeleteDNSRecord(w http.ResponseWriter, r *http.Request) {
	if err := h.DNS.DeleteDNSRecord(r.Context(), servicepkg.DeleteDNSRecordInput{RecordID: r.PathValue("recordId")}); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
func (h NetworkPolicyHandler) ListSecurityGroups(w http.ResponseWriter, r *http.Request) {
	items, err := h.Access.ListSecurityGroups(r.Context(), r.PathValue("networkId"))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, items)
}
func (h NetworkPolicyHandler) CreateSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var in servicepkg.CreateSecurityGroupInput
	if !serviceapi.DecodeJSONOrError(w, r, &in) {
		return
	}
	in.NetworkID = r.PathValue("networkId")
	item, err := h.Access.CreateSecurityGroup(r.Context(), in)
	writePolicyResult(w, item, err, http.StatusCreated)
}
func (h NetworkPolicyHandler) UpdateSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var in servicepkg.UpdateSecurityGroupInput
	if !serviceapi.DecodeJSONOrError(w, r, &in) {
		return
	}
	in.SecurityGroupID = r.PathValue("securityGroupId")
	item, err := h.Access.UpdateSecurityGroup(r.Context(), in)
	writePolicyResult(w, item, err, http.StatusOK)
}
func (h NetworkPolicyHandler) DeleteSecurityGroup(w http.ResponseWriter, r *http.Request) {
	if err := h.Access.DeleteSecurityGroup(r.Context(), servicepkg.DeleteSecurityGroupInput{SecurityGroupID: r.PathValue("securityGroupId")}); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
func (h NetworkPolicyHandler) ListSecurityRules(w http.ResponseWriter, r *http.Request) {
	items, err := h.Access.ListSecurityRules(r.Context(), r.PathValue("securityGroupId"))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, items)
}
func (h NetworkPolicyHandler) CreateSecurityRule(w http.ResponseWriter, r *http.Request) {
	var in servicepkg.CreateSecurityRuleInput
	if !serviceapi.DecodeJSONOrError(w, r, &in) {
		return
	}
	in.SecurityGroupID = r.PathValue("securityGroupId")
	item, err := h.Access.AddSecurityRule(r.Context(), in)
	writePolicyResult(w, item, err, http.StatusCreated)
}
func (h NetworkPolicyHandler) UpdateSecurityRule(w http.ResponseWriter, r *http.Request) {
	var in servicepkg.UpdateSecurityRuleInput
	if !serviceapi.DecodeJSONOrError(w, r, &in) {
		return
	}
	in.RuleID = r.PathValue("ruleId")
	item, err := h.Access.UpdateSecurityRule(r.Context(), in)
	writePolicyResult(w, item, err, http.StatusOK)
}
func (h NetworkPolicyHandler) DeleteSecurityRule(w http.ResponseWriter, r *http.Request) {
	if err := h.Access.DeleteSecurityRule(r.Context(), servicepkg.DeleteSecurityRuleInput{RuleID: r.PathValue("ruleId")}); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
