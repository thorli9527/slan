package util

// Ptr 返回传入值的指针。
//
// 用于构造 DTO、观测字段和可选配置值时减少样板代码。
func Ptr[T any](value T) *T {
	return &value
}
