package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
)

func requestUserID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "userId", "userId")
}

func requestOwnerID(r *http.Request) string {
	return serviceapi.FirstNonEmpty(
		serviceapi.PathOrQuery(r, "userId", "ownerUserId"),
		serviceapi.PathOrQuery(r, "ownerId", "ownerId"),
	)
}

func requestNetworkID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "networkId", "networkId")
}

func requestDeviceID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "deviceId", "deviceId")
}
