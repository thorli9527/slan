package service

import "errors"

var (
	// ErrUnauthorized indicates the caller did not provide a valid credential.
	//
	// HTTP mapping:
	// - 401 UNAUTHORIZED
	//
	// Common causes:
	// - missing/invalid bearer token
	// - expired/consumed refresh token
	// - invalid console login key
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden indicates the caller is authenticated but not allowed to access the target resource.
	//
	// HTTP mapping:
	// - 403 FORBIDDEN
	//
	// Common causes:
	// - device/network/node not owned by current user
	// - non-owner attempting to mutate owner-only resources
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound indicates the target resource does not exist or is not visible under current access rules.
	//
	// HTTP mapping:
	// - 404 NOT_FOUND
	//
	// Notes:
	// - Implementations may deliberately return ErrNotFound instead of ErrForbidden to avoid information leaks.
	ErrNotFound = errors.New("not found")
	// ErrConflict indicates the request conflicts with existing state (e.g., duplicate create).
	//
	// HTTP mapping:
	// - 409 CONFLICT
	ErrConflict = errors.New("conflict")
	// ErrInvalidArgument indicates the request is missing required fields, contains invalid values,
	// or violates constraints.
	//
	// HTTP mapping:
	// - 400 INVALID_ARGUMENT
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrDeviceLimitExceeded indicates the caller exceeded a device-related quota/limit.
	// Implementations may map this to a dedicated error code for UI display.
	ErrDeviceLimitExceeded = errors.New("device limit exceeded")
	// ErrNotImplemented indicates an API surface exists but the backing implementation is not ready.
	// Implementations should avoid returning this in public production APIs.
	ErrNotImplemented = errors.New("not implemented")
)
