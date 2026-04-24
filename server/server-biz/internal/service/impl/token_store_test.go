package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/slan/server/server-biz/internal/repo"
	controlws "github.com/slan/server/server-biz/internal/ws"
)

type memoryTokenStore struct {
	mu sync.Mutex

	accessTokens         map[string]repo.AccessTokenSession
	refreshTokens        map[string]string
	opsTokens            map[string]string
	controlSessionTokens map[string]string
	authCallbackPayloads map[string][]byte
	authCallbackReceived map[string]int64
	networkRevisions     map[string]uint64
	connectPlanGate      map[string]struct{}
	peerCandidateGate    map[string]struct{}
}

func newMemoryTokenStore() *memoryTokenStore {
	return &memoryTokenStore{
		accessTokens:         make(map[string]repo.AccessTokenSession),
		refreshTokens:        make(map[string]string),
		opsTokens:            make(map[string]string),
		controlSessionTokens: make(map[string]string),
		authCallbackPayloads: make(map[string][]byte),
		authCallbackReceived: make(map[string]int64),
		networkRevisions:     make(map[string]uint64),
		connectPlanGate:      make(map[string]struct{}),
		peerCandidateGate:    make(map[string]struct{}),
	}
}

func (s *memoryTokenStore) StoreAccessToken(
	_ context.Context,
	token, userID, deviceID string,
	_ time.Duration,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessTokens[token] = repo.AccessTokenSession{
		UserID:   userID,
		DeviceID: deviceID,
		IssuedAt: time.Now().Unix(),
	}
	return nil
}

func (s *memoryTokenStore) DeleteAccessToken(_ context.Context, accessToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.accessTokens, accessToken)
	return nil
}

func (s *memoryTokenStore) StoreRefreshToken(_ context.Context, token, userID string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshTokens[token] = userID
	return nil
}

func (s *memoryTokenStore) StoreOpsAccessToken(_ context.Context, token, adminID string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opsTokens[token] = adminID
	return nil
}

func (s *memoryTokenStore) StoreControlSessionToken(_ context.Context, token, userID string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.controlSessionTokens[token] = userID
	return nil
}

func (s *memoryTokenStore) MarkAuthCallbackReceived(_ context.Context, callbackID string, receivedAt int64, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authCallbackReceived[callbackID] = receivedAt
	delete(s.authCallbackPayloads, callbackID)
	return nil
}

func (s *memoryTokenStore) AuthCallbackReceivedAt(_ context.Context, callbackID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authCallbackReceived[callbackID], nil
}

func (s *memoryTokenStore) StoreAuthCallbackPayload(_ context.Context, callbackID string, payload any, _ time.Duration) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authCallbackPayloads[callbackID] = encoded
	return nil
}

func (s *memoryTokenStore) LoadAuthCallbackPayload(_ context.Context, callbackID string, target any) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.authCallbackPayloads[callbackID]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(value, target); err != nil {
		return false, err
	}
	return true, nil
}

func (s *memoryTokenStore) Authenticate(_ context.Context, accessToken string) (repo.AccessTokenSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.accessTokens[accessToken]
	if !ok {
		return repo.AccessTokenSession{}, fmt.Errorf("token not found")
	}
	return session, nil
}

func (s *memoryTokenStore) AuthenticateControlSessionToken(_ context.Context, token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.controlSessionTokens[token]
	if !ok {
		return "", fmt.Errorf("token not found")
	}
	return userID, nil
}

func (s *memoryTokenStore) AuthenticateOpsAccessToken(_ context.Context, token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	adminID, ok := s.opsTokens[token]
	if !ok {
		return "", fmt.Errorf("token not found")
	}
	return adminID, nil
}

func (s *memoryTokenStore) PublishControlSyncEvent(_ context.Context, _ controlws.ControlSyncEvent) error {
	return nil
}

func (s *memoryTokenStore) SubscribeControlSyncEvents(_ context.Context, _ func(controlws.ControlSyncEvent)) error {
	return nil
}

func (s *memoryTokenStore) NextNetworkRevision(_ context.Context, networkID string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.networkRevisions[networkID]++
	return s.networkRevisions[networkID], nil
}

func (s *memoryTokenStore) CurrentNetworkRevision(_ context.Context, networkID string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.networkRevisions[networkID], nil
}

func (s *memoryTokenStore) AcquireConnectPlanRetry(_ context.Context, networkID, nodeID, peerNodeID string) (bool, error) {
	key := networkID + ":" + nodeID + ":" + peerNodeID
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.connectPlanGate[key]; exists {
		return false, nil
	}
	s.connectPlanGate[key] = struct{}{}
	return true, nil
}

func (s *memoryTokenStore) ResetConnectPlanRetry(_ context.Context, networkID, nodeID, peerNodeID string) error {
	key := networkID + ":" + nodeID + ":" + peerNodeID
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connectPlanGate, key)
	return nil
}

func (s *memoryTokenStore) AcquirePeerCandidateDelivery(_ context.Context, networkID, sourceNodeID, targetNodeID string, candidate controlws.PeerCandidate, _ time.Duration) (bool, error) {
	key := networkID + ":" + sourceNodeID + ":" + targetNodeID + ":" + candidate.CandidateType + ":" + candidate.Endpoint
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.peerCandidateGate[key]; exists {
		return false, nil
	}
	s.peerCandidateGate[key] = struct{}{}
	return true, nil
}
