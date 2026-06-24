package repository

type NetworkRepository interface {
	NetworkCoreRepository
	NetworkDNSRepository
	NetworkSecurityRepository
}
