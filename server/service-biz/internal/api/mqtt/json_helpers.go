package mqtt

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
)

const maxMQTTWebhookBodyBytes int64 = 256 << 10

func jsonDecoder(r *http.Request) func(any) error {
	return func(out any) error {
		return serviceapi.DecodeJSONWithLimit(r, out, maxMQTTWebhookBodyBytes)
	}
}
