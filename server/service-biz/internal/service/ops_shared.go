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
}

type OpsNodeService struct {
	Catalog repository.OpsRepository
	Now     func() time.Time
}

type OpsCustomerService struct {
	Users   repository.UserRepository
	Devices repository.DeviceRepository
	Catalog repository.OpsRepository
	Now     func() time.Time
}

type OpsManagedDeviceService struct {
	Users    repository.UserRepository
	Devices  repository.DeviceRepository
	Networks repository.NetworkRepository
	Now      func() time.Time
}

type OpsCatalogDownloadService struct {
	Catalog repository.OpsRepository
	Now     func() time.Time
}

type OpsCatalogPlanService struct {
	Catalog repository.OpsRepository
	Now     func() time.Time
}

type OpsCatalogProductService struct {
	Catalog repository.OpsRepository
	Now     func() time.Time
}

type OpsCatalogOrderService struct {
	Users   repository.UserRepository
	Catalog repository.OpsRepository
	Now     func() time.Time
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

func requireOpsProduct(ctx context.Context, catalog repository.OpsRepository, productID string) (model.Product, error) {
	item, ok, err := catalog.GetProduct(ctx, productID)
	if err != nil {
		return model.Product{}, err
	}
	if !ok {
		return model.Product{}, ErrNotFound
	}
	return item, nil
}

func requireOpsOrder(ctx context.Context, catalog repository.OpsRepository, orderID string) (model.Order, error) {
	item, ok, err := catalog.GetOrder(ctx, orderID)
	if err != nil {
		return model.Order{}, err
	}
	if !ok {
		return model.Order{}, ErrNotFound
	}
	return item, nil
}

func opsCustomerFromUser(ctx context.Context, catalog repository.OpsRepository, user model.User) model.Customer {
	planCode, _, _ := catalog.GetCustomerPlan(ctx, user.UserID)
	return model.Customer{
		CustomerID: user.UserID,
		UserID:     user.UserID,
		Email:      user.Email,
		Name:       user.Name,
		Country:    user.Country,
		Province:   user.Province,
		City:       user.City,
		IPRegion:   user.IPRegion,
		Status:     user.Status,
		PlanCode:   planCode,
		UpdatedAt:  user.UpdatedAt,
	}
}
