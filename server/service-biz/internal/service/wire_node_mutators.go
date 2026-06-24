package service

import "github.com/slan/service-biz/internal/model"

func relayNodeModel(item model.RelayNode, fallbackTransport string) model.RelayNode {
	if item.Transport == "" {
		item.Transport = fallbackTransport
	}
	return item
}
