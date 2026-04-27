package impl

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

const entitlementSyncInterval = 30 * time.Second

type computedUserEntitlement struct {
	availableDeviceCount int
	dnsAvailable         bool
}

func (s *dbState) startEntitlementSyncLoop() {
	go func() {
		ticker := time.NewTicker(entitlementSyncInterval)
		defer ticker.Stop()
		for {
			s.syncAllUserEntitlements(context.Background(), "periodic entitlement sync")
			<-ticker.C
		}
	}()
}

func (s *dbState) syncAllUserEntitlements(ctx context.Context, reason string) {
	users, err := s.pg.ListUsers(ctx)
	if err != nil {
		return
	}
	for _, user := range users {
		_ = s.syncUserEntitlements(ctx, user.UserID, reason)
	}
}

func (s *dbState) syncUserEntitlements(ctx context.Context, userID, reason string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	user, err := s.pg.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	bindingsChanged, err := s.reconcileUserDeviceOrderBindings(ctx, userID, now)
	if err != nil {
		return err
	}
	entitlement, err := s.computeUserEntitlement(ctx, userID)
	if err != nil {
		return err
	}
	entitlement.availableDeviceCount = minimumAvailableDeviceCount(entitlement.availableDeviceCount)
	if !bindingsChanged && user.AvailableDeviceCount == entitlement.availableDeviceCount && user.DNSAvailable == entitlement.dnsAvailable {
		s.enforceUserDeviceLimit(ctx, userID, entitlement.availableDeviceCount)
		return nil
	}
	if err := s.pg.UpdateUserEntitlements(ctx, userID, entitlement.availableDeviceCount, entitlement.dnsAvailable, now); err != nil {
		return err
	}
	s.enforceUserDeviceLimit(ctx, userID, entitlement.availableDeviceCount)
	networkID := strings.TrimSpace(user.ActiveNetworkID)
	s.publishUserEntitlementChanged(userID, networkID, entitlement.availableDeviceCount, entitlement.dnsAvailable, reason)
	if networkID != "" {
		s.publishNetworkRestartRequired(networkID, "")
	}
	return nil
}

func (s *dbState) computeUserEntitlement(ctx context.Context, userID string) (computedUserEntitlement, error) {
	product := s.currentDefaultProduct(ctx)
	out := computedUserEntitlement{
		availableDeviceCount: product.MaxActiveDevices,
	}
	out.availableDeviceCount = minimumAvailableDeviceCount(out.availableDeviceCount)
	orders, err := s.pg.ListPurchaseOrdersByUser(ctx, userID)
	if err != nil {
		return out, err
	}
	paidDeviceOrders := s.paidDeviceOrderIDs(ctx, orders)
	bindings, err := s.pg.ListPurchaseOrderDeviceBindingsByUser(ctx, userID)
	if err != nil {
		return out, err
	}
	now := time.Now().Unix()
	usedSlotsByOrder := map[string]int{}
	activeDeviceSlots := 0
	for _, binding := range bindings {
		if !paidDeviceOrders[binding.OrderID] {
			continue
		}
		usedSlotsByOrder[binding.OrderID]++
		if binding.ExpiresAt > now {
			activeDeviceSlots++
		}
	}
	for _, order := range orders {
		if !isPaidPurchaseOrderStatus(order.Status) {
			continue
		}
		productType := strings.TrimSpace(strings.ToLower(order.ProductType))
		if productType == "" {
			if product, err := s.pg.GetProductByCode(ctx, order.ProductCode); err == nil {
				productType = strings.TrimSpace(strings.ToLower(product.ProductType))
			}
		}
		switch productType {
		case "addon_device":
			capacity := s.deviceOrderCapacity(ctx, order)
			unboundSlots := capacity - usedSlotsByOrder[order.OrderID]
			if unboundSlots > 0 {
				out.availableDeviceCount += unboundSlots
			}
		case "addon_dns":
			if purchaseOrderExpiresAt(order) <= now {
				continue
			}
			out.dnsAvailable = true
		}
	}
	out.availableDeviceCount += activeDeviceSlots
	return out, nil
}

func minimumAvailableDeviceCount(count int) int {
	if count < defaultFreeMaxActiveDevices {
		return defaultFreeMaxActiveDevices
	}
	return count
}

func (s *dbState) enforceUserDeviceLimit(ctx context.Context, userID string, limit int) {
	if limit < 0 {
		limit = 0
	}
	attachments, err := s.pg.ListActiveAttachmentsByUser(ctx, userID)
	if err != nil || len(attachments) <= limit {
		return
	}
	for _, attachment := range attachments[limit:] {
		if err := s.pg.SuspendAttachment(ctx, attachment.AttachmentID); err != nil {
			continue
		}
		s.publishDeviceIPReassigned(attachment.NetworkID, attachment.DeviceID, attachment.AttachmentID, "")
	}
}

func (s *dbState) reconcileUserDeviceOrderBindings(ctx context.Context, userID string, now int64) (bool, error) {
	attachments, err := s.pg.ListActiveAttachmentsByUser(ctx, userID)
	if err != nil || len(attachments) <= defaultFreeMaxActiveDevices {
		return false, err
	}
	orders, err := s.pg.ListPurchaseOrdersByUser(ctx, userID)
	if err != nil {
		return false, err
	}
	paidDeviceOrders := s.paidDeviceOrderIDs(ctx, orders)
	bindings, err := s.pg.ListPurchaseOrderDeviceBindingsByUser(ctx, userID)
	if err != nil {
		return false, err
	}
	activeBindingByDevice := map[string]repo.PurchaseOrderDeviceBinding{}
	usedSlotsByOrder := map[string]int{}
	for _, binding := range bindings {
		if !paidDeviceOrders[binding.OrderID] {
			continue
		}
		usedSlotsByOrder[binding.OrderID]++
		if binding.ExpiresAt > now {
			activeBindingByDevice[binding.DeviceID] = binding
		}
	}
	changed := false
	for _, attachment := range attachments[defaultFreeMaxActiveDevices:] {
		if _, ok := activeBindingByDevice[attachment.DeviceID]; ok {
			continue
		}
		order, ok := s.nextAssignableDeviceOrder(ctx, orders, usedSlotsByOrder)
		if !ok {
			continue
		}
		expiresAt := purchaseOrderExpiresAtForMonths(now, order.BillingCycle, maxInt(order.Months, 1))
		binding := repo.PurchaseOrderDeviceBinding{
			BindingID: fmt.Sprintf("odb:%s:%s:%s", order.OrderID, attachment.DeviceID, util.NewID("bind")),
			OrderID:   order.OrderID,
			UserID:    userID,
			DeviceID:  attachment.DeviceID,
			BoundAt:   now,
			ExpiresAt: expiresAt,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.pg.CreatePurchaseOrderDeviceBinding(ctx, binding); err != nil {
			return changed, err
		}
		changed = true
		usedSlotsByOrder[order.OrderID]++
		activeBindingByDevice[attachment.DeviceID] = binding
	}
	return changed, nil
}

func (s *dbState) nextAssignableDeviceOrder(ctx context.Context, orders []repo.PurchaseOrder, usedSlotsByOrder map[string]int) (repo.PurchaseOrder, bool) {
	for _, order := range oldestPurchaseOrdersFirst(orders) {
		if !isPaidPurchaseOrderStatus(order.Status) {
			continue
		}
		productType := strings.TrimSpace(strings.ToLower(order.ProductType))
		if productType == "" {
			if product, err := s.pg.GetProductByCode(ctx, order.ProductCode); err == nil {
				productType = strings.TrimSpace(strings.ToLower(product.ProductType))
			}
		}
		if productType != "addon_device" {
			continue
		}
		if usedSlotsByOrder[order.OrderID] >= s.deviceOrderCapacity(ctx, order) {
			continue
		}
		return order, true
	}
	return repo.PurchaseOrder{}, false
}

func oldestPurchaseOrdersFirst(orders []repo.PurchaseOrder) []repo.PurchaseOrder {
	out := append([]repo.PurchaseOrder(nil), orders...)
	sort.SliceStable(out, func(i, j int) bool {
		left := purchaseOrderSortTime(out[i])
		right := purchaseOrderSortTime(out[j])
		if left == right {
			return out[i].OrderID < out[j].OrderID
		}
		return left < right
	})
	return out
}

func purchaseOrderSortTime(order repo.PurchaseOrder) int64 {
	if order.PaidAt > 0 {
		return order.PaidAt
	}
	if order.CreatedAt > 0 {
		return order.CreatedAt
	}
	return order.UpdatedAt
}

func (s *dbState) deviceOrderCapacity(ctx context.Context, order repo.PurchaseOrder) int {
	unitQuantity := 1
	if product, err := s.pg.GetProductByCode(ctx, order.ProductCode); err == nil && product.UnitQuantity > 0 {
		unitQuantity = product.UnitQuantity
	}
	quantity := order.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	return unitQuantity * quantity
}

func (s *dbState) paidDeviceOrderIDs(ctx context.Context, orders []repo.PurchaseOrder) map[string]bool {
	out := map[string]bool{}
	for _, order := range orders {
		if !isPaidPurchaseOrderStatus(order.Status) {
			continue
		}
		productType := strings.TrimSpace(strings.ToLower(order.ProductType))
		if productType == "" {
			if product, err := s.pg.GetProductByCode(ctx, order.ProductCode); err == nil {
				productType = strings.TrimSpace(strings.ToLower(product.ProductType))
			}
		}
		if productType == "addon_device" {
			out[order.OrderID] = true
		}
	}
	return out
}

func (s *dbState) publishUserEntitlementChanged(userID, networkID string, availableDeviceCount int, dnsAvailable bool, reason string) {
	if strings.TrimSpace(userID) == "" || s.tokens == nil {
		return
	}
	_ = s.tokens.PublishControlSyncEvent(context.Background(), controlmsg.ControlSyncEvent{
		Type:         "user_entitlement_changed",
		NetworkID:    networkID,
		TargetUserID: userID,
		Entitlement: &controlmsg.UserEntitlementChanged{
			UserID:               userID,
			NetworkID:            networkID,
			AvailableDeviceCount: availableDeviceCount,
			DNSAvailable:         dnsAvailable,
			Reason:               reason,
		},
	})
}

func (s *dbState) syncOrderUserEntitlement(ctx context.Context, order repo.PurchaseOrder, reason string) {
	if strings.TrimSpace(order.UserID) == "" {
		return
	}
	_ = s.syncUserEntitlements(ctx, order.UserID, reason)
}
