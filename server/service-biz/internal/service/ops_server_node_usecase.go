package service

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

const serverNodeDeploymentTimeout = 20 * time.Minute

type OpsServerNodeService struct {
	ServerNodes repository.ServerNodeRepository
	Nodes       repository.RuntimeNodeRepository
	Audit       repository.AuditRepository
	Cipher      SecretCipher
	Deployer    ServerNodeDeployer
	NewNodeID   func() string
	Now         func() time.Time
}

func (s OpsServerNodeService) ListServerNodes(ctx context.Context) ([]OpsServerNodeView, error) {
	items, err := s.ServerNodes.ListServerNodes(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]OpsServerNodeView, 0, len(items))
	for _, item := range items {
		item = serverNodeDefaults(item)
		if item.DeployStatus == "deploying" && currentTime(s.Now).Unix()-item.UpdatedAt >= int64(serverNodeDeploymentTimeout/time.Second) {
			item.DeployStatus = "failed"
			item.LastDeployError = "部署任务超时或服务重启，请重新部署"
			item.UpdatedAt = currentTime(s.Now).Unix()
			_ = s.ServerNodes.SaveServerNode(ctx, item)
		}
		views = append(views, serverNodeView(item))
	}
	return views, nil
}

func (s OpsServerNodeService) UpsertServerNode(ctx context.Context, input UpsertServerNodeInput) (OpsServerNodeView, error) {
	input.Name, input.Host, input.SSHUsername = strings.TrimSpace(input.Name), strings.TrimSpace(input.Host), strings.TrimSpace(input.SSHUsername)
	if input.Name == "" || net.ParseIP(input.Host) == nil || input.SSHUsername == "" || input.SSHUsername != "root" {
		return OpsServerNodeView{}, fmt.Errorf("%w: name, IPv4/IPv6 host and root SSH user are required", ErrInvalidArgument)
	}
	if !validServerNodePorts(input) {
		return OpsServerNodeView{}, fmt.Errorf("%w: invalid or duplicate service port", ErrInvalidArgument)
	}
	now := currentTime(s.Now).Unix()
	item := model.ServerNode{NodeID: strings.TrimSpace(input.NodeID)}
	var previous model.ServerNode
	if item.NodeID != "" {
		stored, ok, err := s.ServerNodes.GetServerNode(ctx, item.NodeID)
		if err != nil {
			return OpsServerNodeView{}, err
		}
		if !ok {
			return OpsServerNodeView{}, ErrNotFound
		}
		item = stored
		item = serverNodeDefaults(item)
		previous = item
		if input.Host != item.Host && item.SSHHostKeyFingerprint != "" {
			if strings.TrimSpace(input.SSHPassword) == "" {
				return OpsServerNodeView{}, fmt.Errorf("%w: changing a trusted host requires the SSH password", ErrInvalidArgument)
			}
			item.SSHHostKeyFingerprint = ""
		}
	} else {
		if strings.TrimSpace(input.SSHPassword) == "" {
			return OpsServerNodeView{}, fmt.Errorf("%w: SSH password is required", ErrInvalidArgument)
		}
		item.NodeID = s.NewNodeID()
		item.RelayNodeID, item.RelayTCPNodeID, item.PunchNodeID = "relay"+item.NodeID, "derp"+item.NodeID, "punch"+item.NodeID
		item.DeployStatus, item.CreatedAt = "not_deployed", now
	}
	if input.SSHPassword != "" {
		ciphertext, err := s.Cipher.Encrypt(input.SSHPassword)
		if err != nil {
			return OpsServerNodeView{}, err
		}
		item.SSHPasswordCiphertext = ciphertext
	}
	item.Name, item.Host, item.SSHPort, item.SSHUsername = input.Name, input.Host, input.SSHPort, input.SSHUsername
	item.RelayUDPPort, item.RelayAdminPort, item.RelayTCPPort = input.RelayUDPPort, input.RelayAdminPort, input.RelayTCPPort
	item.PunchUDPPort, item.PunchHTTPPort = input.PunchUDPPort, input.PunchHTTPPort
	item.APIProxyPort, item.MQTTProxyPort, item.UpdatedAt = input.APIProxyPort, input.MQTTProxyPort, now
	item.RelayEnabled = input.RelayEnabled
	item.PunchEnabled = input.PunchEnabled
	item.ProxyEnabled = input.ProxyEnabled
	if previous.NodeID != "" && !serverNodeDeploymentConfigEqual(previous, item) {
		item.DeployStatus = "not_deployed"
		item.LastDeployError = ""
	}
	if err := s.ensureUniqueHost(ctx, item); err != nil {
		return OpsServerNodeView{}, err
	}
	if err := s.ServerNodes.SaveServerNode(ctx, item); err != nil {
		return OpsServerNodeView{}, err
	}
	return serverNodeView(item), nil
}

func serverNodeDeploymentConfigEqual(left, right model.ServerNode) bool {
	return left.Host == right.Host && left.RelayUDPPort == right.RelayUDPPort && left.RelayAdminPort == right.RelayAdminPort && left.RelayTCPPort == right.RelayTCPPort &&
		left.PunchUDPPort == right.PunchUDPPort && left.PunchHTTPPort == right.PunchHTTPPort &&
		left.APIProxyPort == right.APIProxyPort && left.MQTTProxyPort == right.MQTTProxyPort &&
		left.RelayEnabled == right.RelayEnabled && left.PunchEnabled == right.PunchEnabled &&
		left.ProxyEnabled == right.ProxyEnabled
}

func serverNodeDefaults(item model.ServerNode) model.ServerNode {
	if item.APIProxyPort == 0 {
		item.APIProxyPort = 28080
	}
	if item.MQTTProxyPort == 0 {
		item.MQTTProxyPort = 1883
	}
	if item.RelayTCPPort == 0 {
		item.RelayTCPPort = 29120
	}
	if item.RelayTCPNodeID == "" && item.NodeID != "" {
		item.RelayTCPNodeID = "derp" + item.NodeID
	}
	return item
}

func (s OpsServerNodeService) InspectServerNodeHostKey(ctx context.Context, nodeID string) (ServerNodeHostKeyView, error) {
	item, password, err := s.serverNodeDeploymentCredentials(ctx, nodeID)
	if err != nil {
		return ServerNodeHostKeyView{}, err
	}
	fingerprint, err := s.Deployer.ProbeHostKey(ctx, item, password)
	if err != nil {
		return ServerNodeHostKeyView{}, err
	}
	return ServerNodeHostKeyView{Fingerprint: fingerprint}, nil
}

func (s OpsServerNodeService) DeployServerNode(ctx context.Context, nodeID, confirmedHostKey string) (OpsServerNodeView, error) {
	item, password, err := s.serverNodeDeploymentCredentials(ctx, nodeID)
	if err != nil {
		return OpsServerNodeView{}, err
	}
	confirmedHostKey = strings.TrimSpace(confirmedHostKey)
	if item.SSHHostKeyFingerprint == "" {
		if confirmedHostKey == "" {
			return OpsServerNodeView{}, fmt.Errorf("%w: SSH host key confirmation is required", ErrInvalidArgument)
		}
		observed, probeErr := s.Deployer.ProbeHostKey(ctx, item, password)
		if probeErr != nil {
			return OpsServerNodeView{}, probeErr
		}
		if observed != confirmedHostKey {
			return OpsServerNodeView{}, fmt.Errorf("%w: SSH host key changed before deployment", ErrConflict)
		}
		item.SSHHostKeyFingerprint = observed
	}
	if item.DeployStatus == "deploying" && currentTime(s.Now).Unix()-item.UpdatedAt < int64(serverNodeDeploymentTimeout/time.Second) {
		return OpsServerNodeView{}, fmt.Errorf("%w: deployment is already running", ErrConflict)
	}
	item.DeployStatus, item.LastDeployError, item.UpdatedAt = "deploying", "", currentTime(s.Now).Unix()
	if err := s.ServerNodes.SaveServerNode(ctx, item); err != nil {
		return OpsServerNodeView{}, err
	}
	result, deployErr := s.Deployer.Deploy(ctx, item, password)
	now := currentTime(s.Now).Unix()
	item.UpdatedAt = now
	if deployErr != nil {
		item.DeployStatus, item.LastDeployError = "failed", truncateDeployError(deployErr.Error())
		_ = s.ServerNodes.SaveServerNode(context.Background(), item)
		s.recordDeployAudit(ctx, item, "failure")
		return serverNodeView(item), deployErr
	}
	item.DeployStatus, item.LastDeployError, item.LastDeployedAt = "succeeded", "", now
	if item.SSHHostKeyFingerprint == "" {
		item.SSHHostKeyFingerprint = result.HostKeyFingerprint
	}
	if err := s.ServerNodes.SaveServerNode(ctx, item); err != nil {
		return OpsServerNodeView{}, err
	}
	if err := s.registerDeployedNodes(ctx, item, now); err != nil {
		return OpsServerNodeView{}, err
	}
	s.recordDeployAudit(ctx, item, "success")
	return serverNodeView(item), nil
}

func (s OpsServerNodeService) serverNodeDeploymentCredentials(ctx context.Context, nodeID string) (model.ServerNode, string, error) {
	item, ok, err := s.ServerNodes.GetServerNode(ctx, strings.TrimSpace(nodeID))
	if err != nil {
		return model.ServerNode{}, "", err
	}
	if !ok {
		return model.ServerNode{}, "", ErrNotFound
	}
	item = serverNodeDefaults(item)
	item, changed, err := s.adoptExistingRuntimeNodeIDs(ctx, item)
	if err != nil {
		return model.ServerNode{}, "", err
	}
	if changed {
		item.UpdatedAt = currentTime(s.Now).Unix()
		if err := s.ServerNodes.SaveServerNode(ctx, item); err != nil {
			return model.ServerNode{}, "", err
		}
	}
	password, err := s.Cipher.Decrypt(item.SSHPasswordCiphertext)
	if err != nil {
		return model.ServerNode{}, "", err
	}
	return item, password, nil
}

func (s OpsServerNodeService) adoptExistingRuntimeNodeIDs(ctx context.Context, item model.ServerNode) (model.ServerNode, bool, error) {
	changed := false
	relays, err := s.Nodes.ListRelayNodes(ctx)
	if err != nil {
		return item, false, err
	}
	udpEndpoint := normalizeNodeEndpoint(fmt.Sprintf("%s:%d", item.Host, item.RelayUDPPort))
	tcpEndpoint := normalizeNodeEndpoint(fmt.Sprintf("%s:%d", item.Host, item.RelayTCPPort))
	for _, runtimeNode := range relays {
		endpoint := normalizeNodeEndpoint(runtimeNode.Endpoint)
		switch {
		case runtimeNode.Transport == relayTransportUDP && endpoint == udpEndpoint && item.RelayNodeID != runtimeNode.NodeID:
			item.RelayNodeID, changed = runtimeNode.NodeID, true
		case runtimeNode.Transport == relayTransportDerpTLS && endpoint == tcpEndpoint && item.RelayTCPNodeID != runtimeNode.NodeID:
			item.RelayTCPNodeID, changed = runtimeNode.NodeID, true
		}
	}
	punchNodes, err := s.Nodes.ListPunchNodes(ctx)
	if err != nil {
		return item, false, err
	}
	punchEndpoint := normalizeNodeEndpoint(fmt.Sprintf("%s:%d", item.Host, item.PunchUDPPort))
	for _, runtimeNode := range punchNodes {
		if normalizeNodeEndpoint(runtimeNode.Endpoint) == punchEndpoint && item.PunchNodeID != runtimeNode.NodeID {
			item.PunchNodeID, changed = runtimeNode.NodeID, true
		}
	}
	return item, changed, nil
}

func (s OpsServerNodeService) recordDeployAudit(ctx context.Context, item model.ServerNode, status string) {
	if s.Audit == nil {
		return
	}
	eventID, err := randomHex(12)
	if err != nil {
		return
	}
	_ = s.Audit.SaveAuditEvent(context.Background(), model.AuditEvent{
		EventID: "audit_" + eventID, ActorType: "operator", ActorID: AuthenticatedOperatorID(ctx),
		Action: "deploy_node_services", ResourceType: "server_node", ResourceID: item.NodeID,
		Status: status, Detail: "host=" + item.Host, CreatedAt: currentTime(s.Now).Unix(),
	})
}

func (s OpsServerNodeService) DeleteServerNode(ctx context.Context, nodeID string) error {
	item, ok, err := s.ServerNodes.GetServerNode(ctx, strings.TrimSpace(nodeID))
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if item.DeployStatus == "deploying" {
		return fmt.Errorf("%w: deployment is running", ErrConflict)
	}
	item = serverNodeDefaults(item)
	if err := s.Nodes.DeleteRelayNode(ctx, item.RelayNodeID); err != nil {
		return err
	}
	if err := s.Nodes.DeleteRelayNode(ctx, item.RelayTCPNodeID); err != nil {
		return err
	}
	if err := s.Nodes.DeletePunchNode(ctx, item.PunchNodeID); err != nil {
		return err
	}
	return s.ServerNodes.DeleteServerNode(ctx, item.NodeID)
}

func (s OpsServerNodeService) ensureUniqueHost(ctx context.Context, current model.ServerNode) error {
	items, err := s.ServerNodes.ListServerNodes(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.NodeID != current.NodeID && item.Host == current.Host {
			return fmt.Errorf("%w: server host already exists", ErrConflict)
		}
	}
	return nil
}

func (s OpsServerNodeService) registerDeployedNodes(ctx context.Context, item model.ServerNode, now int64) error {
	relayStatus, relayHealth := "disabled", "down"
	if item.RelayEnabled {
		relayStatus, relayHealth = "active", "warning"
	}
	relay := model.RelayNode{NodeID: item.RelayNodeID, Name: item.Name + " UDP Relay", Region: "managed", Endpoint: fmt.Sprintf("udp://%s:%d", item.Host, item.RelayUDPPort), Transport: "relay_udp", Priority: 100, MaxBandwidthMbps: 1000, MonthlyTrafficGB: 10240, MaxSessions: 5000, Status: relayStatus, Health: relayHealth, CreatedAt: now, UpdatedAt: now}
	if existing, ok, err := s.Nodes.GetRelayNode(ctx, relay.NodeID); err != nil {
		return err
	} else if ok {
		relay.CreatedAt = existing.CreatedAt
	}
	if err := s.Nodes.SaveRelayNode(ctx, relay); err != nil {
		return err
	}
	punchStatus, punchHealth := "disabled", "down"
	if item.PunchEnabled {
		punchStatus, punchHealth = "active", "healthy"
	}
	punch := model.PunchNode{NodeID: item.PunchNodeID, Name: item.Name + " UDP Punch", Endpoint: fmt.Sprintf("%s:%d", item.Host, item.PunchUDPPort), MaxSessions: 10000, Status: punchStatus, Health: punchHealth, Priority: 100, CreatedAt: now, UpdatedAt: now}
	if existing, ok, err := s.Nodes.GetPunchNode(ctx, punch.NodeID); err != nil {
		return err
	} else if ok {
		punch.CreatedAt = existing.CreatedAt
	}
	return s.Nodes.SavePunchNode(ctx, punch)
}

func serverNodeView(item model.ServerNode) OpsServerNodeView {
	return OpsServerNodeView{NodeID: item.NodeID, Name: item.Name, Host: item.Host, SSHPort: item.SSHPort, SSHUsername: item.SSHUsername, SSHPasswordConfigured: item.SSHPasswordCiphertext != "", SSHHostKeyFingerprint: item.SSHHostKeyFingerprint, RelayUDPPort: item.RelayUDPPort, RelayAdminPort: item.RelayAdminPort, RelayTCPPort: item.RelayTCPPort, PunchUDPPort: item.PunchUDPPort, PunchHTTPPort: item.PunchHTTPPort, APIProxyPort: item.APIProxyPort, MQTTProxyPort: item.MQTTProxyPort, APIProxyURL: "http://" + net.JoinHostPort(item.Host, fmt.Sprint(item.APIProxyPort)), MQTTProxyURL: "mqtt://" + net.JoinHostPort(item.Host, fmt.Sprint(item.MQTTProxyPort)), RelayEnabled: item.RelayEnabled, PunchEnabled: item.PunchEnabled, ProxyEnabled: item.ProxyEnabled, RelayNodeID: item.RelayNodeID, RelayTCPNodeID: item.RelayTCPNodeID, PunchNodeID: item.PunchNodeID, DeployStatus: item.DeployStatus, LastDeployError: item.LastDeployError, LastDeployedAt: item.LastDeployedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func validServerNodePorts(input UpsertServerNodeInput) bool {
	if input.RelayTCPPort >= 65535 {
		return false
	}
	ports := []int{input.SSHPort, input.RelayUDPPort, input.RelayAdminPort, input.RelayTCPPort, input.RelayTCPPort + 1, input.PunchUDPPort, input.PunchHTTPPort, input.APIProxyPort, input.MQTTProxyPort}
	seen := map[int]bool{}
	for _, port := range ports {
		if port < 1 || port > 65535 || seen[port] {
			return false
		}
		seen[port] = true
	}
	return true
}

func truncateDeployError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1000 {
		value = value[:1000]
	}
	return value
}
