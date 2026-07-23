package service

import (
	"context"
	"errors"
	"testing"

	"github.com/slan/service-biz/internal/model"
)

func TestValidateManagedDNSRecordTargetOnlyAcceptsCurrentNetworkDevice(t *testing.T) {
	networks := &networkAccessTestNetworks{
		networkDevices: map[string][]model.NetworkDevice{
			"network-1": {
				{
					NetworkID:    "network-1",
					DeviceID:     "device-1",
					Enabled:      true,
					MemberStatus: model.NetworkMemberStatusActive,
				},
			},
		},
	}

	if err := validateManagedDNSRecordTarget(
		context.Background(), networks, "network-1", "A", "device-1",
	); err != nil {
		t.Fatalf("expected current network device target to pass: %v", err)
	}

	for _, target := range []string{"10.0.1.2", "device-from-another-network"} {
		err := validateManagedDNSRecordTarget(
			context.Background(), networks, "network-1", "A", target,
		)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("expected target %q to be rejected, got %v", target, err)
		}
	}
}
