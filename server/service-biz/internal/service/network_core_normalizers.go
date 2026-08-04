package service

import "strings"

func normalizeNetworkID(networkID string) string {
	return strings.TrimSpace(networkID)
}
