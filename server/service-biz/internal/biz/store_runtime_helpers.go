package biz

func (s *Store) peerRuntimeActiveLocked(deviceID string, now int64) bool {
	status := s.runtimeStatuses[deviceID]
	if !status.DeviceEnabled || !status.NetworkEnabled {
		return false
	}
	lastSeen := status.LastSeenAt
	if lastSeen == 0 {
		lastSeen = status.LastReportAt
	}
	return lastSeen > 0 && now-lastSeen <= int64(activePeerTTL.Seconds())
}
