package service

import (
	"testing"
	"time"
)

func TestNetworkMembershipMessageIDSeparatesDistinctChangesAtSameTime(t *testing.T) {
	now := time.Unix(1_700_000_000, 123_456_789)
	base := networkMembershipMessageID(now, "device-1", "network-1", "joined", 7)

	cases := map[string]string{
		"network":   networkMembershipMessageID(now, "device-1", "network-2", "joined", 7),
		"operation": networkMembershipMessageID(now, "device-1", "network-1", "left", 7),
		"version":   networkMembershipMessageID(now, "device-1", "network-1", "joined", 8),
		"device":    networkMembershipMessageID(now, "device-2", "network-1", "joined", 7),
	}
	for field, messageID := range cases {
		if messageID == base {
			t.Fatalf("changing %s must produce a distinct message ID", field)
		}
	}

	if duplicate := networkMembershipMessageID(now, "device-1", "network-1", "joined", 7); duplicate != base {
		t.Fatalf("identical logical changes must produce a stable message ID: got %q, want %q", duplicate, base)
	}
}
