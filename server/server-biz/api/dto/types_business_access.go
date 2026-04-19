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
