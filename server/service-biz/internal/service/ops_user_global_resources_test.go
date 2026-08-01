package service

import (
	"context"
	"testing"
	"time"
)

func TestOpsCreateUserDoesNotCreateNetworkResources(t *testing.T) {
	users := &authUserRegistrationTestUsers{}
	devices := &authUserRegistrationTestDevices{}
	service := OpsUserService{
		Users:     users,
		Devices:   devices,
		NewUserID: func() string { return "ops-user-1" },
		Now:       func() time.Time { return time.Unix(1_700_000_000, 0) },
	}

	view, err := service.CreateUser(context.Background(), CreateUserInput{
		Email: "ops-created@example.com", Password: "Password123!", Name: "Ops Created",
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.User.UserID != "ops-user-1" || view.User.Email != "ops-created@example.com" {
		t.Fatalf("unexpected created user: %+v", view)
	}
	groups, err := devices.ListDeviceGroups(context.Background(), "ops-user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("ops user creation created device groups: %+v", groups)
	}
}
