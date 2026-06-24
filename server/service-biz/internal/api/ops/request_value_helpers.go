package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
)

func firstNonEmpty(values ...string) string {
	return serviceapi.FirstNonEmpty(values...)
}

func requestOperatorID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "operatorId", "operatorId")
}

func requestCustomerID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "customerId", "customerId")
}

func requestDeviceID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "deviceId", "deviceId")
}

func requestNodeID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "nodeId", "nodeId")
}

func requestDownloadID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "downloadId", "downloadId")
}

func requestPlanCode(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "planCode", "planCode")
}

func requestProductID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "productId", "productId")
}

func requestOrderID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "orderId", "orderId")
}

func requestRenewalID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "renewalId", "renewalId")
}
