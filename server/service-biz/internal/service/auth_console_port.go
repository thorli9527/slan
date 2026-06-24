package service

import "context"

type AuthConsoleUseCase interface {
	AuthConsoleKeyUseCase
	AuthConsoleLoginUseCase
}

type AuthConsoleKeyUseCase interface {
	CreateConsoleLoginKey(ctx context.Context, input CreateConsoleLoginKeyInput) (ConsoleLoginKeyView, error)
}

type AuthConsoleLoginUseCase interface {
	ConsoleLogin(ctx context.Context, input ConsoleLoginInput) (AuthSessionView, error)
}
