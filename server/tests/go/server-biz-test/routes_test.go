package serverbiztest

import "testing"

// TestPhase1Flow 覆盖当前关键 HTTP 流程：
// 注册、设备注册、节点注册、建网、加入网络、bootstrap 和 relay 票据签发。
func TestPhase1Flow(t *testing.T) {
	t.Skip("phase1 HTTP flow test still depends on removed in-memory service wiring and needs a new test harness")
}
