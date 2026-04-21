package dto

// RegisterRequest 用于注册新的终端用户账号。
type RegisterRequest struct {
	// Email 是唯一账号标识。
	Email string `json:"email"`
	// Password 是客户端提交的原始凭证，由服务端负责哈希处理。
	Password string `json:"password"`
}

// LoginRequest 用于认证已有的终端用户账号。
type LoginRequest struct {
	// Email 是唯一账号标识。
	Email string `json:"email"`
	// Password 是用于登录校验的原始凭证。
	Password string `json:"password"`
}

// AuthResponse 返回控制面的访问令牌对。
type AuthResponse struct {
	// UserID 是认证通过后的用户标识。
	UserID string `json:"userId"`
	// AccessToken 是受保护接口使用的 Bearer Token。
	AccessToken string `json:"accessToken"`
	// RefreshToken 预留给更长生命周期的会话续期使用。
	RefreshToken string `json:"refreshToken,omitempty"`
	// ExpiresIn 是访问令牌的有效期，单位为秒。
	ExpiresIn int64 `json:"expiresIn"`
}

// CompleteAuthCallbackRequest 用于网页端把认证结果暂存到服务端，等待桌面端拉取。
type CompleteAuthCallbackRequest struct {
	AccessToken string `json:"accessToken"`
	UserID string `json:"userId"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn int64 `json:"expiresIn"`
	DeviceID string `json:"deviceId,omitempty"`
	UserLabel string `json:"userLabel,omitempty"`
	Action string `json:"action,omitempty"`
}

// AuthCallbackStatusResponse 描述桌面客户端是否已接收网页登录回调。
type AuthCallbackStatusResponse struct {
	// CallbackID 是网页登录回调的唯一标识。
	CallbackID string `json:"callbackId"`
	// Ready 表示服务端是否已经缓存了可供客户端拉取的回调数据。
	Ready bool `json:"ready"`
	// Received 表示客户端是否已确认收到回调。
	Received bool `json:"received"`
	// ReceivedAt 是客户端确认收到回调的时间戳，单位毫秒。
	ReceivedAt int64 `json:"receivedAt,omitempty"`
	// Payload 是等待客户端消费的认证结果。
	Payload *CompleteAuthCallbackRequest `json:"payload,omitempty"`
}
