package impl

import (
	"context"
	"testing"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func TestOpsOverviewCountsFreshNetworkOnlineDevices(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now()

	for _, device := range []repo.Device{
		{DeviceID: "dev-online", UserID: "user-1", MachineID: "machine-1", Name: "online", Platform: "windows", Status: "reachable", CreatedAt: now.Unix()},
		{DeviceID: "dev-reachable", UserID: "user-1", MachineID: "machine-2", Name: "reachable", Platform: "windows", Status: "reachable", CreatedAt: now.Unix()},
		{DeviceID: "dev-stale", UserID: "user-1", MachineID: "machine-3", Name: "stale", Platform: "windows", Status: "online", CreatedAt: now.Unix()},
	} {
		if err := state.pg.InsertDevice(ctx, device); err != nil {
			t.Fatalf("insert device %s: %v", device.DeviceID, err)
		}
	}
	for _, stateRecord := range []repo.DeviceNetworkState{
		{DeviceID: "dev-online", NetworkID: "net-1", ControlReachable: true, NetworkOnline: true, TunnelUp: true, LastProbeOK: true, LastSeenAt: now.Unix(), UpdatedAt: now.Unix()},
		{DeviceID: "dev-reachable", NetworkID: "net-1", ControlReachable: true, NetworkOnline: false, TunnelUp: false, LastProbeOK: false, LastSeenAt: now.Unix(), UpdatedAt: now.Unix()},
		{DeviceID: "dev-stale", NetworkID: "net-1", ControlReachable: true, NetworkOnline: true, TunnelUp: true, LastProbeOK: true, LastSeenAt: now.Add(-2 * time.Minute).Unix(), UpdatedAt: now.Add(-2 * time.Minute).Unix()},
	} {
		if err := state.pg.UpsertDeviceNetworkState(ctx, stateRecord); err != nil {
			t.Fatalf("upsert state for %s: %v", stateRecord.DeviceID, err)
		}
	}

	overview, err := (dbOpsService{state: state}).Overview()
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.OnlineDeviceCount != 1 {
		t.Fatalf("expected only fresh network-online device to be counted, got %+v", overview)
	}
}

func TestOpsProductManagementSeedsFreeProduct(t *testing.T) {
	state := newNetworkTestState(t)
	if err := state.seedDefaultProducts(context.Background()); err != nil {
		t.Fatalf("seed default products: %v", err)
	}

	items, err := (dbOpsService{state: state}).ListProducts()
	if err != nil {
		t.Fatalf("list products: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("expected seeded free, extra-device monthly/quarterly/yearly, and dns products, got %+v", items)
	}
	var free, extraDevice, extraDeviceQuarter, extraDeviceYear, dns repo.Product
	for _, item := range items {
		switch item.ProductCode {
		case "free":
			free = repo.Product{ProductCode: item.ProductCode, IsDefault: item.IsDefault, MaxActiveDevices: item.MaxActiveDevices, BandwidthLimitMbps: item.BandwidthLimitMbps}
		case "extra-device":
			extraDevice = repo.Product{ProductCode: item.ProductCode, ProductType: item.ProductType, PriceCents: item.PriceCents, BillingCycle: item.BillingCycle}
		case "extra-device-quarter":
			extraDeviceQuarter = repo.Product{ProductCode: item.ProductCode, ProductType: item.ProductType, PriceCents: item.PriceCents, BillingCycle: item.BillingCycle}
		case "extra-device-year":
			extraDeviceYear = repo.Product{ProductCode: item.ProductCode, ProductType: item.ProductType, PriceCents: item.PriceCents, BillingCycle: item.BillingCycle}
		case "dns":
			dns = repo.Product{ProductCode: item.ProductCode, ProductType: item.ProductType, PriceCents: item.PriceCents}
		}
	}
	if free.ProductCode != "free" || !free.IsDefault || free.MaxActiveDevices != 2 || free.BandwidthLimitMbps != 1 {
		t.Fatalf("unexpected free product defaults: %+v", free)
	}
	if extraDevice.ProductType != "addon_device" || extraDevice.PriceCents != 1000 || extraDevice.BillingCycle != "month" {
		t.Fatalf("unexpected extra device product defaults: %+v", extraDevice)
	}
	if extraDeviceQuarter.ProductType != "addon_device" || extraDeviceQuarter.PriceCents != 3000 || extraDeviceQuarter.BillingCycle != "quarter" {
		t.Fatalf("unexpected extra device quarter product defaults: %+v", extraDeviceQuarter)
	}
	if extraDeviceYear.ProductType != "addon_device" || extraDeviceYear.PriceCents != 12000 || extraDeviceYear.BillingCycle != "year" {
		t.Fatalf("unexpected extra device year product defaults: %+v", extraDeviceYear)
	}
	if dns.ProductType != "addon_dns" || dns.PriceCents != 1000 {
		t.Fatalf("unexpected dns product defaults: %+v", dns)
	}
}

func TestOpsProductManagementUpdatesExistingProductByCode(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.seedDefaultProducts(ctx); err != nil {
		t.Fatalf("seed default products: %v", err)
	}
	ops := dbOpsService{state: state}
	updated, err := ops.UpsertProduct(dto.UpsertProductRequest{
		ProductCode:        "extra-device",
		ProductName:        "Extra Device Updated",
		Description:        "updated",
		ProductType:        "addon_device",
		PriceCents:         1500,
		Currency:           "CNY",
		BillingCycle:       "month",
		UnitQuantity:       1,
		MaxActiveDevices:   0,
		BandwidthLimitMbps: 0,
		Status:             "active",
	})
	if err != nil {
		t.Fatalf("update product by code: %v", err)
	}
	if updated.ProductName != "Extra Device Updated" || updated.PriceCents != 1500 {
		t.Fatalf("product was not updated: %+v", updated)
	}
	items, err := ops.ListProducts()
	if err != nil {
		t.Fatalf("list products: %v", err)
	}
	count := 0
	for _, item := range items {
		if item.ProductCode == "extra-device" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one extra-device product after update, got %d in %+v", count, items)
	}
}

func TestCreatePurchaseOrderForExtraDeviceProduct(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.seedDefaultProducts(ctx); err != nil {
		t.Fatalf("seed products: %v", err)
	}

	order, err := (dbNetworkService{state: state}).CreatePurchaseOrder("user-1", dto.CreatePurchaseOrderRequest{
		ProductCode: "extra-device",
		Quantity:    2,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.ProductCode != "extra-device" || order.Quantity != 2 || order.Months != 1 || order.AmountCents != 2000 || order.Status != "pending" {
		t.Fatalf("unexpected order: %+v", order)
	}
	if order.MerchantID != defaultMerchantID || order.MerchantName != defaultMerchantName || order.UnitCents != 1000 {
		t.Fatalf("order should snapshot merchant and unit price: %+v", order)
	}

	ops := dbOpsService{state: state}
	paid, err := ops.UpdatePurchaseOrderStatus(order.OrderID, dto.UpdatePurchaseOrderStatusRequest{Status: "paid"})
	if err != nil {
		t.Fatalf("mark order paid: %v", err)
	}
	if paid.Status != "paid" || paid.UserEmail != "user@local.slan" || paid.ExpiresAt != 0 || paid.PaidAt == 0 {
		t.Fatalf("paid device order should wait for device binding before expiry starts: %+v", paid)
	}
	user, err := state.pg.GetUserByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("get synced user: %v", err)
	}
	if user.AvailableDeviceCount != 4 {
		t.Fatalf("expected two free devices plus two purchased devices, got %+v", user)
	}

	quarterOrder, err := (dbNetworkService{state: state}).CreatePurchaseOrder("user-1", dto.CreatePurchaseOrderRequest{
		ProductCode: "extra-device-quarter",
		Quantity:    1,
	})
	if err != nil {
		t.Fatalf("create quarter order: %v", err)
	}
	paidQuarter, err := ops.UpdatePurchaseOrderStatus(quarterOrder.OrderID, dto.UpdatePurchaseOrderStatusRequest{Status: "paid"})
	if err != nil {
		t.Fatalf("mark quarter order paid: %v", err)
	}
	if paidQuarter.AmountCents != 3000 || paidQuarter.BillingCycle != "quarter" || paidQuarter.ExpiresAt != 0 {
		t.Fatalf("quarter device order should use quarterly price and wait for device binding: %+v", paidQuarter)
	}
}

func TestOpsCreatePaidOrderUpdatesUserEntitlement(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-2",
		Email:        "paid@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.seedDefaultProducts(ctx); err != nil {
		t.Fatalf("seed products: %v", err)
	}
	order, err := (dbOpsService{state: state}).CreatePaidPurchaseOrder(dto.OpsCreatePaidOrderRequest{
		UserID:      "user-2",
		ProductCode: "extra-device",
		Quantity:    3,
		Months:      2,
	})
	if err != nil {
		t.Fatalf("create paid order: %v", err)
	}
	if order.Status != "paid" || order.Quantity != 3 || order.Months != 2 || order.AmountCents != 6000 || order.ExpiresAt != 0 {
		t.Fatalf("unexpected paid order: %+v", order)
	}
	user, err := state.pg.GetUserByID(ctx, "user-2")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.AvailableDeviceCount != 5 {
		t.Fatalf("expected 2 free + 3 paid devices, got %+v", user)
	}

	dnsOrder, err := (dbOpsService{state: state}).CreatePaidPurchaseOrder(dto.OpsCreatePaidOrderRequest{
		UserID:      "user-2",
		ProductCode: "dns",
		Months:      3,
	})
	if err != nil {
		t.Fatalf("create paid dns order: %v", err)
	}
	if dnsOrder.Quantity != 1 || dnsOrder.Months != 3 || dnsOrder.AmountCents != 3000 {
		t.Fatalf("unexpected dns amount: %+v", dnsOrder)
	}
	renewedDNSOrder, err := (dbOpsService{state: state}).CreatePaidPurchaseOrder(dto.OpsCreatePaidOrderRequest{
		UserID:      "user-2",
		ProductCode: "dns",
		Quantity:    99,
		Months:      2,
	})
	if err != nil {
		t.Fatalf("create renewed paid dns order: %v", err)
	}
	if renewedDNSOrder.Quantity != 1 || renewedDNSOrder.Months != 2 || renewedDNSOrder.ExpiresAt <= dnsOrder.ExpiresAt {
		t.Fatalf("dns renewal should force quantity to 1 and extend from existing expiry: first=%+v renewed=%+v", dnsOrder, renewedDNSOrder)
	}
	user, err = state.pg.GetUserByID(ctx, "user-2")
	if err != nil {
		t.Fatalf("get user after dns: %v", err)
	}
	if !user.DNSAvailable {
		t.Fatalf("expected dns available after paid dns order: %+v", user)
	}
}

func TestPaidDeviceOrderBindsWhenExtraDeviceNeedsEntitlement(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now().Unix()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-bind",
		Email:        "bind@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.seedDefaultProducts(ctx); err != nil {
		t.Fatalf("seed products: %v", err)
	}
	createNetworkFixture(t, state, "user-bind", "net-bind", "subnet-bind", "10.77.0.0/24")
	for index, deviceID := range []string{"dev-free-1", "dev-free-2", "dev-paid"} {
		if err := state.pg.InsertDevice(ctx, repo.Device{
			DeviceID:  deviceID,
			UserID:    "user-bind",
			MachineID: deviceID,
			Name:      deviceID,
			Platform:  "windows",
			Status:    "online",
			CreatedAt: now + int64(index),
		}); err != nil {
			t.Fatalf("insert device %s: %v", deviceID, err)
		}
		if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
			AttachmentID: "att-" + deviceID,
			NetworkID:    "net-bind",
			SubnetID:     "subnet-bind",
			DeviceID:     deviceID,
			VirtualIP:    "10.77.0." + string(rune('2'+index)),
			Status:       "active",
		}); err != nil {
			t.Fatalf("attach device %s: %v", deviceID, err)
		}
	}

	order, err := (dbOpsService{state: state}).CreatePaidPurchaseOrder(dto.OpsCreatePaidOrderRequest{
		UserID:      "user-bind",
		ProductCode: "extra-device",
		Quantity:    1,
		Months:      1,
	})
	if err != nil {
		t.Fatalf("create paid order: %v", err)
	}
	if order.ExpiresAt <= order.CreatedAt {
		t.Fatalf("device order should expose binding expiry after it is assigned: %+v", order)
	}
	bindings, err := state.pg.ListPurchaseOrderDeviceBindingsByUser(ctx, "user-bind")
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].OrderID != order.OrderID || bindings[0].DeviceID != "dev-paid" {
		t.Fatalf("expected extra device to consume the paid order, got %+v", bindings)
	}
	if bindings[0].BoundAt < now || bindings[0].ExpiresAt-bindings[0].BoundAt < 29*24*60*60 {
		t.Fatalf("binding expiry should start at bind time, got %+v", bindings[0])
	}
	user, err := state.pg.GetUserByID(ctx, "user-bind")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.AvailableDeviceCount != 3 {
		t.Fatalf("expected 2 free + 1 bound paid device, got %+v", user)
	}
}

func TestDeviceOrderBindingConsumesOldestPaidOrderFirst(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now().Unix()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-fifo",
		Email:        "fifo@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.seedDefaultProducts(ctx); err != nil {
		t.Fatalf("seed products: %v", err)
	}
	product, err := state.pg.GetProductByCode(ctx, "extra-device")
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	createNetworkFixture(t, state, "user-fifo", "net-fifo", "subnet-fifo", "10.76.0.0/24")
	for index, deviceID := range []string{"dev-fifo-free-1", "dev-fifo-free-2", "dev-fifo-paid"} {
		if err := state.pg.InsertDevice(ctx, repo.Device{
			DeviceID:  deviceID,
			UserID:    "user-fifo",
			MachineID: deviceID,
			Name:      deviceID,
			Platform:  "windows",
			Status:    "online",
			CreatedAt: now + int64(index),
		}); err != nil {
			t.Fatalf("insert device %s: %v", deviceID, err)
		}
		if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
			AttachmentID: "att-" + deviceID,
			NetworkID:    "net-fifo",
			SubnetID:     "subnet-fifo",
			DeviceID:     deviceID,
			VirtualIP:    "10.76.0." + string(rune('2'+index)),
			Status:       "active",
		}); err != nil {
			t.Fatalf("attach device %s: %v", deviceID, err)
		}
	}
	for _, order := range []repo.PurchaseOrder{
		{
			OrderID:      "order-fifo-new",
			UserID:       "user-fifo",
			UserEmail:    "fifo@local.slan",
			MerchantID:   defaultMerchantID,
			MerchantName: defaultMerchantName,
			ProductID:    product.ProductID,
			ProductCode:  product.ProductCode,
			ProductName:  product.ProductName,
			ProductType:  product.ProductType,
			Quantity:     1,
			Months:       1,
			UnitCents:    product.PriceCents,
			AmountCents:  product.PriceCents,
			Currency:     product.Currency,
			BillingCycle: product.BillingCycle,
			Status:       "paid",
			PaidAt:       now - 60,
			CreatedAt:    now - 60,
			UpdatedAt:    now - 60,
		},
		{
			OrderID:      "order-fifo-old",
			UserID:       "user-fifo",
			UserEmail:    "fifo@local.slan",
			MerchantID:   defaultMerchantID,
			MerchantName: defaultMerchantName,
			ProductID:    product.ProductID,
			ProductCode:  product.ProductCode,
			ProductName:  product.ProductName,
			ProductType:  product.ProductType,
			Quantity:     1,
			Months:       1,
			UnitCents:    product.PriceCents,
			AmountCents:  product.PriceCents,
			Currency:     product.Currency,
			BillingCycle: product.BillingCycle,
			Status:       "paid",
			PaidAt:       now - 120,
			CreatedAt:    now - 120,
			UpdatedAt:    now - 120,
		},
	} {
		if err := state.pg.CreatePurchaseOrder(ctx, order); err != nil {
			t.Fatalf("create order %s: %v", order.OrderID, err)
		}
	}
	if err := state.syncUserEntitlements(ctx, "user-fifo", "test fifo order binding"); err != nil {
		t.Fatalf("sync entitlement: %v", err)
	}
	bindings, err := state.pg.ListPurchaseOrderDeviceBindingsByUser(ctx, "user-fifo")
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].OrderID != "order-fifo-old" {
		t.Fatalf("expected oldest paid order to bind first, got %+v", bindings)
	}
}

func TestExpiredDeviceOrderBindingConsumesNextPaidOrderFromBindTime(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now().Unix()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-rebind",
		Email:        "rebind@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.seedDefaultProducts(ctx); err != nil {
		t.Fatalf("seed products: %v", err)
	}
	product, err := state.pg.GetProductByCode(ctx, "extra-device")
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	createNetworkFixture(t, state, "user-rebind", "net-rebind", "subnet-rebind", "10.78.0.0/24")
	for index, deviceID := range []string{"dev-r-free-1", "dev-r-free-2", "dev-r-paid"} {
		if err := state.pg.InsertDevice(ctx, repo.Device{
			DeviceID:  deviceID,
			UserID:    "user-rebind",
			MachineID: deviceID,
			Name:      deviceID,
			Platform:  "windows",
			Status:    "online",
			CreatedAt: now + int64(index),
		}); err != nil {
			t.Fatalf("insert device %s: %v", deviceID, err)
		}
		if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
			AttachmentID: "att-" + deviceID,
			NetworkID:    "net-rebind",
			SubnetID:     "subnet-rebind",
			DeviceID:     deviceID,
			VirtualIP:    "10.78.0." + string(rune('2'+index)),
			Status:       "active",
		}); err != nil {
			t.Fatalf("attach device %s: %v", deviceID, err)
		}
	}
	oldOrder := repo.PurchaseOrder{
		OrderID:      "order-old-device",
		UserID:       "user-rebind",
		UserEmail:    "rebind@local.slan",
		MerchantID:   defaultMerchantID,
		MerchantName: defaultMerchantName,
		ProductID:    product.ProductID,
		ProductCode:  product.ProductCode,
		ProductName:  product.ProductName,
		ProductType:  product.ProductType,
		Quantity:     1,
		Months:       1,
		UnitCents:    product.PriceCents,
		AmountCents:  product.PriceCents,
		Currency:     product.Currency,
		BillingCycle: product.BillingCycle,
		Status:       "paid",
		PaidAt:       now - 90*24*60*60,
		CreatedAt:    now - 90*24*60*60,
		UpdatedAt:    now - 90*24*60*60,
	}
	if err := state.pg.CreatePurchaseOrder(ctx, oldOrder); err != nil {
		t.Fatalf("create old order: %v", err)
	}
	if err := state.pg.CreatePurchaseOrderDeviceBinding(ctx, repo.PurchaseOrderDeviceBinding{
		BindingID: "binding-old-device",
		OrderID:   oldOrder.OrderID,
		UserID:    "user-rebind",
		DeviceID:  "dev-r-paid",
		BoundAt:   now - 90*24*60*60,
		ExpiresAt: now - 60,
		CreatedAt: now - 90*24*60*60,
		UpdatedAt: now - 90*24*60*60,
	}); err != nil {
		t.Fatalf("create expired binding: %v", err)
	}

	newOrder, err := (dbOpsService{state: state}).CreatePaidPurchaseOrder(dto.OpsCreatePaidOrderRequest{
		UserID:      "user-rebind",
		ProductCode: "extra-device",
		Quantity:    1,
		Months:      1,
	})
	if err != nil {
		t.Fatalf("create replacement paid order: %v", err)
	}
	bindings, err := state.pg.ListPurchaseOrderDeviceBindingsByUser(ctx, "user-rebind")
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	var replacement repo.PurchaseOrderDeviceBinding
	for _, binding := range bindings {
		if binding.OrderID == newOrder.OrderID {
			replacement = binding
		}
	}
	if replacement.OrderID == "" || replacement.DeviceID != "dev-r-paid" {
		t.Fatalf("expected replacement order to bind expired extra device, got order=%+v bindings=%+v", newOrder, bindings)
	}
	if replacement.BoundAt < now || replacement.ExpiresAt-replacement.BoundAt < 29*24*60*60 {
		t.Fatalf("replacement order should start its validity at bind time, got %+v", replacement)
	}
}

func TestCancelledDeviceOrderBindingIsIgnoredByEntitlementJob(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now().Unix()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-cancel",
		Email:        "cancel@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.seedDefaultProducts(ctx); err != nil {
		t.Fatalf("seed products: %v", err)
	}
	product, err := state.pg.GetProductByCode(ctx, "extra-device")
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	createNetworkFixture(t, state, "user-cancel", "net-cancel", "subnet-cancel", "10.79.0.0/24")
	for index, deviceID := range []string{"dev-c-free-1", "dev-c-free-2", "dev-c-paid"} {
		if err := state.pg.InsertDevice(ctx, repo.Device{
			DeviceID:  deviceID,
			UserID:    "user-cancel",
			MachineID: deviceID,
			Name:      deviceID,
			Platform:  "windows",
			Status:    "online",
			CreatedAt: now + int64(index),
		}); err != nil {
			t.Fatalf("insert device %s: %v", deviceID, err)
		}
		if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
			AttachmentID: "att-" + deviceID,
			NetworkID:    "net-cancel",
			SubnetID:     "subnet-cancel",
			DeviceID:     deviceID,
			VirtualIP:    "10.79.0." + string(rune('2'+index)),
			Status:       "active",
		}); err != nil {
			t.Fatalf("attach device %s: %v", deviceID, err)
		}
	}
	order := repo.PurchaseOrder{
		OrderID:      "order-cancelled-device",
		UserID:       "user-cancel",
		UserEmail:    "cancel@local.slan",
		MerchantID:   defaultMerchantID,
		MerchantName: defaultMerchantName,
		ProductID:    product.ProductID,
		ProductCode:  product.ProductCode,
		ProductName:  product.ProductName,
		ProductType:  product.ProductType,
		Quantity:     1,
		Months:       1,
		UnitCents:    product.PriceCents,
		AmountCents:  product.PriceCents,
		Currency:     product.Currency,
		BillingCycle: product.BillingCycle,
		Status:       "cancelled",
		PaidAt:       now - 60,
		CancelledAt:  now,
		CreatedAt:    now - 60,
		UpdatedAt:    now,
	}
	if err := state.pg.CreatePurchaseOrder(ctx, order); err != nil {
		t.Fatalf("create cancelled order: %v", err)
	}
	if err := state.pg.CreatePurchaseOrderDeviceBinding(ctx, repo.PurchaseOrderDeviceBinding{
		BindingID: "binding-cancelled-device",
		OrderID:   order.OrderID,
		UserID:    "user-cancel",
		DeviceID:  "dev-c-paid",
		BoundAt:   now - 60,
		ExpiresAt: now + 30*24*60*60,
		CreatedAt: now - 60,
		UpdatedAt: now - 60,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}
	if err := state.syncUserEntitlements(ctx, "user-cancel", "test cancelled order"); err != nil {
		t.Fatalf("sync entitlement: %v", err)
	}
	user, err := state.pg.GetUserByID(ctx, "user-cancel")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.AvailableDeviceCount != 2 {
		t.Fatalf("cancelled order binding should not keep paid device quota, got %+v", user)
	}
	attachment, err := state.pg.GetAttachmentByID(ctx, "att-dev-c-paid")
	if err != nil {
		t.Fatalf("get attachment: %v", err)
	}
	if attachment.Status != "suspended" {
		t.Fatalf("extra device should be suspended after cancelled order, got %+v", attachment)
	}
}

func TestPaidPendingDNSOrderExtendsExistingActiveDNS(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-3",
		Email:        "dns-renew@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.seedDefaultProducts(ctx); err != nil {
		t.Fatalf("seed products: %v", err)
	}
	first, err := (dbOpsService{state: state}).CreatePaidPurchaseOrder(dto.OpsCreatePaidOrderRequest{
		UserID:      "user-3",
		ProductCode: "dns",
		Months:      1,
	})
	if err != nil {
		t.Fatalf("create first dns order: %v", err)
	}
	pending, err := (dbNetworkService{state: state}).CreatePurchaseOrder("user-3", dto.CreatePurchaseOrderRequest{
		ProductCode: "dns",
		Quantity:    7,
		Months:      2,
	})
	if err != nil {
		t.Fatalf("create pending dns order: %v", err)
	}
	if pending.Quantity != 1 || pending.Months != 2 || pending.AmountCents != 2000 {
		t.Fatalf("pending dns order should force quantity to 1 and price by months: %+v", pending)
	}
	paid, err := (dbOpsService{state: state}).UpdatePurchaseOrderStatus(pending.OrderID, dto.UpdatePurchaseOrderStatusRequest{Status: "paid"})
	if err != nil {
		t.Fatalf("mark pending dns paid: %v", err)
	}
	if paid.ExpiresAt <= first.ExpiresAt {
		t.Fatalf("paid pending dns order should extend existing expiry: first=%+v paid=%+v", first, paid)
	}
}
