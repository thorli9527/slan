package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
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
