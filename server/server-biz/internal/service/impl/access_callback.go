package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/mqttauth"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s dbAuthService) GetCallbackStatus(callbackID string) (dto.AuthCallbackStatusResponse, error) {
	callbackID = strings.TrimSpace(callbackID)
	if callbackID == "" {
		return dto.AuthCallbackStatusResponse{}, ErrInvalidArgument
	}
	var payload dto.CompleteAuthCallbackRequest
	ready, err := s.state.tokens.LoadAuthCallbackPayload(context.Background(), callbackID, &payload)
	if err != nil {
		return dto.AuthCallbackStatusResponse{}, err
	}
	var responsePayload *dto.CompleteAuthCallbackRequest
	if ready {
		responsePayload = &payload
	}
	return dto.AuthCallbackStatusResponse{
		CallbackID: callbackID,
		Ready:      ready,
		Payload:    responsePayload,
	}, nil
}

func (s dbAuthService) CompleteCallback(callbackID string, req dto.CompleteAuthCallbackRequest) error {
	callbackID = strings.TrimSpace(callbackID)
	if callbackID == "" {
		return ErrInvalidArgument
	}
	req.AccessToken = strings.TrimSpace(req.AccessToken)
	req.UserID = strings.TrimSpace(req.UserID)
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.UserLabel = strings.TrimSpace(req.UserLabel)
	req.Action = strings.TrimSpace(req.Action)
	if req.AccessToken == "" || req.UserID == "" {
		return ErrInvalidArgument
	}
	if req.DeviceID != "" && !usableClientDeviceID(req.DeviceID) {
		return fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument)
	}
	ctx := context.Background()
	session, err := s.state.tokens.Authenticate(ctx, req.AccessToken)
	if err != nil {
		return ErrUnauthorized
	}
	if session.UserID != req.UserID {
		return ErrForbidden
	}
	if req.DeviceID != "" {
		device, err := s.state.pg.GetDeviceByID(ctx, req.DeviceID)
		if err != nil {
			if repo.IsNotFound(err) {
				return ErrForbidden
			}
			return err
		}
		if device.UserID != req.UserID {
			return ErrForbidden
		}
	}
	if req.ExpiresIn <= 0 {
		req.ExpiresIn = 3600
	}
	if err := s.state.tokens.StoreAuthCallbackPayload(
		ctx,
		callbackID,
		req,
		10*time.Minute,
	); err != nil {
		return err
	}
	s.state.publishAuthCallbackToDevice(ctx, callbackID, req)
	return nil
}

func (s *dbState) publishAuthCallbackToDevice(ctx context.Context, callbackID string, payload dto.CompleteAuthCallbackRequest) {
	deviceID := strings.TrimSpace(payload.DeviceID)
	if deviceID == "" || !s.cfg.MQTT.Enabled {
		return
	}
	credential := mqttauth.ServerSubscriberCredential(s.cfg.MQTT, time.Now())
	if credential == nil {
		return
	}
	timeout := time.Duration(s.cfg.MQTT.PublishTimeoutMilliseconds) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	publishCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_ = mqttauth.PublishJSONWithOptions(
		publishCtx,
		s.cfg.MQTT,
		credential.ClientID,
		credential.Username,
		credential.Password,
		mqttauth.ControlDownTopic(s.cfg.MQTT, deviceID),
		controlmsg.Envelope{
			Type:      "auth_callback",
			MessageID: util.NewID("msg"),
			Payload: map[string]any{
				"callbackId":   callbackID,
				"accessToken":  payload.AccessToken,
				"refreshToken": payload.RefreshToken,
				"userId":       payload.UserID,
				"userLabel":    payload.UserLabel,
				"deviceId":     payload.DeviceID,
				"expiresIn":    payload.ExpiresIn,
				"action":       payload.Action,
			},
		},
		mqttauth.PublishOptions{QoS: mqttauth.PublishQoSExactlyOnce},
	)
}
