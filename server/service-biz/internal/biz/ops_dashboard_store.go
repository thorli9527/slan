package biz

import (
	"strings"
	"time"
)

func (s *Store) OpsDashboard() map[string]any {
	customers := s.ListCustomers()
	orders := s.ListOrders()
	relayNodes := s.ListRelayNodes()
	revenueMonth := 0.0
	revenueYear := 0.0
	paidOrders := 0
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()).Unix()
	for _, order := range orders {
		if order.PayStatus != "paid" {
			continue
		}
		paidOrders++
		if order.PaidAt >= monthStart {
			revenueMonth += order.Amount
		}
		if order.PaidAt >= yearStart {
			revenueYear += order.Amount
		}
	}
	return map[string]any{
		"customers":    len(customers),
		"orders":       len(orders),
		"paidOrders":   paidOrders,
		"relayNodes":   len(relayNodes),
		"revenueMonth": revenueMonth,
		"revenueYear":  revenueYear,
	}
}

func (s *Store) customerProfileLocked(user User) CustomerProfile {
	quota := s.deviceQuotaLocked(user.UserID)
	profile := s.customerProfiles[user.UserID]
	status := quota.Status
	if strings.TrimSpace(profile.Status) != "" {
		status = profile.Status
	}
	if user.Status != "" && user.Status != "active" {
		status = user.Status
	}
	return CustomerProfile{
		CustomerID:     user.UserID,
		Email:          user.Email,
		Name:           user.Name,
		Country:        profile.Country,
		Province:       profile.Province,
		City:           profile.City,
		IPRegion:       profile.IPRegion,
		PlanCode:       quota.PlanCode,
		PlanExpiresAt:  quota.PlanExpiresAt,
		OwnDevices:     quota.OwnDevices,
		InvitedDevices: quota.InvitedDevices,
		Status:         status,
	}
}

func (s *Store) deviceQuotaLocked(userID string) DeviceQuota {
	ownDevices := 0
	for _, device := range s.devices {
		if device.OwnerID == userID {
			ownDevices++
		}
	}
	invitedDevices := 0
	for _, grant := range s.deviceAccessGrants {
		if grant.UserID == userID && grant.Status == "active" {
			invitedDevices++
		}
	}
	assignment := s.customerPlans[userID]
	planCode := defaultString(assignment.PlanCode, "free")
	plan := s.opsPlans[planCode]
	if plan.Code == "" {
		plan = s.opsPlans["free"]
		planCode = defaultString(plan.Code, "free")
	}
	total := ownDevices + invitedDevices
	remaining := plan.TotalDeviceLimit - total
	if remaining < 0 {
		remaining = 0
	}
	status := "active"
	if assignment.ExpiresAt > 0 && assignment.ExpiresAt < time.Now().Unix() {
		status = "expired"
	}
	return DeviceQuota{
		UserID:             userID,
		PlanCode:           planCode,
		PlanName:           defaultString(plan.Name, planCode),
		OwnDeviceLimit:     plan.OwnDeviceLimit,
		InvitedDeviceLimit: plan.InvitedDeviceLimit,
		TotalDeviceLimit:   plan.TotalDeviceLimit,
		OwnDevices:         ownDevices,
		InvitedDevices:     invitedDevices,
		TotalDevices:       total,
		RemainingDevices:   remaining,
		PlanExpiresAt:      assignment.ExpiresAt,
		Status:             status,
	}
}

func (s *Store) activeRelayCandidatesLocked() []RelayCandidate {
	out := make([]RelayCandidate, 0, len(s.relayNodes))
	now := time.Now().Unix()
	nodes := sortedValues(s.relayNodes, lessOpsRelayNode)
	for _, node := range nodes {
		if node.Status != "active" || node.Health != "healthy" || wireNodeStale(node, now) {
			continue
		}
		transport := node.Transport
		if transport == "relay_udp" {
			transport = "udp"
		}
		if transport != "udp" && transport != "derp_tcp_tls_443" {
			continue
		}
		out = append(out, RelayCandidate{
			EndpointID: node.NodeID,
			Transport:  transport,
			Address:    relayCandidatePublicAddress(node.PublicAddr),
			RegionID:   node.Region,
			ClusterID:  node.Region,
		})
	}
	return out
}
