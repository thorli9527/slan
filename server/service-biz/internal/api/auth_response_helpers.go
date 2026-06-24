package api

import (
	"context"

	authpayload "github.com/slan/service-biz/internal/api/authpayload"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type authNetworkReader interface {
	ListNetworks(ctx context.Context, ownerID string) ([]servicepkg.NetworkSummaryView, error)
}

func AuthSessionPayload(ctx context.Context, networks authNetworkReader, view servicepkg.AuthSessionView) map[string]any {
	return authpayload.SessionWithDefaultNetwork(view, defaultNetworkSummary(ctx, networks, view.User.UserID))
}

func AppAuthSessionPayload(ctx context.Context, networks authNetworkReader, view servicepkg.AuthSessionView) map[string]any {
	return AuthSessionPayload(ctx, networks, view)
}

func defaultNetworkSummary(ctx context.Context, networks authNetworkReader, userID string) *servicepkg.NetworkSummaryView {
	if networks == nil {
		return nil
	}
	items, err := networks.ListNetworks(ctx, userID)
	if err != nil || len(items) == 0 {
		return nil
	}
	return &items[0]
}
