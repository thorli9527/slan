package service

import "github.com/slan/server/server-biz/api/dto"

// Auth 定义账号注册和登录能力。
type Auth interface {
	// Register 创建一个新用户并返回后续访问业务接口所需的认证结果。
	Register(req dto.RegisterRequest) (dto.AuthResponse, error)
	// Login 校验用户身份并返回新的访问令牌与当前用户信息。
	Login(req dto.LoginRequest) (dto.AuthResponse, error)
	// GetCallbackStatus 返回网页登录回调是否已被桌面客户端接收。
	GetCallbackStatus(callbackID string) (dto.AuthCallbackStatusResponse, error)
	// CompleteCallback 暂存网页登录结果，等待桌面客户端主动拉取。
	CompleteCallback(callbackID string, req dto.CompleteAuthCallbackRequest) error
	// MarkCallbackReceived 标记桌面客户端已接收网页登录回调。
	MarkCallbackReceived(callbackID string) error
}

// TokenVerifier 定义控制面对访问令牌的校验能力。
type TokenVerifier interface {
	// Authenticate 校验访问令牌并返回其绑定的用户 ID。
	Authenticate(accessToken string) (string, error)
}
