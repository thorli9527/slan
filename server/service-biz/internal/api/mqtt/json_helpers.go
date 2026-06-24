package mqtt

import (
	"encoding/json"
	"net/http"
)

func jsonDecoder(r *http.Request) func(any) error {
	return func(out any) error {
		return json.NewDecoder(r.Body).Decode(out)
	}
}
