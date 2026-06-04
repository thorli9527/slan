package biz

import (
	"strconv"
	"strings"
)

func numericIDSuffix(id string) int {
	index := strings.LastIndex(id, "-")
	if index < 0 || index+1 >= len(id) {
		return 0
	}
	value, _ := strconv.Atoi(id[index+1:])
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxUint32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func nullZeroInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
