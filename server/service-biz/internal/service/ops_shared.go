package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type OpsAuthSessionService struct {
	Operators         repository.OperatorRepository
	OperatorSessions  repository.OperatorSessionRepository
	NewOperatorSessID func(string) string
	Now               func() time.Time
}

type OpsOperatorService struct {
	Operators repository.OperatorRepository
	Now       func() time.Time
}

type OpsOperatorPasswordService struct {
	Operators repository.OperatorRepository
	Now       func() time.Time
}

type OpsDashboardService struct {
	Users     repository.UserRepository
	Devices   repository.DeviceRepository
	Networks  repository.NetworkRepository
	Operators repository.OperatorRepository
}

type OpsAuditService struct {
	Audit repository.AuditRepository
	Now   func() time.Time
}

type OpsNodeService struct {
	Nodes repository.OpsNodeRepository
	Now   func() time.Time
}

type OpsUserService struct {
	Users     repository.UserRepository
	Devices   repository.DeviceRepository
	NewUserID func() string
	Now       func() time.Time
}

type OpsManagedDeviceService struct {
	Users    repository.UserRepository
	Devices  repository.DeviceRepository
	Networks repository.NetworkRepository
	Now      func() time.Time
}

func opsNow(now func() time.Time) time.Time {
	return currentTime(now)
}

func requireOpsUser(ctx context.Context, users repository.UserRepository, userID string) (model.User, error) {
	user, ok, err := users.GetUser(ctx, userID)
	if err != nil {
		return model.User{}, err
	}
	if !ok {
		return model.User{}, ErrNotFound
	}
	return user, nil
}

func requireOpsDevice(ctx context.Context, devices repository.DeviceRepository, deviceID string) (model.Device, error) {
	device, ok, err := devices.GetDevice(ctx, deviceID)
	if err != nil {
		return model.Device{}, err
	}
	if !ok {
		return model.Device{}, ErrNotFound
	}
	return device, nil
}

func requireOpsOperator(ctx context.Context, operators repository.OperatorRepository, operatorID string) (model.Operator, error) {
	item, ok, err := operators.GetOperator(ctx, operatorID)
	if err != nil {
		return model.Operator{}, err
	}
	if !ok {
		return model.Operator{}, ErrNotFound
	}
	return item, nil
}
