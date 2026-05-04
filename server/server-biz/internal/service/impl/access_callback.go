// 本文件实现“浏览器登录回调”相关逻辑：
// - CompleteCallback：浏览器侧把登录结果写入服务端（通过 accessToken 自证身份）
// - GetCallbackStatus：客户端轮询回调状态并获取 payload
// - publishAuthCallbackToDevice：当目标设备已建立 MQTT 控制通道时，额外通过 control/down 主动推送回调消息
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
	callbackID = strings.TrimSpace(callbackID) // 规范化 callbackId
	if callbackID == "" {                      // callbackId 必填
		return dto.AuthCallbackStatusResponse{}, ErrInvalidArgument
	}
	var payload dto.CompleteAuthCallbackRequest                                                      // token store 中保存的回调 payload（登录结果）
	ready, err := s.state.tokens.LoadAuthCallbackPayload(context.Background(), callbackID, &payload) // 从 token store 读取：ready 表示是否已有 payload
	if err != nil {                                                                                  // token store 异常
		return dto.AuthCallbackStatusResponse{}, err
	}
	var responsePayload *dto.CompleteAuthCallbackRequest // 响应中 payload 字段指针：仅在 ready 时返回内容
	if ready {                                           // 已准备好
		responsePayload = &payload // 返回 payload 的地址
	}
	return dto.AuthCallbackStatusResponse{ // 组装响应
		CallbackID: callbackID,      // 回调 ID
		Ready:      ready,           // 是否已完成
		Payload:    responsePayload, // 若 ready 则包含 payload，否则为 nil
	}, nil
}

func (s dbAuthService) CompleteCallback(callbackID string, req dto.CompleteAuthCallbackRequest) error {
	callbackID = strings.TrimSpace(callbackID) // 规范化 callbackId
	if callbackID == "" {                      // callbackId 必填
		return ErrInvalidArgument
	}
	req.AccessToken = strings.TrimSpace(req.AccessToken) // 回调携带的 access token（用于服务端校验回调发起者身份）
	req.UserID = strings.TrimSpace(req.UserID)           // 回调声称的 userId
	req.DeviceID = strings.TrimSpace(req.DeviceID)       // 可选：关联设备（用于把回调推送到特定设备）
	req.UserLabel = strings.TrimSpace(req.UserLabel)     // 可选：用户展示名
	req.Action = strings.TrimSpace(req.Action)           // 可选：回调动作（例如登录/绑定等）
	if req.AccessToken == "" || req.UserID == "" {       // accessToken 与 userId 是最低必需字段
		return ErrInvalidArgument
	}
	if req.DeviceID != "" && !usableClientDeviceID(req.DeviceID) { // deviceId 若存在必须合法
		return fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument)
	}
	ctx := context.Background()                                       // 上下文
	session, err := s.state.tokens.Authenticate(ctx, req.AccessToken) // 用 accessToken 校验回调请求的主体
	if err != nil {                                                   // token 无效/过期
		return ErrUnauthorized
	}
	if session.UserID != req.UserID { // access token 对应的 userId 必须与请求 body 中的 userId 一致，避免伪造 payload
		return ErrForbidden
	}
	if req.DeviceID != "" { // 如果指定了 deviceId，则进一步校验该设备归属，避免把回调推送给别人的设备
		device, err := s.state.pg.GetDeviceByID(ctx, req.DeviceID) // 查设备
		if err != nil {                                            // 查设备失败
			if repo.IsNotFound(err) { // 设备不存在
				return ErrForbidden
			}
			return err // 其他错误透传
		}
		if device.UserID != req.UserID { // 设备不归属该 user
			return ErrForbidden
		}
	}
	if req.ExpiresIn <= 0 { // 回调 token 的默认有效期（秒）：如果未提供则默认 1h
		req.ExpiresIn = 3600
	}
	if err := s.state.tokens.StoreAuthCallbackPayload( // 写入 token store：callbackId -> payload
		ctx,            // 上下文
		callbackID,     // key
		req,            // payload
		10*time.Minute, // 回调 payload 在服务端的保留期：客户端轮询窗口
	); err != nil {
		return err
	}
	s.state.publishAuthCallbackToDevice(ctx, callbackID, req) // 若设备在线且 MQTT 启用，尝试通过控制通道主动推送
	return nil                                                // 成功
}

func (s *dbState) publishAuthCallbackToDevice(ctx context.Context, callbackID string, payload dto.CompleteAuthCallbackRequest) {
	deviceID := strings.TrimSpace(payload.DeviceID) // 目标设备 ID（必须存在才知道投递到哪个 control/down topic）
	if deviceID == "" || !s.cfg.MQTT.Enabled {      // 未指定 deviceId 或 MQTT 未启用：直接不推送
		return
	}
	credential := mqttauth.ServerSubscriberCredential(s.cfg.MQTT, time.Now()) // server-biz 作为“server principal”发布下行消息所用的 MQTT 凭证
	if credential == nil {                                                    // 未能生成凭证（例如配置缺失）
		return
	}
	timeout := time.Duration(s.cfg.MQTT.PublishTimeoutMilliseconds) * time.Millisecond // 发布超时：优先用配置
	if timeout <= 0 {                                                                  // 配置未设置时给默认值
		timeout = 3 * time.Second
	}
	publishCtx, cancel := context.WithTimeout(ctx, timeout) // 发布使用带超时的 ctx，避免阻塞调用链
	defer cancel()                                          // 释放资源
	_ = mqttauth.PublishJSONWithOptions(                    // 发布失败时忽略错误：客户端仍可通过轮询 GET /auth/callback-status 获取 payload
		publishCtx,          // 发布上下文
		s.cfg.MQTT,          // MQTT 配置
		credential.ClientID, // server principal clientId
		credential.Username, // server principal username
		credential.Password, // server principal password
		mqttauth.ControlDownTopic(s.cfg.MQTT, deviceID), // 目标设备的下行控制 topic：{topicPrefix}/{deviceId}/control/down
		controlmsg.Envelope{ // 控制通道统一 envelope：type + messageId + payload
			Type:      "auth_callback",   // 消息类型：登录回调
			MessageID: util.NewID("msg"), // 消息 ID：用于幂等/诊断
			Payload: map[string]any{ // payload：把回调内容透传到设备侧
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
		mqttauth.PublishOptions{QoS: mqttauth.PublishQoSExactlyOnce}, // QoS2：确保消息可靠到达（至少一次/恰好一次语义由 broker 实现保证）
	)
}
