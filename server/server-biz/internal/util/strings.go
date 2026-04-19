package util

import (
	"sort"
	"strings"
)

// NormalizeEmail 对邮箱做控制面统一规范化。
//
// 当前规则只包含：
// 1. 去掉首尾空白
// 2. 转成小写
//
// 这样仓库里所有用户邮箱的查询、写入和比较都能保持一致。
func NormalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

// FirstNonEmpty 返回第一个非空白字符串。
//
// 这个 helper 常用于：
// - 展示字段回退
// - metrics 标签回退
// - 配置字段默认值选择
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// DedupeTrimmed 返回去空白、去空字符串、去重后的字符串切片。
//
// 保留原有相对顺序，适合处理：
// - role/menu 绑定请求
// - relay/DERP 节点偏好列表
// - 各类用户输入的字符串数组
func DedupeTrimmed(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// SortedStrings 返回一个排序后的副本，不修改原切片。
//
// 主要用于生成稳定的 cache key、签名 payload 和测试可预测输出。
func SortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
