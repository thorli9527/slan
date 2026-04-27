package service

import "errors"

var (
	// ErrUnauthorized 表示调用方未提供有效身份凭证。
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden 表示调用方身份有效，但无权访问目标资源。
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound 表示目标资源不存在或当前上下文不可见。
	ErrNotFound = errors.New("not found")
	// ErrConflict 表示请求与现有状态冲突，例如重复创建。
	ErrConflict = errors.New("conflict")
	// ErrInvalidArgument 表示请求参数缺失、非法或不满足约束。
	ErrInvalidArgument = errors.New("invalid argument")
	ErrPaymentRequired = errors.New("payment required")
	ErrNotImplemented  = errors.New("not implemented")
)
