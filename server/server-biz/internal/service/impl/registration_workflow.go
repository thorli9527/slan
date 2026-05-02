package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/mqttauth"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s dbDeviceService) requireRegisterDeviceRequest(req dto.RegisterDeviceRequest) error {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Platform) == "" || strings.TrimSpace(req.PublicKey) == "" {
		return fmt.Errorf("%w: name, platform, and publicKey are required", ErrInvalidArgument)
	}
	if deviceID := strings.TrimSpace(req.DeviceID); deviceID != "" && !usableClientDeviceID(deviceID) {
		return fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument)
	}
	return nil
}

func (s dbNodeService) requireRegisterNodeRequest(req dto.RegisterNodeRequest) error {
	if strings.TrimSpace(req.DeviceID) == "" || strings.TrimSpace(req.NodeID) == "" || strings.TrimSpace(req.NodePublicKey) == "" {
		return fmt.Errorf("%w: deviceId, nodeId, and nodePublicKey are required", ErrInvalidArgument)
	}
	return nil
}

func (s *dbState) requireUser(ctx context.Context, userID string) error {
	if _, err := s.pg.GetUserByID(ctx, userID); err != nil {
		if repo.IsNotFound(err) {
			return ErrUnauthorized
		}
		return err
	}
	return nil
}

func (s dbDeviceService) upsertDeviceRecord(ctx context.Context, userID string, req dto.RegisterDeviceRequest) (repo.Device, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		deviceID = util.NewID("dev")
	}
	record := repo.Device{
		DeviceID:      deviceID,
		UserID:        userID,
		Name:          req.Name,
		Platform:      req.Platform,
		DeviceVersion: strings.TrimSpace(req.DeviceVersion),
		CountryCode:   normalizeRelayCountryCode(req.CountryCode),
		Status:        "offline",
		PublicKey:     &req.PublicKey,
		CreatedAt:     time.Now().Unix(),
	}
	if strings.TrimSpace(req.DeviceID) != "" {
		current, err := s.state.pg.GetDeviceByID(ctx, deviceID)
		if err == nil {
			if current.UserID != userID {
				return repo.Device{}, ErrForbidden
			}
			record.CreatedAt = current.CreatedAt
			if err := s.state.pg.UpdateDevice(ctx, record); err != nil {
				return repo.Device{}, err
			}
			return s.state.pg.GetDeviceByID(ctx, deviceID)
		}
		if !repo.IsNotFound(err) {
			return repo.Device{}, err
		}
		if err := s.state.pg.InsertDevice(ctx, record); err != nil {
			if !repo.IsUniqueViolation(err) {
				return repo.Device{}, err
			}
			current, err := s.state.pg.GetDeviceByID(ctx, deviceID)
			if err != nil {
				return repo.Device{}, err
			}
			if current.UserID != userID {
				return repo.Device{}, ErrForbidden
			}
			return current, nil
		}
		return record, nil
	}
	if err := s.state.pg.InsertDevice(ctx, record); err != nil {
		if !repo.IsUniqueViolation(err) {
			return repo.Device{}, err
		}
		current, err := s.state.pg.GetDeviceByID(ctx, deviceID)
		if err != nil {
			return repo.Device{}, err
		}
		if current.UserID != userID {
			return repo.Device{}, ErrForbidden
		}
		return current, nil
	}
	return record, nil
}

func usableClientDeviceID(deviceID string) bool {
	value := strings.TrimSpace(deviceID)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if lower == "authcallbackid" || lower == "windows-plugin-login" || lower == "macos-plugin-login" {
		return false
	}
	return !strings.HasPrefix(lower, "cb-")
}

func (s *dbState) buildDeviceDTO(ctx context.Context, record repo.Device) dto.Device {
	user, err := s.pg.GetUserByID(ctx, record.UserID)
	if err != nil {
		return record.ToDTO(nil)
	}
	return s.buildDeviceDTOForNetwork(ctx, record, strings.TrimSpace(user.ActiveNetworkID))
}

func (s *dbState) preferredDeviceListNetworkID(ctx context.Context, userID string, networks []repo.Network) string {
	if user, err := s.pg.GetUserByID(ctx, userID); err == nil && strings.TrimSpace(user.ActiveNetworkID) != "" {
		return strings.TrimSpace(user.ActiveNetworkID)
	}
	for _, network := range networks {
		if network.OwnerUserID == userID {
			return network.NetworkID
		}
	}
	if len(networks) > 0 {
		return networks[0].NetworkID
	}
	return ""
}

func (s *dbState) deviceListVisibleNetworks(ctx context.Context, userID string) ([]repo.Network, error) {
	seen := make(map[string]struct{})
	var out []repo.Network
	appendNetwork := func(network repo.Network) {
		if strings.TrimSpace(network.NetworkID) == "" {
			return
		}
		if _, ok := seen[network.NetworkID]; ok {
			return
		}
		seen[network.NetworkID] = struct{}{}
		out = append(out, network)
	}
	if user, err := s.pg.GetUserByID(ctx, userID); err == nil && strings.TrimSpace(user.ActiveNetworkID) != "" {
		if network, err := s.pg.GetNetworkByID(ctx, strings.TrimSpace(user.ActiveNetworkID)); err == nil {
			appendNetwork(network)
		}
	}
	if network, err := s.pg.GetOwnedNetworkByUser(ctx, userID); err == nil {
		appendNetwork(network)
	}
	networks, err := s.pg.ListVisibleNetworksByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, network := range networks {
		appendNetwork(network)
	}
	return out, nil
}

func (s *dbState) firstDeviceMembershipNetwork(ctx context.Context, deviceID string, networkIDs []string) string {
	for _, networkID := range networkIDs {
		if _, err := s.pg.GetMemberByNetworkDevice(ctx, networkID, deviceID); err == nil {
			return networkID
		}
	}
	return ""
}

func (s *dbState) deviceListVisibleNetworkForDevice(ctx context.Context, userID, deviceID string, networks []repo.Network) (string, bool) {
	for _, network := range networks {
		member, err := s.pg.GetMemberByNetworkDevice(ctx, network.NetworkID, deviceID)
		if err != nil {
			if repo.IsNotFound(err) {
				continue
			}
			return "", false
		}
		if network.OwnerUserID == userID || member.Status == "active" {
			return network.NetworkID, true
		}
	}
	return "", false
}

func (s *dbState) buildDeviceDTOForNetwork(ctx context.Context, record repo.Device, networkID string) dto.Device {
	networkIDs, _ := s.deviceNetworkIDs(ctx, record.DeviceID)
	device := record.ToDTO(networkIDs)
	user, err := s.pg.GetUserByID(ctx, record.UserID)
	if err != nil {
		return device
	}
	device.OwnerEmail = user.Email
	device.LinkStatus = record.Status
	activeNetworkID := strings.TrimSpace(networkID)
	if activeNetworkID == "" {
		return device
	}
	attachments, err := s.pg.ListAttachmentsByDevice(ctx, record.DeviceID)
	if err == nil {
		if repaired, repairErr := s.ensureDeviceAttachmentsVirtualIPs(ctx, attachments); repairErr == nil {
			attachments = repaired
		}
		for _, attachment := range attachments {
			if attachment.NetworkID == activeNetworkID {
				device.CurrentVirtualIP = attachment.VirtualIP
				break
			}
		}
	}
	if member, err := s.pg.GetMemberByNetworkDevice(ctx, activeNetworkID, record.DeviceID); err == nil {
		device.JoinedAt = member.CreatedAt
		device.MembershipStatus = member.Status
		device.NetworkRole = member.Role
	}
	hasFreshNetworkState := false
	if state, ok := s.loadFreshDeviceNetworkState(ctx, record.DeviceID, activeNetworkID); ok {
		hasFreshNetworkState = true
		stateDTO := state.ToDTO()
		device.NetworkState = &stateDTO
		if state.NetworkOnline {
			device.LinkStatus = "online"
		} else if state.ControlReachable {
			device.LinkStatus = "reachable"
		} else {
			device.LinkStatus = "offline"
		}
		device.CurrentVirtualIP = state.VirtualIP
	}
	if state, err := s.pg.GetLatestDeviceConnectionState(ctx, activeNetworkID, record.DeviceID); err == nil {
		if !hasFreshNetworkState && strings.TrimSpace(state.State) != "" {
			device.LinkStatus = state.State
		}
		device.ConnectivityProtocol = normalizeConnectivityProtocol(state.Path)
	}
	return device
}

func (s *dbState) buildDeviceMQTTCredential(record repo.Device) *dto.MQTTCredential {
	return mqttauth.DeviceCredential(s.cfg.MQTT, record.DeviceID, record.DeviceID, time.Now())
}

func (s dbNodeService) buildNodeDTO(ctx context.Context, req dto.RegisterNodeRequest) dto.Node {
	node := dto.Node{
		NodeID:        req.NodeID,
		DeviceID:      req.DeviceID,
		NodePublicKey: req.NodePublicKey,
		Capabilities:  append([]string(nil), req.Capabilities...),
	}
	node.NetworkIDs, _ = s.state.activeDeviceNetworkIDs(ctx, req.DeviceID)
	return node
}

func normalizeConnectivityProtocol(path string) string {
	value := strings.TrimSpace(path)
	switch value {
	case "":
		return ""
	case "derp":
		return "relay"
	default:
		return value
	}
}
