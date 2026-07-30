package app

import (
	"context"

	"github.com/slan/service-biz/internal/bootstrap"
	"github.com/slan/service-biz/internal/repository"
)

type gormProviders struct {
	Repositories Repositories
	IDs          IDGenerators
}

func newGormProviders(persistence Persistence) gormProviders {
	store := newGormStore(persistence.Gorm)
	repositories := bindRepositories(store)
	bootstrap.EnsureDownloadAssets(context.Background(), repositories.Download.Catalog)
	return gormProviders{
		Repositories: repositories,
		IDs:          bindIDGenerators(store),
	}
}

func newGormStore(cfg repository.GormConfig) *repository.GormStore {
	store, err := repository.OpenGormStore(cfg)
	if err != nil {
		panic(err)
	}
	if err := bootstrap.InitializeGormStore(context.Background(), store); err != nil {
		panic(err)
	}
	return store
}

func bindRepositories(store *repository.GormStore) Repositories {
	return Repositories{
		Auth:     bindAuthRepositories(store),
		Device:   bindDeviceRepositories(store),
		Network:  bindNetworkRepositories(store),
		Ops:      bindOpsRepositories(store),
		Wire:     bindWireRepositories(store),
		MQTT:     bindMQTTRepositories(store),
		Download: bindDownloadRepositories(store),
	}
}

func bindIDGenerators(store *repository.GormStore) IDGenerators {
	return IDGenerators{
		Auth:     bindAuthIDGenerators(store),
		Device:   bindDeviceIDGenerators(store),
		Network:  bindNetworkIDGenerators(store),
		Ops:      bindOpsIDGenerators(store),
		Wire:     bindWireIDGenerators(store),
		Download: bindDownloadIDGenerators(store),
	}
}

func bindAuthRepositories(store *repository.GormStore) AuthRepositories {
	return AuthRepositories{
		Users:       store,
		Sessions:    store,
		UserAliases: store,
		Devices:     store,
	}
}

func bindDeviceRepositories(store *repository.GormStore) DeviceRepositories {
	return DeviceRepositories{
		Users:         store,
		Devices:       store,
		Relations:     store,
		Networks:      store,
		NetworkGroups: store,
	}
}

func bindNetworkRepositories(store *repository.GormStore) NetworkRepositories {
	return NetworkRepositories{
		Users:           store,
		Devices:         store,
		Relations:       store,
		Networks:        store,
		Ops:             store,
		EventDeliveries: store,
	}
}

func bindOpsRepositories(store *repository.GormStore) OpsRepositories {
	return OpsRepositories{
		Users:            store,
		Devices:          store,
		Networks:         store,
		Operators:        store,
		OperatorSessions: store,
		Audit:            store,
		Catalog:          store,
	}
}

func bindWireRepositories(store *repository.GormStore) WireRepositories {
	return WireRepositories{
		Devices:  store,
		Networks: store,
		Catalog:  store,
	}
}

func bindMQTTRepositories(store *repository.GormStore) MQTTRepositories {
	return MQTTRepositories{
		Networks:        store,
		EventDeliveries: store,
	}
}

func bindDownloadRepositories(store *repository.GormStore) DownloadRepositories {
	return DownloadRepositories{
		Catalog: store,
	}
}

func bindAuthIDGenerators(store *repository.GormStore) AuthIDGenerators {
	return AuthIDGenerators{
		NewUserID:    store.NewUserID,
		NewSessionID: store.NewSessionID,
	}
}

func bindDeviceIDGenerators(store *repository.GormStore) DeviceIDGenerators {
	return DeviceIDGenerators{
		NewDeviceID:  store.NewDeviceID,
		NewSessionID: store.NewSessionID,
	}
}

func bindNetworkIDGenerators(store *repository.GormStore) NetworkIDGenerators {
	return NetworkIDGenerators{
		NewNetworkID: store.NewNetworkID,
		NewInviteID:  store.NewInviteID,
		NewSessionID: store.NewSessionID,
	}
}

func bindOpsIDGenerators(store *repository.GormStore) OpsIDGenerators {
	return OpsIDGenerators{
		NewSessionID:  store.NewSessionID,
		NewOperatorID: store.NewOperatorID,
		NewProductID:  store.NewProductID,
		NewOrderID:    store.NewOrderID,
	}
}

func bindWireIDGenerators(store *repository.GormStore) WireIDGenerators {
	return WireIDGenerators{
		NewRelayNodeID: store.NewRelayNodeID,
		NewPunchNodeID: store.NewPunchNodeID,
	}
}

func bindDownloadIDGenerators(store *repository.GormStore) DownloadIDGenerators {
	return DownloadIDGenerators{
		NewDownloadID: store.NewClientDownloadID,
	}
}
