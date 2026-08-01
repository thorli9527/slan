package ops

import (
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestUserPayloadUsesUserID(t *testing.T) {
	payload := userPayload(servicepkg.OpsUserView{
		User: servicepkg.OpsUserRecord{UserID: "user-1"},
	})
	if got := payload["userId"]; got != "user-1" {
		t.Fatalf("userId = %#v, want user-1", got)
	}
	if _, exists := payload["customerId"]; exists {
		t.Fatal("retired customerId field is still published")
	}
}
