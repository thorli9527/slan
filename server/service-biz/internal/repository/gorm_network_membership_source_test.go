package repository

import (
	"testing"

	"github.com/slan/service-biz/internal/model"
)

func TestNetworkDeviceMembershipSourceRoundTrip(t *testing.T) {
	for _, source := range []model.NetworkMembershipSource{
		model.NetworkMembershipSourceDirect,
		model.NetworkMembershipSourceDeviceGroup,
		model.NetworkMembershipSourceExcluded,
	} {
		input := model.NetworkDevice{
			NetworkID: "net-1", DeviceID: "dev-1", Enabled: source != model.NetworkMembershipSourceExcluded,
			MemberStatus: model.NetworkMemberStatusActive, MembershipSource: source,
		}
		if source == model.NetworkMembershipSourceExcluded {
			input.MemberStatus = model.NetworkMemberStatusRemoved
		}
		got := networkDeviceRecordFromModel(input).model()
		if got.MembershipSource != source {
			t.Fatalf("source %q was not preserved: %+v", source, got)
		}
		if got.Enabled != input.Enabled || got.MemberStatus != input.MemberStatus {
			t.Fatalf("membership state changed during round trip: input=%+v got=%+v", input, got)
		}
	}
}
