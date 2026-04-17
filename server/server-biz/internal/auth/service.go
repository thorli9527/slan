package auth

import "github.com/slan/server/server-biz/api/dto"

// Service 定义账号注册和登录能力。
type Service interface {
	// Register 创建新用户账号并返回访问令牌对。
	Register(req dto.RegisterRequest) (dto.AuthResponse, error)
	// Login 校验登录凭证并返回访问令牌对。
	Login(req dto.LoginRequest) (dto.AuthResponse, error)
}
