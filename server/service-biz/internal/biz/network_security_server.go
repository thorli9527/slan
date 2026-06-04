package biz

import (
	"net/http"
)

func (s *Server) listSecurityGroups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Network.ListSecurityGroups(r.PathValue("networkId"))})
}

func (s *Server) createSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateSecurityGroupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	group, err := s.services.Network.CreateSecurityGroup(r.PathValue("networkId"), req)
	if err != nil {
		s.recordNetworkMutationAudit(r, "security_group.add", "security_group", "", r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "name": req.Name, "defaultPolicy": req.DefaultPolicy})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "security_group.add", "security_group", group.SecurityGroupID, group.NetworkID, "", "succeeded", map[string]string{"name": group.Name, "defaultPolicy": group.DefaultPolicy})
	s.notifyNetworkConfigChanged(group.NetworkID, "security_group_added", "security_group", "add", group.SecurityGroupID, "")
	writeJSON(w, http.StatusCreated, group)
}

func (s *Server) deleteSecurityGroup(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	securityGroupID := r.PathValue("securityGroupId")
	if err := s.services.Network.DeleteSecurityGroup(networkID, securityGroupID); err != nil {
		s.recordNetworkMutationAudit(r, "security_group.delete", "security_group", securityGroupID, networkID, "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "security_group.delete", "security_group", securityGroupID, networkID, "", "succeeded", nil)
	s.notifyNetworkConfigChanged(networkID, "security_group_removed", "security_group", "remove", securityGroupID, "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSecurityRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Network.ListSecurityRules(r.PathValue("securityGroupId"))})
}

func (s *Server) addSecurityRule(w http.ResponseWriter, r *http.Request) {
	var req SecurityRuleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rule, _, err := s.services.Network.AddSecurityRule(r.PathValue("securityGroupId"), req)
	if err != nil {
		s.recordNetworkMutationAudit(r, "security_rule.add", "security_rule", "", "", "", "failed", map[string]string{"error": err.Error(), "securityGroupId": r.PathValue("securityGroupId"), "direction": req.Direction, "action": req.Action})
		writeError(w, err)
		return
	}
	networkID, err := s.services.Network.SecurityGroupNetworkID(rule.SecurityGroupID)
	if err == nil {
		s.recordNetworkMutationAudit(r, "security_rule.add", "security_rule", rule.RuleID, networkID, "", "succeeded", map[string]string{"securityGroupId": rule.SecurityGroupID, "direction": rule.Direction, "action": rule.Action, "enabled": boolString(rule.Enabled)})
		s.notifyNetworkConfigChanged(networkID, securityRuleReason(rule.Direction, "added"), "security_rule", "add", rule.RuleID, "")
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) updateSecurityRule(w http.ResponseWriter, r *http.Request) {
	var req SecurityRuleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rule, _, err := s.services.Network.UpdateSecurityRule(r.PathValue("ruleId"), req)
	if err != nil {
		s.recordNetworkMutationAudit(r, "security_rule.update", "security_rule", r.PathValue("ruleId"), "", "", "failed", map[string]string{"error": err.Error(), "direction": req.Direction, "action": req.Action})
		writeError(w, err)
		return
	}
	networkID, err := s.services.Network.SecurityGroupNetworkID(rule.SecurityGroupID)
	if err == nil {
		s.recordNetworkMutationAudit(r, "security_rule.update", "security_rule", rule.RuleID, networkID, "", "succeeded", map[string]string{"securityGroupId": rule.SecurityGroupID, "direction": rule.Direction, "action": rule.Action, "enabled": boolString(rule.Enabled)})
		s.notifyNetworkConfigChanged(networkID, securityRuleReason(rule.Direction, "updated"), "security_rule", "update", rule.RuleID, "")
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) deleteSecurityRule(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("ruleId")
	rule, networkID, err := s.services.Network.DeleteSecurityRule(ruleID)
	if err != nil {
		s.recordNetworkMutationAudit(r, "security_rule.delete", "security_rule", ruleID, networkID, "", "failed", map[string]string{"error": err.Error(), "securityGroupId": rule.SecurityGroupID})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "security_rule.delete", "security_rule", ruleID, networkID, "", "succeeded", map[string]string{"securityGroupId": rule.SecurityGroupID, "direction": rule.Direction})
	s.notifyNetworkConfigChanged(networkID, securityRuleReason(rule.Direction, "removed"), "security_rule", "remove", ruleID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
