package util

// MinInt 返回两个整数中的较小值。
//
// 目前主要用于控制面下发建议值时设置上限，例如推荐 fanout 数量。
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
