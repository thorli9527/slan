package api

import (
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestAuthSessionPayloadDoesNotInferUserNetwork(t *testing.T) {
	payload := AuthSessionPayload(servicepkg.AuthSessionView{
		User: servicepkg.UserView{UserID: "user-1", Email: "user@example.com"},
		Session: servicepkg.UserSessionView{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		},
	})

	if _, ok := payload["defaultNetwork"]; ok {
		t.Fatal("authentication payload must not infer a default network from user ownership")
	}
	if _, ok := payload["activeNetworkId"]; ok {
		t.Fatal("authentication payload must not select an active network")
	}
}
