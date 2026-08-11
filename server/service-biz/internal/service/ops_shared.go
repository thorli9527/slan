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
	Audit             repository.AuditRepository
	NewOperatorSessID func(string) string
	Now               func() time.Time
}

type OpsOperatorService struct {
	Operators        repository.OperatorRepository
	OperatorSessions repository.OperatorSessionRepository
	Audit            repository.AuditRepository
	Now              func() time.Time
}

type OpsOperatorPasswordService struct {
	Operators        repository.OperatorRepository
	OperatorSessions repository.OperatorSessionRepository
	Audit            repository.AuditRepository
	Now              func() time.Time
}

type OpsDashboardService struct {
	Customers repository.CustomerRepository
	Devices   repository.DeviceRepository
	Inventory repository.DeviceInventoryRepository
	Networks  repository.NetworkRepository
	Operators repository.OperatorRepository
}

type OpsAuditService struct {
	Audit repository.AuditRepository
}

type RuntimeNodeRegistryService struct {
	Nodes repository.RuntimeNodeRepository
	Now   func() time.Time
}

type OpsCustomerService struct {
	Customers     repository.CustomerRepository
	NewCustomerID func() string
	Now           func() time.Time
}

type OpsManagedDeviceService struct {
	Devices         repository.DeviceRepository
	Inventory       repository.DeviceInventoryRepository
	Networks        repository.NetworkRepository
	Audit           repository.AuditRepository
	Credentials     repository.DeviceCredentialRepository
	EventPublisher  NetworkEventPublisher
	DevicePublisher DeviceControlPublisher
	DeviceRuntime   repository.DeviceRuntimeRepository
	NewDeviceID     func() string
	Now             func() time.Time
}

func opsNow(now func() time.Time) time.Time {
	return currentTime(now)
}

func requireOpsCustomer(ctx context.Context, customers repository.CustomerRepository, customerID string) (model.Customer, error) {
	customer, ok, err := customers.GetCustomer(ctx, customerID)
	if err != nil {
		return model.Customer{}, err
	}
	if !ok {
		return model.Customer{}, ErrNotFound
	}
	return customer, nil
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
