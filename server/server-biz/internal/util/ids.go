package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// HashPassword 对原始密码做固定哈希。
//
// 这里仍然是 phase-1 的简化实现，只做 SHA-256，
// 目的是让开发期控制面具备最小可用认证能力。
// 生产环境应替换成带盐、可调成本的密码哈希方案。
func HashPassword(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// NewID 生成一个带前缀的业务 ID。
//
// 该函数优先使用随机字节；随机源不可用时退化为时间戳，
// 保证控制面在受限环境里仍能继续工作。
func NewID(prefix string) string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf[:]))
}

// OpaqueToken 生成不透明令牌字符串。
//
// 目前用于 access token、refresh token 等轻量会话票据。
// 它不是自描述 JWT，而是交给 Redis/服务端状态来解释。
func OpaqueToken(kind, subject string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(kind + ":" + subject + ":" + NewID("tok")))
}

// RandomHex 返回固定长度的十六进制随机字符串。
func RandomHex(bytes int) string {
	if bytes <= 0 {
		return ""
	}
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		return hex.EncodeToString(sum[:bytes])
	}
	return hex.EncodeToString(buf)
}
