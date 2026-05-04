// Package impl implements server-biz business services backed by persistent storage (Postgres/Redis)
// and runtime components. This file focuses on account access: register/login/refresh/password change,
// plus console-login-key workflows used by the web console and desktop clients.
package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
)

type dbAuthService struct{ state *dbState }   // Auth 相关 service 实现：依赖 dbState 以访问数据库与 token 存储
type dbTokenVerifier struct{ state *dbState } // Bearer Token 校验实现：依赖 dbState.tokens

type consoleLoginKeyPayload struct {
	UserID string `json:"userId"` // 兑换后签发 AuthResponse 的主体用户
	// DeviceID 是可选设备上下文：用于把 token 与某台设备关联（例如 console 自动打开该设备视图）。
	DeviceID string `json:"deviceId,omitempty"`
}

var (
	_ service.Auth          = dbAuthService{}   // 编译期断言：dbAuthService 满足 service.Auth 接口
	_ service.TokenVerifier = dbTokenVerifier{} // 编译期断言：dbTokenVerifier 满足 service.TokenVerifier 接口
)

func (s dbAuthService) Register(req dto.RegisterRequest) (dto.AuthResponse, error) {
	if !s.state.cfg.Auth.AllowRegistration { // 配置开关：是否允许终端用户自助注册
		return dto.AuthResponse{}, ErrForbidden // 不允许注册时直接拒绝
	}
	email := strings.TrimSpace(strings.ToLower(req.Email)) // 规范化邮箱：去空格并统一小写，避免同邮箱不同写法重复注册
	password := strings.TrimSpace(req.Password)            // 去掉首尾空格，避免无意输入空格导致后续无法登录
	if email == "" || len(password) < 8 {                  // 账号与密码基本校验：邮箱不能为空；密码最小长度约束（>=8）
		return dto.AuthResponse{}, fmt.Errorf("%w: email and password are required", ErrInvalidArgument) // 参数不合法统一映射为 INVALID_ARGUMENT
	}

	ctx := context.Background()                                      // 当前实现：服务方法内部自行创建 ctx（调用方路由层未传入 ctx）
	if _, err := s.state.pg.GetUserByEmail(ctx, email); err == nil { // 如果能查到用户，说明邮箱已存在
		return dto.AuthResponse{}, fmt.Errorf("%w: email already exists", ErrConflict) // 冲突：同邮箱重复注册
	} else if !repo.IsNotFound(err) { // 除了 NotFound 以外的错误都视为底层存储异常
		return dto.AuthResponse{}, err // 直接透传底层错误，由统一错误映射层转换 HTTP/错误码
	} // NotFound：允许继续创建新用户

	userID := util.NewID("user")                    // 生成业务用户 ID（带前缀便于排查与区分实体类型）
	if err := s.state.pg.CreateUser(ctx, repo.User{ // 写入持久化用户记录
		UserID:       userID,                      // 主键
		Email:        email,                       // 已规范化邮箱
		PasswordHash: util.HashPassword(password), // 存储密码哈希而非明文
	}); err != nil { // 插入失败（例如唯一索引冲突、数据库异常）
		return dto.AuthResponse{}, err // 透传错误
	}
	return s.state.issueAuthResponse(ctx, userID, "") // 注册成功后立即签发 access/refresh token；deviceId 为空表示不绑定特定设备
}

func (s dbAuthService) Login(req dto.LoginRequest) (dto.AuthResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email)) // 规范化邮箱：与注册时一致
	password := strings.TrimSpace(req.Password)            // 去空格后的密码（用于校验）
	deviceID := strings.TrimSpace(req.DeviceID)            // 可选：登录时携带 deviceId，用于把 token 与设备上下文关联/校验
	if email == "" || password == "" {                     // 必填字段校验
		return dto.AuthResponse{}, fmt.Errorf("%w: email and password are required", ErrInvalidArgument) // 参数缺失
	}
	if deviceID != "" && !usableClientDeviceID(deviceID) { // 如果传了 deviceId，则需要满足“可用设备 ID”规则（避免保留值/前缀）
		return dto.AuthResponse{}, fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument) // 不合法 deviceId
	}
	ctx := context.Background()                        // 创建上下文
	user, err := s.state.pg.GetUserByEmail(ctx, email) // 通过邮箱查用户
	if err != nil {                                    // 查询失败分支
		if repo.IsNotFound(err) { // 不存在该邮箱：不泄露账号是否存在，统一返回未授权
			return dto.AuthResponse{}, ErrUnauthorized
		}
		return dto.AuthResponse{}, err // 其余错误：数据库异常等
	}
	if user.PasswordHash != util.HashPassword(password) && user.PasswordHash != util.HashPassword(req.Password) { // 校验密码哈希；同时兼容未 trim 的原始输入
		return dto.AuthResponse{}, ErrUnauthorized // 密码不匹配
	}
	if deviceID != "" { // 如果调用方带了 deviceId，则强制校验设备归属，避免把别人的 deviceId 绑定进当前用户会话
		device, err := s.state.pg.GetDeviceByID(ctx, deviceID) // 查询 device
		if err != nil {                                        // 查询失败
			if repo.IsNotFound(err) { // 设备不存在：按未授权处理
				return dto.AuthResponse{}, ErrUnauthorized
			}
			return dto.AuthResponse{}, err // 其余错误：数据库异常
		}
		if device.UserID != user.UserID { // 设备存在但不属于该用户：明确禁止
			return dto.AuthResponse{}, ErrForbidden
		}
	}
	return s.state.issueAuthResponse(ctx, user.UserID, deviceID) // 登录成功，签发新 token；若带 deviceId 则 token 带设备上下文
}

func (s dbAuthService) Refresh(req dto.RefreshTokenRequest) (dto.AuthResponse, error) {
	refreshToken := strings.TrimSpace(req.RefreshToken) // refresh token：去掉首尾空格
	deviceID := strings.TrimSpace(req.DeviceID)         // 可选：刷新时携带 deviceId，用于校验/绑定 token 上下文
	if refreshToken == "" {                             // refreshToken 必须存在
		return dto.AuthResponse{}, fmt.Errorf("%w: refreshToken is required", ErrInvalidArgument)
	}
	if deviceID != "" && !usableClientDeviceID(deviceID) { // deviceId 若存在则也必须合法
		return dto.AuthResponse{}, fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument)
	}
	ctx := context.Background()                                               // 上下文
	userID, err := s.state.tokens.AuthenticateRefreshToken(ctx, refreshToken) // 校验 refresh token 并取回 userId
	if err != nil {                                                           // token 不存在/过期/签名不合法等
		return dto.AuthResponse{}, ErrUnauthorized // 刷新失败按未授权处理
	}
	if err := s.state.tokens.DeleteRefreshToken(ctx, refreshToken); err != nil { // 单次使用：消费/删除旧 refresh token，避免重放
		return dto.AuthResponse{}, err
	}
	if deviceID != "" { // 若携带 deviceId，校验该 device 是否属于当前 user，避免跨用户绑定
		device, err := s.state.pg.GetDeviceByID(ctx, deviceID)
		if err != nil {
			if repo.IsNotFound(err) { // 设备不存在
				return dto.AuthResponse{}, ErrUnauthorized
			}
			return dto.AuthResponse{}, err
		}
		if device.UserID != userID { // 设备归属不一致
			return dto.AuthResponse{}, ErrForbidden
		}
	}
	return s.state.issueAuthResponse(ctx, userID, deviceID) // refresh 成功后签发新的 access/refresh token
}

func (s dbAuthService) ChangePassword(userID string, req dto.ChangePasswordRequest) error {
	userID = strings.TrimSpace(userID) // 规范化 userId
	if userID == "" {                  // 无 userId 意味着未鉴权或上下文缺失
		return ErrUnauthorized
	}
	currentPassword := strings.TrimSpace(req.CurrentPassword) // 当前密码（trim）
	newPassword := strings.TrimSpace(req.NewPassword)         // 新密码（trim）
	if currentPassword == "" || len(newPassword) < 8 {        // 入参校验：当前密码不能为空；新密码最小长度约束
		return fmt.Errorf("%w: currentPassword and newPassword are required, newPassword must be at least 8 characters", ErrInvalidArgument)
	}
	ctx := context.Background()                      // 上下文
	user, err := s.state.pg.GetUserByID(ctx, userID) // 查用户
	if err != nil {                                  // 查询失败
		if repo.IsNotFound(err) { // 用户不存在按未授权处理
			return ErrUnauthorized
		}
		return err // 其他错误透传
	}
	if user.PasswordHash != util.HashPassword(currentPassword) && user.PasswordHash != util.HashPassword(req.CurrentPassword) { // 校验旧密码（兼容未 trim）
		return ErrUnauthorized
	}
	return s.state.pg.UpdateUserPassword(ctx, userID, util.HashPassword(newPassword)) // 更新数据库中的密码哈希
}

func (s dbAuthService) CreateConsoleLoginKey(userID string, req dto.CreateConsoleLoginKeyRequest) (dto.ConsoleLoginKeyResponse, error) {
	userID = strings.TrimSpace(userID)          // 规范化 userId
	deviceID := strings.TrimSpace(req.DeviceID) // 可选：绑定设备上下文（用于 console 自动选中设备等）
	if userID == "" {                           // 未鉴权
		return dto.ConsoleLoginKeyResponse{}, ErrUnauthorized
	}
	if deviceID != "" && !usableClientDeviceID(deviceID) { // 如果传了 deviceId 则必须合法
		return dto.ConsoleLoginKeyResponse{}, fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument)
	}
	ctx := context.Background()                                    // 上下文
	if _, err := s.state.pg.GetUserByID(ctx, userID); err != nil { // 校验用户存在
		if repo.IsNotFound(err) { // 用户不存在
			return dto.ConsoleLoginKeyResponse{}, ErrUnauthorized
		}
		return dto.ConsoleLoginKeyResponse{}, err // 其他错误透传
	}
	if deviceID != "" { // deviceId 存在时校验 device 归属，防止拿别人的 deviceId 生成可登录 key
		device, err := s.state.pg.GetDeviceByID(ctx, deviceID)
		if err != nil {
			if repo.IsNotFound(err) { // 设备不存在
				return dto.ConsoleLoginKeyResponse{}, ErrForbidden
			}
			return dto.ConsoleLoginKeyResponse{}, err
		}
		if device.UserID != userID { // 设备不属于当前用户
			return dto.ConsoleLoginKeyResponse{}, ErrForbidden
		}
	}
	ttl := 2 * time.Minute                                                               // console login key 的有效期：短时效，降低泄露风险
	loginKey := util.OpaqueToken("console", userID)                                      // 生成不可逆的随机 token（前缀用于区分用途）
	if err := s.state.tokens.StoreConsoleLoginKey(ctx, loginKey, consoleLoginKeyPayload{ // 将 token->payload 写入 token store（通常为 Redis）
		UserID:   userID,   // 兑换后签发 token 的主体用户
		DeviceID: deviceID, // 可选设备上下文
	}, ttl); err != nil {
		return dto.ConsoleLoginKeyResponse{}, err // 写入失败
	}
	return dto.ConsoleLoginKeyResponse{ // 返回给调用方，由 Web Console/客户端携带使用
		LoginKey:  loginKey,                 // 一次性 key
		ExpiresIn: int64(ttl / time.Second), // 以秒表示的过期时间
	}, nil
}

func (s dbAuthService) ConsumeConsoleLoginKey(req dto.ConsumeConsoleLoginKeyRequest) (dto.AuthResponse, error) {
	loginKey := strings.TrimSpace(req.LoginKey) // 待消费的 console login key
	if loginKey == "" {                         // 必填校验
		return dto.AuthResponse{}, ErrInvalidArgument
	}
	ctx := context.Background()                                               // 上下文
	var payload consoleLoginKeyPayload                                        // token store 中存储的 payload
	ok, err := s.state.tokens.ConsumeConsoleLoginKey(ctx, loginKey, &payload) // 原子消费：读取并删除 key，避免多次兑换
	if err != nil {                                                           // token store 异常
		return dto.AuthResponse{}, err
	}
	if !ok || strings.TrimSpace(payload.UserID) == "" { // 未命中/已过期/payload 不完整
		return dto.AuthResponse{}, ErrUnauthorized
	}
	return s.state.issueAuthResponse(ctx, payload.UserID, payload.DeviceID) // 兑换成功：为 payload.UserID 签发 token（并可携带 device 上下文）
}
