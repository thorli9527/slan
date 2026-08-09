package service

import "github.com/slan/service-biz/internal/repository"

func newOpsOperatorID(operators repository.OperatorRepository) string {
	return repositoryID[operatorIDProvider](operators, "op", func(provider operatorIDProvider) string {
		return provider.NewOperatorID()
	})
}

func runtimeNodeID(nodes repository.RuntimeNodeRepository, current, kind string) string {
	if current != "" {
		return current
	}
	switch kind {
	case "relay":
		return repositoryID[relayNodeIDProvider](nodes, kind, func(provider relayNodeIDProvider) string {
			return provider.NewRelayNodeID()
		})
	case "punch":
		return repositoryID[punchNodeIDProvider](nodes, kind, func(provider punchNodeIDProvider) string {
			return provider.NewPunchNodeID()
		})
	}
	return kind
}
