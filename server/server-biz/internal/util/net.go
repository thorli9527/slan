package util

import (
	"strconv"
	"strings"
)

// SplitRelayAddress 把 relay 地址拆成 host 和 port。
//
// 输入通常是 `host:port` 或 `ip:port`。
// 如果端口无法解析，则返回原地址和 0，调用方可按“未知端口”继续处理。
func SplitRelayAddress(address string) (string, int) {
	parts := strings.Split(address, ":")
	if len(parts) < 2 {
		return address, 0
	}
	port, _ := strconv.Atoi(parts[len(parts)-1])
	host := strings.Join(parts[:len(parts)-1], ":")
	return host, port
}
