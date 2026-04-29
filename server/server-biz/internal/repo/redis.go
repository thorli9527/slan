package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
)

type RedisTokenStore struct {
	client *redis.Client
}

type AccessTokenSession struct {
	UserID   string `json:"userId"`
	DeviceID string `json:"deviceId,omitempty"`
	IssuedAt int64  `json:"issuedAt"`
}

const controlSyncChannel = "control_sync_events"
const networkRevisionKeyPrefix = "network_revision:"
const connectPlanRetryCountKeyPrefix = "connect_plan_retry_count:"
const connectPlanRetryGateKeyPrefix = "connect_plan_retry_gate:"
const peerCandidateDeliveryKeyPrefix = "peer_candidate_delivery:"
const authCallbackPayloadKeyPrefix = "auth_callback_payload:"
const consoleLoginKeyPrefix = "console_login_key:"
const deviceNetworkStateKeyPrefix = "device_network_state:"

func NewRedisTokenStore(client *redis.Client) *RedisTokenStore {
	return &RedisTokenStore{client: client}
}

func (s *RedisTokenStore) StoreAccessToken(
	ctx context.Context,
	token, userID, deviceID string,
	ttl time.Duration,
) error {
	payload, err := json.Marshal(AccessTokenSession{
		UserID:   userID,
		DeviceID: deviceID,
		IssuedAt: time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	return s.client.Set(ctx, "access_token:"+token, payload, ttl).Err()
}

func (s *RedisTokenStore) DeleteAccessToken(ctx context.Context, accessToken string) error {
	return s.client.Del(ctx, "access_token:"+accessToken).Err()
}

func (s *RedisTokenStore) StoreRefreshToken(ctx context.Context, token, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, "refresh_token:"+token, userID, ttl).Err()
}

func (s *RedisTokenStore) AuthenticateRefreshToken(ctx context.Context, token string) (string, error) {
	userID, err := s.client.Get(ctx, "refresh_token:"+token).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("token not found")
		}
		return "", err
	}
	return userID, nil
}

func (s *RedisTokenStore) DeleteRefreshToken(ctx context.Context, token string) error {
	return s.client.Del(ctx, "refresh_token:"+token).Err()
}

func (s *RedisTokenStore) StoreOpsAccessToken(ctx context.Context, token, adminID string, ttl time.Duration) error {
	return s.client.Set(ctx, "ops_access_token:"+token, adminID, ttl).Err()
}

func (s *RedisTokenStore) StoreControlSessionToken(ctx context.Context, token, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, "control_session_token:"+token, userID, ttl).Err()
}

func (s *RedisTokenStore) DeleteControlSessionToken(ctx context.Context, token string) error {
	return s.client.Del(ctx, "control_session_token:"+token).Err()
}

func (s *RedisTokenStore) StoreAuthCallbackPayload(ctx context.Context, callbackID string, payload any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, authCallbackPayloadKeyPrefix+callbackID, encoded, ttl).Err()
}

func (s *RedisTokenStore) LoadAuthCallbackPayload(ctx context.Context, callbackID string, target any) (bool, error) {
	value, err := s.client.Get(ctx, authCallbackPayloadKeyPrefix+callbackID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(value, target); err != nil {
		return false, err
	}
	return true, nil
}

func (s *RedisTokenStore) StoreConsoleLoginKey(ctx context.Context, loginKey string, payload any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, consoleLoginKeyPrefix+loginKey, encoded, ttl).Err()
}

func (s *RedisTokenStore) ConsumeConsoleLoginKey(ctx context.Context, loginKey string, target any) (bool, error) {
	value, err := s.client.GetDel(ctx, consoleLoginKeyPrefix+loginKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(value, target); err != nil {
		return false, err
	}
	return true, nil
}

func (s *RedisTokenStore) StoreDeviceNetworkState(ctx context.Context, state DeviceNetworkState, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 90 * time.Second
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, deviceNetworkStateKey(state.DeviceID, state.NetworkID), encoded, ttl).Err()
}

func (s *RedisTokenStore) LoadDeviceNetworkState(ctx context.Context, deviceID, networkID string) (DeviceNetworkState, bool, error) {
	value, err := s.client.Get(ctx, deviceNetworkStateKey(deviceID, networkID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return DeviceNetworkState{}, false, nil
		}
		return DeviceNetworkState{}, false, err
	}
	var state DeviceNetworkState
	if err := json.Unmarshal(value, &state); err != nil {
		return DeviceNetworkState{}, false, err
	}
	return state, true, nil
}

func (s *RedisTokenStore) ListDeviceNetworkStates(ctx context.Context) ([]DeviceNetworkState, error) {
	var cursor uint64
	var out []DeviceNetworkState
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, deviceNetworkStateKeyPrefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		cursor = nextCursor
		if len(keys) > 0 {
			values, err := s.client.MGet(ctx, keys...).Result()
			if err != nil {
				return nil, err
			}
			for _, value := range values {
				if value == nil {
					continue
				}
				var raw []byte
				switch typed := value.(type) {
				case string:
					raw = []byte(typed)
				case []byte:
					raw = typed
				default:
					raw = []byte(fmt.Sprint(typed))
				}
				var state DeviceNetworkState
				if err := json.Unmarshal(raw, &state); err != nil {
					log.Printf("redis device network state decode skipped: %v", err)
					continue
				}
				out = append(out, state)
			}
		}
		if cursor == 0 {
			break
		}
	}
	return out, nil
}

func (s *RedisTokenStore) DeleteDeviceNetworkState(ctx context.Context, deviceID, networkID string) error {
	return s.client.Del(ctx, deviceNetworkStateKey(deviceID, networkID)).Err()
}

func (s *RedisTokenStore) Authenticate(ctx context.Context, accessToken string) (AccessTokenSession, error) {
	value, err := s.client.Get(ctx, "access_token:"+accessToken).Bytes()
	if err != nil {
		if err == redis.Nil {
			return AccessTokenSession{}, fmt.Errorf("token not found")
		}
		return AccessTokenSession{}, err
	}
	return parseAccessTokenSession(value)
}

func parseAccessTokenSession(value []byte) (AccessTokenSession, error) {
	var session AccessTokenSession
	if err := json.Unmarshal(value, &session); err != nil {
		return AccessTokenSession{}, err
	}
	if session.UserID == "" {
		return AccessTokenSession{}, fmt.Errorf("token session missing user")
	}
	return session, nil
}

func (s *RedisTokenStore) AuthenticateControlSessionToken(ctx context.Context, token string) (string, error) {
	userID, err := s.client.Get(ctx, "control_session_token:"+token).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("token not found")
		}
		return "", err
	}
	return userID, nil
}

func (s *RedisTokenStore) AuthenticateOpsAccessToken(ctx context.Context, token string) (string, error) {
	adminID, err := s.client.Get(ctx, "ops_access_token:"+token).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("token not found")
		}
		return "", err
	}
	return adminID, nil
}

func (s *RedisTokenStore) DeleteOpsAccessToken(ctx context.Context, token string) error {
	return s.client.Del(ctx, "ops_access_token:"+token).Err()
}

func (s *RedisTokenStore) PublishControlSyncEvent(ctx context.Context, event controlmsg.ControlSyncEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return s.client.Publish(ctx, controlSyncChannel, payload).Err()
}

func (s *RedisTokenStore) SubscribeControlSyncEvents(ctx context.Context, handler func(controlmsg.ControlSyncEvent)) error {
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			pubsub := s.client.Subscribe(ctx, controlSyncChannel)
			if _, err := pubsub.Receive(ctx); err != nil {
				log.Printf("redis control-sync subscribe failed channel=%s err=%v", controlSyncChannel, err)
				_ = pubsub.Close()
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
					continue
				}
			}

			ch := pubsub.Channel()
			reconnect := false
			for !reconnect {
				select {
				case <-ctx.Done():
					_ = pubsub.Close()
					return
				case msg, ok := <-ch:
					if !ok {
						log.Printf("redis control-sync channel closed channel=%s", controlSyncChannel)
						reconnect = true
						break
					}
					var event controlmsg.ControlSyncEvent
					if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
						log.Printf("redis control-sync decode failed channel=%s err=%v", controlSyncChannel, err)
						continue
					}
					handler(event)
				}
			}
			_ = pubsub.Close()
			log.Printf("redis control-sync reconnecting channel=%s", controlSyncChannel)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()

	return nil
}

func (s *RedisTokenStore) NextNetworkRevision(ctx context.Context, networkID string) (uint64, error) {
	value, err := s.client.Incr(ctx, networkRevisionKeyPrefix+networkID).Uint64()
	if err != nil {
		return 0, err
	}
	return value, nil
}

func (s *RedisTokenStore) CurrentNetworkRevision(ctx context.Context, networkID string) (uint64, error) {
	value, err := s.client.Get(ctx, networkRevisionKeyPrefix+networkID).Uint64()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}
	return value, nil
}

func (s *RedisTokenStore) AcquireConnectPlanRetry(ctx context.Context, networkID, nodeID, peerNodeID string) (bool, error) {
	pairKey := connectPlanRetryPairKey(networkID, nodeID, peerNodeID)
	countKey := connectPlanRetryCountKeyPrefix + pairKey
	gateKey := connectPlanRetryGateKeyPrefix + pairKey

	count, err := s.client.Incr(ctx, countKey).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := s.client.Expire(ctx, countKey, 5*time.Minute).Err(); err != nil {
			return false, err
		}
	}
	backoff := connectPlanRetryBackoff(int(count))
	if backoff <= 0 {
		return true, nil
	}
	allowed, err := s.client.SetNX(ctx, gateKey, count, backoff).Result()
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func (s *RedisTokenStore) ResetConnectPlanRetry(ctx context.Context, networkID, nodeID, peerNodeID string) error {
	pairKey := connectPlanRetryPairKey(networkID, nodeID, peerNodeID)
	return s.client.Del(
		ctx,
		connectPlanRetryCountKeyPrefix+pairKey,
		connectPlanRetryGateKeyPrefix+pairKey,
	).Err()
}

func (s *RedisTokenStore) AcquirePeerCandidateDelivery(ctx context.Context, networkID, sourceNodeID, targetNodeID string, candidate controlmsg.PeerCandidate, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = 10 * time.Second
	}
	key := peerCandidateDeliveryKeyPrefix + peerCandidateDeliveryKey(networkID, sourceNodeID, targetNodeID, candidate)
	return s.client.SetNX(ctx, key, 1, ttl).Result()
}

func connectPlanRetryPairKey(networkID, nodeID, peerNodeID string) string {
	if nodeID > peerNodeID {
		nodeID, peerNodeID = peerNodeID, nodeID
	}
	return networkID + ":" + nodeID + ":" + peerNodeID
}

func peerCandidateDeliveryKey(networkID, sourceNodeID, targetNodeID string, candidate controlmsg.PeerCandidate) string {
	return networkID + ":" + sourceNodeID + ":" + targetNodeID + ":" + candidate.CandidateType + ":" + candidate.Endpoint + ":" + fmt.Sprintf("%d", candidate.Priority)
}

func deviceNetworkStateKey(deviceID, networkID string) string {
	return deviceNetworkStateKeyPrefix + deviceID + ":" + networkID
}

func connectPlanRetryBackoff(failures int) time.Duration {
	switch {
	case failures <= 1:
		return 0
	case failures == 2:
		return 2 * time.Second
	case failures == 3:
		return 5 * time.Second
	case failures == 4:
		return 10 * time.Second
	default:
		return 20 * time.Second
	}
}
