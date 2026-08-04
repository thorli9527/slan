package repository

type DeviceRepository interface {
	DeviceCoreRepository
	DeviceGroupRepository
}
