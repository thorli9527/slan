// 本文件负责 token 的签发与校验：
// - issueAuthResponse：生成并持久化 access/refresh token，返回给客户端
// - dbTokenVerifier.Authenticate：对 HTTP Bearer access token 做校验，并执行“绑定设备的 web session 新鲜度”检查
package impl

import (
	"context"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s *dbState) issueAuthResponse(ctx context.Context, userID, deviceID string) (dto.AuthResponse, error) {
	email := ""                                // 默认邮箱为空；如果能查到用户则补齐
	user, err := s.pg.GetUserByID(ctx, userID) // 查询用户以获取 email（用于客户端展示/校验）
	if err == nil {                            // 查询成功
		email = user.Email // 写入邮箱
	} else if !repo.IsNotFound(err) { // 允许 NotFound（例如 token 兑换场景只知道 userId）继续走，但其余错误直接返回
		return dto.AuthResponse{}, err
	}
	accessToken := util.OpaqueToken("access", userID)                                                // 生成不可逆的 access token（前缀用于区分用途）
	refreshToken := util.OpaqueToken("refresh", userID)                                              // 生成不可逆的 refresh token
	accessTTL := tokenTTL(s.cfg.Auth.AccessTokenTTLSeconds, time.Hour)                               // access token TTL：优先用配置，缺省 1h
	refreshTTL := tokenTTL(s.cfg.Auth.RefreshTokenTTLSeconds, 24*time.Hour)                          // refresh token TTL：优先用配置，缺省 24h
	if err := s.tokens.StoreAccessToken(ctx, accessToken, userID, deviceID, accessTTL); err != nil { // 持久化 access token -> session(userId, deviceId, expiry)
		return dto.AuthResponse{}, err
	}
	if err := s.tokens.StoreRefreshToken(ctx, refreshToken, userID, refreshTTL); err != nil { // 持久化 refresh token -> userId
		return dto.AuthResponse{}, err
	}
	return dto.AuthResponse{ // 组装返回给客户端的响应
		UserID:       userID,                         // 用户 ID
		Email:        email,                          // 用户邮箱（若可查到）
		AccessToken:  accessToken,                    // access token
		RefreshToken: refreshToken,                   // refresh token
		ExpiresIn:    int64(accessTTL / time.Second), // access token 剩余有效期（秒）
	}, nil
}

func tokenTTL(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 { // 未配置或配置为非正数：使用默认值
		return fallback
	}
	return time.Duration(seconds) * time.Second // 配置单位为秒，转换为 time.Duration
}

func (v dbTokenVerifier) Authenticate(accessToken string) (string, error) {
	ctx := context.Background()                                   // 当前接口签名未带 ctx，因此内部创建 background ctx
	session, err := v.state.tokens.Authenticate(ctx, accessToken) // 校验 access token 并解析出 session 信息
	if err != nil {                                               // token 无效/过期/不存在
		return "", ErrUnauthorized
	}
	if session.DeviceID != "" && !v.state.hasFreshDeviceBoundWebSession(ctx, session, time.Now()) { // 如果 token 绑定了 deviceId，则额外要求该设备的 web session 仍“新鲜”
		_ = v.state.tokens.DeleteAccessToken(ctx, accessToken) // 不新鲜时主动删除该 access token，避免继续被重放
		return "", ErrUnauthorized
	}
	return session.UserID, nil // 鉴权成功：返回 userId，供路由层写入 context
}
