package service

import "github.com/slan/service-biz/internal/repository"

func newOpsOperatorID(operators repository.OperatorRepository) string {
	return repositoryID[operatorIDProvider](operators, "op", func(provider operatorIDProvider) string {
		return provider.NewOperatorID()
	})
}

func opsNodeID(catalog repository.OpsRepository, current, kind string) string {
	if current != "" {
		return current
	}
	switch kind {
	case "relay":
		return repositoryID[relayNodeIDProvider](catalog, kind, func(provider relayNodeIDProvider) string {
			return provider.NewRelayNodeID()
		})
	case "punch":
		return repositoryID[punchNodeIDProvider](catalog, kind, func(provider punchNodeIDProvider) string {
			return provider.NewPunchNodeID()
		})
	}
	return kind
}

func opsDownloadID(catalog repository.OpsRepository, current string) string {
	if current != "" {
		return current
	}
	return repositoryID[clientDownloadIDProvider](catalog, "download", func(provider clientDownloadIDProvider) string {
		return provider.NewClientDownloadID()
	})
}

func opsProductID(catalog repository.OpsRepository) string {
	return repositoryID[productIDProvider](catalog, "product", func(provider productIDProvider) string {
		return provider.NewProductID()
	})
}

func opsOrderID(catalog repository.OpsRepository) string {
	return repositoryID[orderIDProvider](catalog, "order", func(provider orderIDProvider) string {
		return provider.NewOrderID()
	})
}
