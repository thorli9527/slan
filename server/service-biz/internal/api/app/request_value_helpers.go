package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
)

func requestNetworkID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "networkId", "networkId")
}

func requestDeviceID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "deviceId", "deviceId")
}
