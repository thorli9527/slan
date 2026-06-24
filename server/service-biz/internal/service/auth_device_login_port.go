package service

import "context"

type AuthDeviceLoginUseCase interface {
	AuthDeviceLoginPrepareUseCase
	AuthDeviceLoginCompleteUseCase
}

type AuthDeviceLoginPrepareUseCase interface {
	PrepareDeviceLoginDevice(ctx context.Context, input PrepareDeviceLoginDeviceInput) (PrepareDeviceLoginDeviceView, error)
}

type AuthDeviceLoginCompleteUseCase interface {
	CompleteDeviceLoginDevice(ctx context.Context, input CompleteDeviceLoginDeviceInput) (CompleteDeviceLoginDeviceView, error)
}
