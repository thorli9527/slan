// 本文件包含“注册相关”入参校验与基础前置条件校验：
// - requireRegisterDeviceRequest / requireRegisterNodeRequest：校验注册设备/节点请求字段
// - requireUser：校验 userId 存在
// - usableClientDeviceID：校验客户端可用的 deviceId（过滤保留值/保留前缀）
package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbDeviceService) requireRegisterDeviceRequest(req dto.RegisterDeviceRequest) error {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Platform) == "" || strings.TrimSpace(req.PublicKey) == "" { // name/platform/publicKey 必填
		return fmt.Errorf("%w: name, platform, and publicKey are required", ErrInvalidArgument) // 缺字段：INVALID_ARGUMENT
	}
	if deviceID := strings.TrimSpace(req.DeviceID); deviceID != "" && !usableClientDeviceID(deviceID) { // deviceId 可选；若提供则必须满足可用规则
		return fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument) // 非法 deviceId
	}
	return nil // 校验通过
}

func (s dbNodeService) requireRegisterNodeRequest(req dto.RegisterNodeRequest) error {
	if strings.TrimSpace(req.DeviceID) == "" || strings.TrimSpace(req.NodeID) == "" || strings.TrimSpace(req.NodePublicKey) == "" { // deviceId/nodeId/nodePublicKey 必填
		return fmt.Errorf("%w: deviceId, nodeId, and nodePublicKey are required", ErrInvalidArgument) // 缺字段：INVALID_ARGUMENT
	}
	return nil // 校验通过
}

func (s *dbState) requireUser(ctx context.Context, userID string) error {
	if _, err := s.pg.GetUserByID(ctx, userID); err != nil { // 查询 userId 是否存在
		if repo.IsNotFound(err) { // 用户不存在
			return ErrUnauthorized // 视为未授权（调用方提供的 userId 不可信）
		}
		return err // 其他错误：数据库异常等
	}
	return nil // user 存在
}

func usableClientDeviceID(deviceID string) bool {
	value := strings.TrimSpace(deviceID) // 去掉首尾空格
	if value == "" {                     // 空字符串不可用
		return false
	}
	lower := strings.ToLower(value)                                                                    // 统一小写做规则匹配（避免大小写绕过）
	if lower == "authcallbackid" || lower == "windows-plugin-login" || lower == "macos-plugin-login" { // 保留值：用于历史兼容/内部流程，禁止作为真实 deviceId
		return false
	}
	return !strings.HasPrefix(lower, "cb-") // 保留前缀：cb- 通常用于 callbackId 等临时标识，禁止作为 deviceId
}
