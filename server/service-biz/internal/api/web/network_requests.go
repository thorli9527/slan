package web

import (
	"strconv"
	"strings"

	servicepkg "github.com/slan/service-biz/internal/service"
)

type createNetworkFields struct {
	ActorUserID      string `json:"actorUserId"`
	Name             string `json:"name"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
	Default          bool   `json:"default"`
}

func (f createNetworkFields) createInput(ownerID string) servicepkg.CreateNetworkInput {
	return servicepkg.CreateNetworkInput{
		OwnerID:          ownerID,
		ActorUserID:      f.ActorUserID,
		Name:             f.Name,
		IntraGroupPolicy: f.IntraGroupPolicy,
		Default:          f.Default,
	}
}

type updateNetworkFields struct {
	Name             string `json:"name"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
	Default          *bool  `json:"default,omitempty"`
}

func (f updateNetworkFields) updateInput(status string) servicepkg.UpdateNetworkInput {
	return servicepkg.UpdateNetworkInput{
		Name:             f.Name,
		IntraGroupPolicy: f.IntraGroupPolicy,
		Default:          f.Default,
		Status:           status,
	}
}

type createNetworkRequest struct {
	OwnerUserID string `json:"ownerUserId"`
	OwnerID     string `json:"ownerId"`
	createNetworkFields
}

func (r createNetworkRequest) toInput() servicepkg.CreateNetworkInput {
	return r.createNetworkFields.createInput(firstNonEmpty(r.OwnerID, r.OwnerUserID))
}

type updateNetworkRequest struct {
	updateNetworkFields
	ActorUserID string `json:"actorUserId"`
	Status      string `json:"status"`
}

func (r updateNetworkRequest) toInput() servicepkg.UpdateNetworkInput {
	input := r.updateNetworkFields.updateInput(r.Status)
	input.ActorUserID = r.ActorUserID
	return input
}

type createDeviceInviteRequest struct {
	NetworkID     string `json:"networkId"`
	DeviceID      string `json:"deviceId"`
	UserID        string `json:"userId"`
	InviterUserID string `json:"inviterUserId"`
	OwnerUserID   string `json:"ownerUserId"`
	TTLSeconds    int64  `json:"ttlSeconds"`
}

func (r createDeviceInviteRequest) toInput() servicepkg.CreateDeviceInviteInput {
	return servicepkg.CreateDeviceInviteInput{
		NetworkID:     r.NetworkID,
		DeviceID:      r.DeviceID,
		UserID:        r.UserID,
		InviterUserID: r.InviterUserID,
		OwnerUserID:   r.OwnerUserID,
		TTLSeconds:    r.TTLSeconds,
	}
}

type acceptDeviceInviteRequest struct {
	InviteID    string `json:"inviteId"`
	InviteCode  string `json:"inviteCode"`
	DeviceID    string `json:"deviceId"`
	UserID      string `json:"userId"`
	ActorUserID string `json:"actorUserId"`
	Alias       string `json:"alias"`
}

func (r acceptDeviceInviteRequest) toInput() servicepkg.AcceptDeviceInviteInput {
	return servicepkg.AcceptDeviceInviteInput{
		InviteID:    r.InviteID,
		InviteCode:  r.InviteCode,
		DeviceID:    r.DeviceID,
		UserID:      r.UserID,
		ActorUserID: r.ActorUserID,
		Alias:       r.Alias,
	}
}

type revokeDeviceInviteRequest struct {
	ActorUserID string `json:"actorUserId"`
}

func (r revokeDeviceInviteRequest) toInput() servicepkg.RevokeDeviceInviteInput {
	return servicepkg.RevokeDeviceInviteInput{ActorUserID: r.ActorUserID}
}

type createDNSZoneFields struct {
	ActorUserID string `json:"actorUserId"`
	ZoneName    string `json:"zoneName"`
}

func (f createDNSZoneFields) createInput() servicepkg.CreateDNSZoneInput {
	return servicepkg.CreateDNSZoneInput{ActorUserID: f.ActorUserID, Name: f.ZoneName}
}

type updateDNSZoneFields struct {
	ActorUserID string `json:"actorUserId"`
	ZoneName    string `json:"zoneName"`
}

func (f updateDNSZoneFields) updateInput(status string) servicepkg.UpdateDNSZoneInput {
	return servicepkg.UpdateDNSZoneInput{ActorUserID: f.ActorUserID, Name: f.ZoneName, Status: status}
}

type createDNSZoneRequest struct {
	createDNSZoneFields
}

func (r createDNSZoneRequest) toInput() servicepkg.CreateDNSZoneInput {
	return r.createDNSZoneFields.createInput()
}

type updateDNSZoneRequest struct {
	updateDNSZoneFields
	Status string `json:"status"`
}

func (r updateDNSZoneRequest) toInput() servicepkg.UpdateDNSZoneInput {
	return r.updateDNSZoneFields.updateInput(r.Status)
}

type dnsRecordFields struct {
	ActorUserID    string `json:"actorUserId"`
	ZoneID         string `json:"zoneId"`
	Name           string `json:"name"`
	RecordType     string `json:"recordType"`
	TargetDeviceID string `json:"targetDeviceId"`
	CNAME          string `json:"cname"`
	Port           string `json:"port"`
	TTL            int    `json:"ttl"`
}

func (f dnsRecordFields) value() string {
	return dnsRecordValue(f.TargetDeviceID, f.CNAME)
}

func (f dnsRecordFields) createInput() servicepkg.CreateDNSRecordInput {
	return servicepkg.CreateDNSRecordInput{
		ActorUserID: f.ActorUserID,
		ZoneID:      f.ZoneID,
		Name:        f.Name,
		Type:        f.RecordType,
		Value:       f.value(),
		Port:        f.Port,
		TTL:         f.TTL,
	}
}

func (f dnsRecordFields) updateInput() servicepkg.UpdateDNSRecordInput {
	return servicepkg.UpdateDNSRecordInput{
		ActorUserID: f.ActorUserID,
		ZoneID:      f.ZoneID,
		Name:        f.Name,
		Type:        f.RecordType,
		Value:       f.value(),
		Port:        f.Port,
		TTL:         f.TTL,
	}
}

type createDNSRecordRequest struct {
	dnsRecordFields
}

func (r createDNSRecordRequest) toInput() servicepkg.CreateDNSRecordInput {
	return r.dnsRecordFields.createInput()
}

type updateDNSRecordRequest struct {
	dnsRecordFields
}

func (r updateDNSRecordRequest) toInput() servicepkg.UpdateDNSRecordInput {
	return r.dnsRecordFields.updateInput()
}

type securityGroupFields struct {
	NetworkID   string `json:"networkId"`
	ActorUserID string `json:"actorUserId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (f securityGroupFields) createInput() servicepkg.CreateSecurityGroupInput {
	return servicepkg.CreateSecurityGroupInput{
		NetworkID:   f.NetworkID,
		ActorUserID: f.ActorUserID,
		Name:        f.Name,
		Description: f.Description,
	}
}

type createSecurityGroupRequest struct {
	securityGroupFields
}

func (r createSecurityGroupRequest) toInput() servicepkg.CreateSecurityGroupInput {
	return r.securityGroupFields.createInput()
}

type updateSecurityGroupRequest struct {
	SecurityGroupID string `json:"securityGroupId"`
	securityGroupFields
}

func (r updateSecurityGroupRequest) toInput() servicepkg.UpdateSecurityGroupInput {
	return servicepkg.UpdateSecurityGroupInput{
		SecurityGroupID: r.SecurityGroupID,
		ActorUserID:     r.ActorUserID,
		Name:            r.Name,
		Description:     r.Description,
	}
}

type securityRuleFieldsRequest struct {
	ActorUserID  string `json:"actorUserId"`
	Direction    string `json:"direction"`
	Priority     int    `json:"priority"`
	Action       string `json:"action"`
	Protocol     string `json:"protocol"`
	PortFrom     int    `json:"portFrom"`
	PortTo       int    `json:"portTo"`
	PeerType     string `json:"peerType"`
	PeerValue    string `json:"peerValue"`
	SubjectType  string `json:"subjectType"`
	SubjectValue string `json:"subjectValue"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
}

func (f securityRuleFieldsRequest) fields() (string, string, string) {
	return securityRuleFields(
		f.PortFrom,
		f.PortTo,
		firstNonEmpty(f.PeerType, f.SubjectType),
		firstNonEmpty(f.PeerValue, f.SubjectValue),
	)
}

func (f securityRuleFieldsRequest) createInput() servicepkg.CreateSecurityRuleInput {
	portRange, peerType, peerValue := f.fields()
	return servicepkg.CreateSecurityRuleInput{
		ActorUserID: f.ActorUserID,
		Direction:   f.Direction,
		Action:      f.Action,
		Protocol:    f.Protocol,
		PortRange:   portRange,
		PeerType:    peerType,
		PeerValue:   peerValue,
		Priority:    f.Priority,
		Description: f.Description,
		Enabled:     f.Enabled,
	}
}

type updateSecurityRuleFieldsRequest struct {
	ActorUserID  string `json:"actorUserId"`
	Direction    string `json:"direction"`
	Priority     *int   `json:"priority,omitempty"`
	Action       string `json:"action"`
	Protocol     string `json:"protocol"`
	PortFrom     int    `json:"portFrom"`
	PortTo       int    `json:"portTo"`
	PeerType     string `json:"peerType"`
	PeerValue    string `json:"peerValue"`
	SubjectType  string `json:"subjectType"`
	SubjectValue string `json:"subjectValue"`
	Description  string `json:"description"`
	Enabled      *bool  `json:"enabled,omitempty"`
}

func (f updateSecurityRuleFieldsRequest) fields() (string, string, string) {
	return securityRuleFields(
		f.PortFrom,
		f.PortTo,
		firstNonEmpty(f.PeerType, f.SubjectType),
		firstNonEmpty(f.PeerValue, f.SubjectValue),
	)
}

func (f updateSecurityRuleFieldsRequest) updateInput() servicepkg.UpdateSecurityRuleInput {
	portRange, peerType, peerValue := f.fields()
	return servicepkg.UpdateSecurityRuleInput{
		ActorUserID: f.ActorUserID,
		Direction:   f.Direction,
		Action:      f.Action,
		Protocol:    f.Protocol,
		PortRange:   portRange,
		PeerType:    peerType,
		PeerValue:   peerValue,
		Priority:    f.Priority,
		Description: f.Description,
		Enabled:     f.Enabled,
	}
}

type createSecurityRuleRequest struct {
	securityRuleFieldsRequest
}

func (r createSecurityRuleRequest) toInput() servicepkg.CreateSecurityRuleInput {
	return r.securityRuleFieldsRequest.createInput()
}

type updateSecurityRuleRequest struct {
	updateSecurityRuleFieldsRequest
}

func (r updateSecurityRuleRequest) toInput() servicepkg.UpdateSecurityRuleInput {
	return r.updateSecurityRuleFieldsRequest.updateInput()
}

func dnsRecordValue(targetDeviceID, cname string) string {
	return firstNonEmpty(targetDeviceID, cname)
}

func securityRuleFields(portFrom, portTo int, peerType, peerValue string) (string, string, string) {
	peerType = strings.TrimSpace(peerType)
	peerValue = strings.TrimSpace(peerValue)
	return formatPortRange(portFrom, portTo), peerType, peerValue
}

func formatPortRange(portFrom, portTo int) string {
	if portFrom == 0 && portTo == 0 {
		return "all"
	}
	if portTo == 0 || portTo == portFrom {
		return strconv.Itoa(portFrom)
	}
	return strconv.Itoa(portFrom) + "," + strconv.Itoa(portTo)
}
