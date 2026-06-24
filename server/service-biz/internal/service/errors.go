package service

import "errors"

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrForbidden       = errors.New("forbidden")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrNotImplemented  = errors.New("not implemented")
)

type classifiedError struct {
	kind    error
	message string
}

func (e classifiedError) Error() string {
	if e.message != "" {
		return e.message
	}
	return e.kind.Error()
}

func (e classifiedError) Unwrap() error {
	return e.kind
}

func invalidArgumentError(message string) error {
	return classifiedError{kind: ErrInvalidArgument, message: message}
}

func conflictError(message string) error {
	return classifiedError{kind: ErrConflict, message: message}
}
