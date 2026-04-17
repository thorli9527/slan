package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	controlws "github.com/slan/server/server-biz/internal/ws"
)

type RedisTokenStore struct {
	client *redis.Client
}

const controlSyncChannel = "control_sync_events"
const networkRevisionKeyPrefix = "network_revision:"

func NewRedisTokenStore(client *redis.Client) *RedisTokenStore {
	return &RedisTokenStore{client: client}
}

func (s *RedisTokenStore) StoreAccessToken(ctx context.Context, token, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, "access_token:"+token, userID, ttl).Err()
}

func (s *RedisTokenStore) StoreRefreshToken(ctx context.Context, token, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, "refresh_token:"+token, userID, ttl).Err()
}

func (s *RedisTokenStore) StoreControlSessionToken(ctx context.Context, token, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, "control_session_token:"+token, userID, ttl).Err()
}

func (s *RedisTokenStore) Authenticate(ctx context.Context, accessToken string) (string, error) {
	userID, err := s.client.Get(ctx, "access_token:"+accessToken).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("token not found")
		}
		return "", err
	}
	return userID, nil
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

func (s *RedisTokenStore) PublishControlSyncEvent(ctx context.Context, event controlws.ControlSyncEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return s.client.Publish(ctx, controlSyncChannel, payload).Err()
}

func (s *RedisTokenStore) SubscribeControlSyncEvents(ctx context.Context, handler func(controlws.ControlSyncEvent)) error {
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			pubsub := s.client.Subscribe(ctx, controlSyncChannel)
			if _, err := pubsub.Receive(ctx); err != nil {
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
						reconnect = true
						break
					}
					var event controlws.ControlSyncEvent
					if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
						continue
					}
					handler(event)
				}
			}
			_ = pubsub.Close()
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
