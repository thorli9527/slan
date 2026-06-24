package service

import (
	"net"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func publicMappingView(item model.PublicMapping) PublicMappingView {
	internal := strings.TrimSpace(item.InternalIP)
	view := PublicMappingView{
		MappingID:    item.MappingID,
		NetworkID:    item.NetworkID,
		Name:         item.Name,
		PublicDomain: item.PublicDomain,
		SourceRecord: item.SourceRecord,
		DeviceID:     item.DeviceID,
		Protocol:     item.Protocol,
		InternalIP:   internal,
		InternalPort: item.InternalPort,
		ExternalPort: item.ExternalPort,
		AccessMode:   item.AccessMode,
		TLSMode:      item.TLSMode,
		TargetType:   "device",
		Status:       normalizePublicMappingStatus(item.Status),
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
	if view.DeviceID == "" && net.ParseIP(internal) == nil {
		view.DeviceID = internal
		view.InternalIP = ""
	}
	switch {
	case view.SourceRecord != "" && view.SourceRecord != view.DeviceID && view.SourceRecord != view.InternalIP:
		view.TargetType = "record"
	case view.DeviceID != "":
		view.TargetType = "device"
	case view.InternalIP != "":
		view.TargetType = "ip"
	}
	return view
}

func securityGroupView(item model.SecurityGroup) SecurityGroupView {
	return SecurityGroupView{
		SecurityGroupID: item.SecurityGroupID,
		NetworkID:       item.NetworkID,
		Name:            item.Name,
		Description:     item.Description,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func securityRuleView(item model.SecurityRule) SecurityRuleView {
	portFrom, portTo := parsePortRange(item.PortRange)
	peerType, peerValue := securityPeer(item.CIDR)
	return SecurityRuleView{
		RuleID:          item.RuleID,
		SecurityGroupID: item.SecurityGroupID,
		Direction:       item.Direction,
		Protocol:        item.Protocol,
		PortRange:       item.PortRange,
		CIDR:            item.CIDR,
		Action:          item.Action,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
		Priority:        item.Priority,
		PortFrom:        portFrom,
		PortTo:          portTo,
		PeerType:        peerType,
		PeerValue:       peerValue,
		Description:     item.Description,
		Enabled:         item.Enabled,
	}
}
