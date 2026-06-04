package biz

import "time"

// RuntimeService 负责把设备、会话、网络配置转换为客户端运行时响应。
type RuntimeService struct {
	store BusinessStore
	mqtt  MQTTConfig
}

func (s RuntimeService) DeviceSessionResponse(device Device, session DeviceSession, configs []NetworkConfig, now time.Time) DeviceSessionResponse {
	mqtt := deviceMQTTCredential(s.mqtt, device.DeviceID, now)
	return DeviceSessionResponse{
		Device:           device,
		DeviceSession:    session,
		MQTT:             mqtt,
		NetworkConfigs:   ItemsResponse{Items: configs},
		RuntimeEndpoints: s.RuntimeEndpointsResponse(mqtt, configs, now),
	}
}

func (s RuntimeService) RuntimeEndpointsResponse(mqtt *MQTTCredential, configs []NetworkConfig, now time.Time) RuntimeEndpointsResponse {
	punchNodes := runtimePunchNodes(s.store.ActivePunchNodes())
	networks := make([]RuntimeNetworkEndpoint, 0, len(configs))
	relayCandidates := make([]RelayCandidate, 0)
	seen := make(map[string]struct{})
	for _, config := range configs {
		networkCandidates := dedupeRuntimeRelayCandidates(config.RelayCandidates)
		networks = append(networks, RuntimeNetworkEndpoint{
			NetworkID:       config.NetworkID,
			RelayCandidates: networkCandidates,
		})
		for _, candidate := range networkCandidates {
			key := relayCandidateRuntimeKey(candidate)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			relayCandidates = append(relayCandidates, candidate)
		}
	}
	return RuntimeEndpointsResponse{
		MQTT:            mqtt,
		PunchNodes:      punchNodes,
		RelayCandidates: relayCandidates,
		Networks:        networks,
		RefreshedAt:     now.Unix(),
	}
}
