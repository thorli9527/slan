package biz

import "strings"

func (s *Store) createDeviceSessionLocked(deviceID, userID string, configs []NetworkConfig, now int64) (DeviceSession, error) {
	deviceToken, err := secureTokenHex(32)
	if err != nil {
		return DeviceSession{}, err
	}
	refreshToken, err := secureTokenHex(32)
	if err != nil {
		return DeviceSession{}, err
	}
	session := DeviceSession{
		SessionID:            newCompactUUID(),
		DeviceID:             deviceID,
		UserID:               userID,
		DeviceToken:          "dt_" + deviceToken,
		DeviceTokenExpiresAt: now + int64(deviceSessionTTL.Seconds()),
		DeviceRefreshToken:   "drt_" + refreshToken,
		RegisteredAt:         now,
		LastRenewedAt:        now,
		ActiveNetworkIDs:     networkIDsFromConfigs(configs),
		State:                "active",
	}
	s.nextDeviceSessionSeq++
	s.deviceSessions[session.SessionID] = session
	s.deviceSessionByToken[session.DeviceToken] = session.SessionID
	return session, nil
}

func (s *Store) revokeDeviceSessionsLocked(deviceID, userID string) {
	for sessionID, session := range s.deviceSessions {
		if session.DeviceID != deviceID || session.UserID != userID || session.State != "active" {
			continue
		}
		session.State = "revoked"
		s.deviceSessions[sessionID] = session
		delete(s.deviceSessionByToken, session.DeviceToken)
	}
}

func (s *Store) removeDeviceFromUserNetworksLocked(deviceID, userID string) {
	for key, membership := range s.networkDevices {
		if membership.DeviceID == deviceID && membership.OwnerUserID == userID {
			delete(s.networkDevices, key)
		}
	}
}

func networkIDsFromConfigs(configs []NetworkConfig) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(configs))
	for _, config := range configs {
		networkID := strings.TrimSpace(config.NetworkID)
		if networkID == "" || seen[networkID] {
			continue
		}
		seen[networkID] = true
		out = append(out, networkID)
	}
	return out
}
