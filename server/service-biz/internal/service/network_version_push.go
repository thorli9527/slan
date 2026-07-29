package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type NetworkVersionPushTracker struct {
	mu       sync.Mutex
	versions map[string]int64
}

func NewNetworkVersionPushTracker() *NetworkVersionPushTracker {
	return &NetworkVersionPushTracker{versions: make(map[string]int64)}
}

func (t *NetworkVersionPushTracker) alreadyPublished(networkID string, version int64) bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.versions[networkID] == version
}

func (t *NetworkVersionPushTracker) markPublished(networkID string, version int64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.versions[networkID] = version
	t.mu.Unlock()
}

// NetworkVersionPushResult describes one periodic version-heartbeat scan.
type NetworkVersionPushResult struct {
	ScannedNetworks   int
	EligibleNetworks  int
	PublishedNetworks int
	SkippedUnchanged  int
}

// PushNetworkVersionHeartbeats publishes only the current version. Clients
// with the same version ignore it; clients behind the server fetch a snapshot.
func (s NetworkCoreService) PushNetworkVersionHeartbeats(ctx context.Context) (NetworkVersionPushResult, error) {
	result := NetworkVersionPushResult{}
	if s.Users == nil || s.Networks == nil || s.EventPublisher == nil {
		return result, nil
	}
	users, err := s.Users.ListUsers(ctx)
	if err != nil {
		return result, err
	}
	seen := make(map[string]struct{})
	var pushErrors []error
	for _, user := range users {
		networks, listErr := s.Networks.ListNetworksByOwner(ctx, strings.TrimSpace(user.UserID))
		if listErr != nil {
			pushErrors = append(pushErrors, listErr)
			continue
		}
		for _, network := range networks {
			networkID := strings.TrimSpace(network.NetworkID)
			if networkID == "" {
				continue
			}
			if _, ok := seen[networkID]; ok {
				continue
			}
			seen[networkID] = struct{}{}
			result.ScannedNetworks++
			members, memberErr := s.Networks.ListNetworkDevices(ctx, networkID)
			if memberErr != nil {
				pushErrors = append(pushErrors, fmt.Errorf("list network %s devices: %w", networkID, memberErr))
				continue
			}
			activeDeviceIDs := make(map[string]struct{})
			for _, member := range members {
				deviceID := strings.TrimSpace(member.DeviceID)
				if deviceID != "" && networkMemberActive(member) {
					activeDeviceIDs[deviceID] = struct{}{}
				}
			}
			if len(activeDeviceIDs) < 2 {
				continue
			}
			version, ok, versionErr := s.Networks.GetNetworkVersion(ctx, networkID)
			if versionErr != nil {
				pushErrors = append(pushErrors, fmt.Errorf("load network %s version: %w", networkID, versionErr))
				continue
			}
			if !ok || version.Version <= 0 {
				continue
			}
			result.EligibleNetworks++
			if s.VersionPushTracker.alreadyPublished(networkID, version.Version) {
				result.SkippedUnchanged++
				continue
			}
			now := networkNow(s.Now)
			if publishErr := publishNetworkEvent(
				ctx,
				s.EventPublisher,
				NetworkEventVersion,
				networkID,
				uint64(version.Version),
				now.UnixMilli(),
				map[string]any{"configVersion": version.Version},
			); publishErr != nil {
				pushErrors = append(pushErrors, fmt.Errorf("publish network %s version: %w", networkID, publishErr))
				continue
			}
			s.VersionPushTracker.markPublished(networkID, version.Version)
			result.PublishedNetworks++
		}
	}
	return result, errors.Join(pushErrors...)
}
