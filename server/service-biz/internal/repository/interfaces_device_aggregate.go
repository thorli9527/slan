package repository

type DeviceRepository interface {
	DeviceCoreRepository
	DeviceBootstrapRepository
	DeviceGroupRepository
}
