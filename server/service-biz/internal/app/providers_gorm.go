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
		Device:  bindDeviceRepositories(store),
		Network: bindNetworkRepositories(store),
		Ops:     bindOpsRepositories(store),
		Wire:    bindWireRepositories(store),
		MQTT:    bindMQTTRepositories(store),
	}
}

func bindIDGenerators(store *repository.GormStore) IDGenerators {
	return IDGenerators{
		Device:  bindDeviceIDGenerators(store),
		Network: bindNetworkIDGenerators(store),
		Ops:     bindOpsIDGenerators(store),
		Wire:    bindWireIDGenerators(store),
	}
}

func bindDeviceRepositories(store *repository.GormStore) DeviceRepositories {
	return DeviceRepositories{
		Devices:       store,
		Credentials:   store,
		Audit:         store,
		Networks:      store,
		NetworkGroups: store,
	}
}

func bindNetworkRepositories(store *repository.GormStore) NetworkRepositories {
	return NetworkRepositories{
		Devices:         store,
		Networks:        store,
		Ops:             store,
		EventDeliveries: store,
	}
}

func bindOpsRepositories(store *repository.GormStore) OpsRepositories {
	return OpsRepositories{
		Customers:        store,
		Devices:          store,
		DeviceInventory:  store,
		Credentials:      store,
		Networks:         store,
		NetworkGroups:    store,
		Operators:        store,
		OperatorSessions: store,
		Audit:            store,
		Nodes:            store,
	}
}

func bindWireRepositories(store *repository.GormStore) WireRepositories {
	return WireRepositories{
		Devices:  store,
		Networks: store,
		Nodes:    store,
	}
}

func bindMQTTRepositories(store *repository.GormStore) MQTTRepositories {
	return MQTTRepositories{
		Devices:         store,
		Networks:        store,
		Credentials:     store,
		EventDeliveries: store,
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
		NewSessionID: store.NewSessionID,
	}
}

func bindOpsIDGenerators(store *repository.GormStore) OpsIDGenerators {
	return OpsIDGenerators{
		NewSessionID:  store.NewSessionID,
		NewOperatorID: store.NewOperatorID,
		NewCustomerID: store.NewCustomerID,
	}
}

func bindWireIDGenerators(store *repository.GormStore) WireIDGenerators {
	return WireIDGenerators{
		NewRelayNodeID: store.NewRelayNodeID,
		NewPunchNodeID: store.NewPunchNodeID,
	}
}
