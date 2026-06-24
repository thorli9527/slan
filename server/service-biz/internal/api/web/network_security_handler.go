package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkSecurityHandler struct {
	NetworkAccess servicepkg.NetworkAccessUseCase
}

func (h NetworkSecurityHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/networks/{networkId}/security-groups", h.ListSecurityGroups),
		serviceapi.NewRoute(http.MethodPost, "/api/networks/{networkId}/security-groups", h.CreateSecurityGroup),
		serviceapi.NewRoute(http.MethodPatch, "/api/networks/{networkId}/security-groups/{securityGroupId}", h.UpdateSecurityGroup),
		serviceapi.NewRoute(http.MethodDelete, "/api/networks/{networkId}/security-groups/{securityGroupId}", h.DeleteSecurityGroup),
		serviceapi.NewRoute(http.MethodGet, "/api/security-groups/{securityGroupId}/rules", h.ListSecurityRules),
		serviceapi.NewRoute(http.MethodPost, "/api/security-groups/{securityGroupId}/rules", h.AddSecurityRule),
		serviceapi.NewRoute(http.MethodPatch, "/api/security-groups/rules/{ruleId}", h.UpdateSecurityRule),
		serviceapi.NewRoute(http.MethodDelete, "/api/security-groups/rules/{ruleId}", h.DeleteSecurityRule),
	}
}

func (h NetworkSecurityHandler) ListSecurityGroups(w http.ResponseWriter, r *http.Request) {
	items, err := h.NetworkAccess.ListSecurityGroups(r.Context(), requestNetworkID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, securityGroupPayload))
}

func (h NetworkSecurityHandler) CreateSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var req createSecurityGroupRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setNetworkAndActor(r, &input.NetworkID, &input.ActorUserID)
	item, err := h.NetworkAccess.CreateSecurityGroup(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, securityGroupPayload(item))
}

func (h NetworkSecurityHandler) UpdateSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var req updateSecurityGroupRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setSecurityGroupAndActor(r, &input.SecurityGroupID, &input.ActorUserID)
	item, err := h.NetworkAccess.UpdateSecurityGroup(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, securityGroupPayload(item))
}

func (h NetworkSecurityHandler) DeleteSecurityGroup(w http.ResponseWriter, r *http.Request) {
	input := servicepkg.DeleteSecurityGroupInput{
		SecurityGroupID: requestSecurityGroupID(r),
		ActorUserID:     requestActorUserID(r),
	}
	if err := h.NetworkAccess.DeleteSecurityGroup(r.Context(), input); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}

func (h NetworkSecurityHandler) ListSecurityRules(w http.ResponseWriter, r *http.Request) {
	items, err := h.NetworkAccess.ListSecurityRules(r.Context(), requestSecurityGroupID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, securityRulePayload))
}

func (h NetworkSecurityHandler) AddSecurityRule(w http.ResponseWriter, r *http.Request) {
	var req createSecurityRuleRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setSecurityGroupAndActor(r, &input.SecurityGroupID, &input.ActorUserID)
	item, err := h.NetworkAccess.AddSecurityRule(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, securityRulePayload(item))
}

func (h NetworkSecurityHandler) UpdateSecurityRule(w http.ResponseWriter, r *http.Request) {
	var req updateSecurityRuleRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setRuleAndActor(r, &input.RuleID, &input.ActorUserID)
	item, err := h.NetworkAccess.UpdateSecurityRule(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, securityRulePayload(item))
}

func (h NetworkSecurityHandler) DeleteSecurityRule(w http.ResponseWriter, r *http.Request) {
	input := servicepkg.DeleteSecurityRuleInput{
		RuleID:      requestRuleID(r),
		ActorUserID: requestActorUserID(r),
	}
	if err := h.NetworkAccess.DeleteSecurityRule(r.Context(), input); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
