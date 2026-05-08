// 本文件实现固定的访问策略同步逻辑（Access Policy Sync）。
//
// 当前策略属于“后台周期性自修复/收敛”：
// - 定时扫描所有用户
// - 按固定规则（例如设备上限）对其资源做收敛
package impl

import (
	"context"
	"time"
)

const accessPolicySyncInterval = 30 * time.Second // 策略同步周期：每 30 秒执行一次

func (s *dbState) startAccessPolicySyncLoop() {
	go func() { // 后台 goroutine：常驻执行策略收敛
		ticker := time.NewTicker(accessPolicySyncInterval) // 周期触发器
		defer ticker.Stop()                                // goroutine 退出时释放 ticker
		for {                                              // 无限循环：直到进程退出
			s.enforceFixedAccessPolicy(context.Background()) // 执行一次策略收敛（当前不透出错误，失败会在下一个周期重试）
			<-ticker.C                                       // 等待下一次 tick
		}
	}() // 立即启动
}

func (s *dbState) enforceFixedAccessPolicy(ctx context.Context) {
	users, err := s.pg.ListUsers(ctx) // 扫描全量用户（用于逐用户执行策略）
	if err != nil {                   // 数据库异常或连接失败
		return // 当前实现选择静默返回：不阻塞主链路
	}
	for _, user := range users { // 对每个用户执行固定策略
		s.enforceUserDeviceLimit(ctx, user.UserID, fixedDeviceLimit()) // 设备上限策略：超过限制则挂起多余 attachment
	}
}

func (s *dbState) enforceUserDeviceLimit(ctx context.Context, userID string, limit int) {
	if limit <= 0 { // limit<=0 视为“不启用限制”
		return
	}
	attachments, err := s.pg.ListActiveAttachmentsByUser(ctx, userID) // 获取用户当前所有 active 的 attachment（通常代表可用设备入网挂载）
	if err != nil || len(attachments) <= limit {                      // 查询失败，或数量未超过限制
		return
	}
	for _, attachment := range attachments[limit:] { // 对超出上限的部分做收敛（保留前 limit 个，其余挂起）
		if err := s.pg.SuspendAttachment(ctx, attachment.AttachmentID); err != nil { // 将 attachment 标记为 suspended/disabled
			continue // 单个挂起失败不影响后续条目
		}
		// 通知：设备 IP 需要重新分配/该 attachment 不再可用。
		// 这里传空字符串表示“没有新的 IP”（客户端应按控制面下发状态做收敛）。
		s.publishDeviceIPReassigned(attachment.NetworkID, attachment.DeviceID, attachment.AttachmentID, "")
	}
}
