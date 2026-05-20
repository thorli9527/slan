package biz

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	defaultOpsAdminEmail    = "admin1"
	defaultOpsAdminPassword = "admin1"
)

func (s *Store) seedOpsDefaultsLocked(now int64) {
	if len(s.operators) == 0 {
		operator := OperatorUser{
			OperatorID:   "op-000001",
			Name:         "超级管理员",
			Email:        defaultOpsAdminEmail,
			Role:         "super_admin",
			Status:       "active",
			PasswordHash: hashPassword(defaultOpsAdminPassword),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		s.operators[operator.OperatorID] = operator
		s.operatorByEmail[operator.Email] = operator.OperatorID
		s.nextOperatorSeq = 2
	}
	if len(s.opsPlans) == 0 {
		for _, plan := range []OpsPlan{
			{Code: "free", Name: "免费版", OwnDeviceLimit: 5, InvitedDeviceLimit: 5, TotalDeviceLimit: 10, RelayMonthlyGB: 50, RelayBandwidthMbps: 5, RelayThrottleMbps: 5, P2PUnlimited: true, Status: "active"},
			{Code: "pro", Name: "专业版", OwnDeviceLimit: 30, InvitedDeviceLimit: 100, TotalDeviceLimit: 130, RelayMonthlyGB: 1024, RelayBandwidthMbps: 100, RelayThrottleMbps: 100, P2PUnlimited: true, CustomDomain: true, ACL: true, MonthlyPrice: 39, YearlyPrice: 299, Status: "active"},
			{Code: "enterprise", Name: "企业版", OwnDeviceLimit: 500, InvitedDeviceLimit: 500, TotalDeviceLimit: 1000, RelayMonthlyGB: 10240, RelayBandwidthMbps: 1000, RelayThrottleMbps: 1000, P2PUnlimited: true, CustomDomain: true, ACL: true, DedicatedRelay: true, AuditLog: true, APIAccess: true, YearlyPrice: 2999, Status: "active"},
		} {
			s.opsPlans[plan.Code] = plan
		}
	}
	if len(s.products) == 0 {
		s.addProductLocked(Product{Name: "专业版月付", Type: "plan", PlanCode: "pro", Period: "monthly", ValidDays: 31, RelayTrafficGB: 1024, RelayBandwidthMbps: 100, ListPrice: 39, SalePrice: 39, Currency: "CNY", AutoRenew: true, Status: "active", Description: "专业版 1 个月"})
		s.addProductLocked(Product{Name: "专业版年付", Type: "plan", PlanCode: "pro", Period: "yearly", ValidDays: 365, RelayTrafficGB: 1024, RelayBandwidthMbps: 100, ListPrice: 468, SalePrice: 299, Currency: "CNY", AutoRenew: true, Status: "active", Description: "专业版 1 年"})
		s.addProductLocked(Product{Name: "企业版年付", Type: "plan", PlanCode: "enterprise", Period: "yearly", ValidDays: 365, RelayTrafficGB: 10240, RelayBandwidthMbps: 1000, ListPrice: 2999, SalePrice: 2999, Currency: "CNY", AutoRenew: false, Status: "active", Description: "企业版基础年付"})
	}
	if len(s.relayNodes) == 0 {
		s.addRelayNodeLocked(defaultOpsRelayNode())
	}
	if len(s.punchNodes) == 0 {
		for _, node := range configuredPunchNodes() {
			s.addPunchNodeLocked(node)
		}
	}
}

func defaultOpsRelayNode() OpsRelayNode {
	publicAddr := "udp://127.0.0.1:3478"
	region := "local"
	for _, candidate := range configuredRelayCandidates() {
		if candidate.Transport != "udp" || strings.TrimSpace(candidate.Address) == "" {
			continue
		}
		publicAddr = relayURL(candidate)
		region = defaultString(candidate.RegionID, region)
		break
	}
	return OpsRelayNode{Name: "默认 UDP Relay", Region: region, Transport: "relay_udp", PublicAddr: publicAddr, MaxBandwidthMbps: 1000, MonthlyTrafficGB: 10240, MaxSessions: 10000, Status: "active", Health: "healthy"}
}

func (s *Store) LoginOperator(email, password string) (OperatorAuthResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(password) == "" {
		return OperatorAuthResponse{}, errBadRequest
	}
	token, err := secureTokenHex(32)
	if err != nil {
		return OperatorAuthResponse{}, err
	}
	sessionID, err := secureTokenHex(12)
	if err != nil {
		return OperatorAuthResponse{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	operatorID, ok := s.operatorByEmail[email]
	if !ok {
		return OperatorAuthResponse{}, errNotFound
	}
	operator := s.operators[operatorID]
	if operator.Status != "active" || !verifyPasswordHash(operator.PasswordHash, password) {
		return OperatorAuthResponse{}, errBadRequest
	}
	now := time.Now().Unix()
	operator.LastLoginAt = now
	operator.UpdatedAt = now
	s.operators[operator.OperatorID] = operator
	session := OperatorSession{SessionID: "opsess-" + sessionID, OperatorID: operator.OperatorID, Token: token, CreatedAt: now, ExpiresAt: now + int64((24 * time.Hour).Seconds())}
	s.operatorSessions[session.SessionID] = session
	return OperatorAuthResponse{Operator: operator, Session: session}, nil
}

func (s *Store) OperatorByToken(token string) (OperatorUser, error) {
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	if token == "" {
		return OperatorUser{}, errUnauthorized
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	for _, session := range s.operatorSessions {
		if session.Token != token {
			continue
		}
		if session.ExpiresAt < now {
			return OperatorUser{}, errUnauthorized
		}
		operator, ok := s.operators[session.OperatorID]
		if !ok || operator.Status != "active" {
			return OperatorUser{}, errUnauthorized
		}
		return operator, nil
	}
	return OperatorUser{}, errUnauthorized
}

func (s *Store) ChangeOwnOperatorPassword(operatorID, oldPassword, newPassword string) error {
	if strings.TrimSpace(operatorID) == "" || strings.TrimSpace(oldPassword) == "" || strings.TrimSpace(newPassword) == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	operator, ok := s.operators[operatorID]
	if !ok {
		return errNotFound
	}
	if !verifyPasswordHash(operator.PasswordHash, oldPassword) {
		return errBadRequest
	}
	operator.PasswordHash = hashPassword(newPassword)
	operator.UpdatedAt = time.Now().Unix()
	s.operators[operator.OperatorID] = operator
	return nil
}

func (s *Store) ListOperators() []OperatorUser {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.operators, func(a, b OperatorUser) bool { return a.Email < b.Email })
}

func (s *Store) UpsertOperator(operator OperatorUser) (OperatorUser, error) {
	operator.Email = strings.ToLower(strings.TrimSpace(operator.Email))
	operator.Name = strings.TrimSpace(operator.Name)
	operator.Role = defaultString(operator.Role, "operator")
	operator.Status = defaultString(operator.Status, "active")
	if operator.Email == "" || operator.Name == "" {
		return OperatorUser{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	if strings.TrimSpace(operator.OperatorID) == "" {
		if _, exists := s.operatorByEmail[operator.Email]; exists {
			return OperatorUser{}, errConflict
		}
		operator.OperatorID = fmt.Sprintf("op-%06d", s.nextOperatorSeq)
		s.nextOperatorSeq++
		operator.PasswordHash = hashPassword(defaultString(operator.PasswordHash, defaultOpsAdminPassword))
		operator.CreatedAt = now
	} else {
		existing, ok := s.operators[operator.OperatorID]
		if !ok {
			return OperatorUser{}, errNotFound
		}
		if existing.Email != operator.Email {
			if _, exists := s.operatorByEmail[operator.Email]; exists {
				return OperatorUser{}, errConflict
			}
			delete(s.operatorByEmail, existing.Email)
		}
		operator.PasswordHash = existing.PasswordHash
		operator.CreatedAt = existing.CreatedAt
		operator.LastLoginAt = existing.LastLoginAt
	}
	operator.UpdatedAt = now
	s.operators[operator.OperatorID] = operator
	s.operatorByEmail[operator.Email] = operator.OperatorID
	return operator, nil
}

func (s *Store) SetOperatorPassword(operatorID, newPassword string) error {
	operatorID = strings.TrimSpace(operatorID)
	newPassword = strings.TrimSpace(newPassword)
	if operatorID == "" || newPassword == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	operator, ok := s.operators[operatorID]
	if !ok {
		return errNotFound
	}
	operator.PasswordHash = hashPassword(newPassword)
	operator.UpdatedAt = time.Now().Unix()
	s.operators[operatorID] = operator
	return nil
}

func (s *Store) ListPlans() []OpsPlan {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.opsPlans, func(a, b OpsPlan) bool { return a.Code < b.Code })
}

func (s *Store) UpsertPlan(plan OpsPlan) (OpsPlan, error) {
	plan.Code = sanitizeDNSLabel(plan.Code)
	plan.Name = strings.TrimSpace(plan.Name)
	plan.Status = defaultString(plan.Status, "active")
	if plan.Code == "" || plan.Name == "" {
		return OpsPlan{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opsPlans[plan.Code] = plan
	return plan, nil
}

func (s *Store) ListProducts() []Product {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.products, func(a, b Product) bool { return a.ProductID < b.ProductID })
}

func (s *Store) UpsertProduct(product Product) (Product, error) {
	product.Name = strings.TrimSpace(product.Name)
	product.Type = defaultString(product.Type, "plan")
	product.Period = defaultString(product.Period, "monthly")
	product.Currency = defaultString(product.Currency, "CNY")
	product.Status = defaultString(product.Status, "active")
	if product.Name == "" || strings.TrimSpace(product.PlanCode) == "" {
		return Product{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.opsPlans[product.PlanCode]; !ok {
		return Product{}, errNotFound
	}
	now := time.Now().Unix()
	if strings.TrimSpace(product.ProductID) == "" {
		return s.addProductLocked(product), nil
	}
	existing, ok := s.products[product.ProductID]
	if !ok {
		return Product{}, errNotFound
	}
	product.CreatedAt = existing.CreatedAt
	product.UpdatedAt = now
	s.products[product.ProductID] = product
	return product, nil
}

func (s *Store) addProductLocked(product Product) Product {
	now := time.Now().Unix()
	product.ProductID = fmt.Sprintf("prod-%06d", s.nextProductSeq)
	s.nextProductSeq++
	product.CreatedAt = now
	product.UpdatedAt = now
	s.products[product.ProductID] = product
	return product
}

func (s *Store) ListRelayNodes() []OpsRelayNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.relayNodes, func(a, b OpsRelayNode) bool { return a.NodeID < b.NodeID })
}

func (s *Store) UpsertRelayNode(node OpsRelayNode) (OpsRelayNode, error) {
	node.Name = strings.TrimSpace(node.Name)
	node.Region = defaultString(node.Region, "default")
	node.Transport = defaultString(node.Transport, "relay_udp")
	node.PublicAddr = strings.TrimSpace(node.PublicAddr)
	node.Status = defaultString(node.Status, "active")
	node.Health = defaultString(node.Health, "healthy")
	if node.Name == "" || node.PublicAddr == "" {
		return OpsRelayNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.relayNodes {
		if existing.PublicAddr == node.PublicAddr && existing.NodeID != node.NodeID {
			return OpsRelayNode{}, errConflict
		}
	}
	now := time.Now().Unix()
	if strings.TrimSpace(node.NodeID) == "" {
		node.Health = "healthy"
		return s.addRelayNodeLocked(node), nil
	}
	existing, ok := s.relayNodes[node.NodeID]
	if !ok {
		return OpsRelayNode{}, errNotFound
	}
	node.CreatedAt = existing.CreatedAt
	node.UpdatedAt = now
	node.UsedTrafficGB = existing.UsedTrafficGB
	node.ActiveSessions = existing.ActiveSessions
	node.Health = existing.Health
	node.TicketKeyRotation = existing.TicketKeyRotation
	s.relayNodes[node.NodeID] = node
	return node, nil
}

func (s *Store) addRelayNodeLocked(node OpsRelayNode) OpsRelayNode {
	now := time.Now().Unix()
	node.NodeID = fmt.Sprintf("relay-%06d", s.nextRelayNodeSeq)
	s.nextRelayNodeSeq++
	node.CreatedAt = now
	node.UpdatedAt = now
	s.relayNodes[node.NodeID] = node
	return node
}

func (s *Store) ListPunchNodes() []OpsPunchNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.punchNodes, func(a, b OpsPunchNode) bool {
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.NodeID < b.NodeID
	})
}

func (s *Store) ActivePunchNodes() []OpsPunchNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	nodes := make([]OpsPunchNode, 0, len(s.punchNodes))
	for _, node := range s.punchNodes {
		if strings.TrimSpace(node.PublicUDPIP) == "" || node.PublicUDPPort <= 0 {
			continue
		}
		if !strings.EqualFold(defaultString(node.Status, "active"), "active") {
			continue
		}
		if !strings.EqualFold(defaultString(node.Health, "healthy"), "healthy") {
			continue
		}
		nodes = append(nodes, node)
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Priority != nodes[j].Priority {
			return nodes[i].Priority < nodes[j].Priority
		}
		return nodes[i].NodeID < nodes[j].NodeID
	})
	return nodes
}

func (s *Store) UpsertPunchNode(node OpsPunchNode) (OpsPunchNode, error) {
	node.Name = strings.TrimSpace(node.Name)
	node.Region = defaultString(node.Region, "default")
	node.PublicUDPIP = strings.TrimSpace(node.PublicUDPIP)
	node.Status = defaultString(node.Status, "active")
	node.Health = defaultString(node.Health, "healthy")
	if node.Name == "" || node.PublicUDPIP == "" || node.PublicUDPPort <= 0 || node.PublicUDPPort > 65534 {
		return OpsPunchNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.punchNodes {
		if existing.PublicUDPIP == node.PublicUDPIP && existing.PublicUDPPort == node.PublicUDPPort && existing.NodeID != node.NodeID {
			return OpsPunchNode{}, errConflict
		}
	}
	now := time.Now().Unix()
	if strings.TrimSpace(node.NodeID) == "" {
		node.Health = defaultString(node.Health, "healthy")
		return s.addPunchNodeLocked(node), nil
	}
	existing, ok := s.punchNodes[node.NodeID]
	if !ok {
		return OpsPunchNode{}, errNotFound
	}
	node.CreatedAt = existing.CreatedAt
	node.UpdatedAt = now
	node.ActiveSessions = existing.ActiveSessions
	s.punchNodes[node.NodeID] = node
	return node, nil
}

func (s *Store) addPunchNodeLocked(node OpsPunchNode) OpsPunchNode {
	now := time.Now().Unix()
	node.NodeID = fmt.Sprintf("punch-%06d", s.nextPunchNodeSeq)
	s.nextPunchNodeSeq++
	node.CreatedAt = now
	node.UpdatedAt = now
	s.punchNodes[node.NodeID] = node
	return node
}

func (s *Store) ListCustomers() []CustomerProfile {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CustomerProfile, 0, len(s.users))
	for _, user := range s.users {
		out = append(out, s.customerProfileLocked(user))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	return out
}

func (s *Store) UpdateCustomerProfile(profile CustomerProfile) (CustomerProfile, error) {
	profile.CustomerID = strings.TrimSpace(profile.CustomerID)
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Country = strings.TrimSpace(profile.Country)
	profile.Province = strings.TrimSpace(profile.Province)
	profile.City = strings.TrimSpace(profile.City)
	profile.IPRegion = strings.TrimSpace(profile.IPRegion)
	profile.Status = defaultString(profile.Status, "active")
	if profile.CustomerID == "" || profile.Email == "" {
		return CustomerProfile{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return CustomerProfile{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	user, ok := s.users[profile.CustomerID]
	if !ok {
		return CustomerProfile{}, errNotFound
	}
	if user.Email != profile.Email {
		if _, exists := s.userByEmail[profile.Email]; exists {
			return CustomerProfile{}, errConflict
		}
		delete(s.userByEmail, user.Email)
		user.Email = profile.Email
		s.userByEmail[user.Email] = user.UserID
	}
	user.Name = profile.Name
	now := time.Now().Unix()
	disableCustomer := profile.Status == "disabled"
	if profile.Status == "disabled" {
		user.Status = "disabled"
	} else {
		user.Status = "active"
	}
	user.UpdatedAt = now
	s.users[user.UserID] = user
	s.customerProfiles[user.UserID] = CustomerProfile{
		CustomerID: user.UserID,
		Country:    profile.Country,
		Province:   profile.Province,
		City:       profile.City,
		IPRegion:   profile.IPRegion,
		Status:     profile.Status,
	}
	affectedDevices := make([]Device, 0)
	affectedStatuses := make([]DeviceRuntimeStatus, 0)
	affectedMemberships := make([]NetworkDevice, 0)
	if disableCustomer {
		for token, session := range s.sessions {
			if session.UserID == user.UserID {
				delete(s.sessions, token)
			}
		}
		for sessionID, session := range s.deviceSessions {
			if session.UserID != user.UserID || session.State != "active" {
				continue
			}
			session.State = "revoked"
			s.deviceSessions[sessionID] = session
			delete(s.deviceSessionByToken, session.DeviceToken)
		}
		for deviceID, device := range s.devices {
			if device.OwnerID != user.UserID {
				continue
			}
			device.Status = "disabled"
			device.UpdatedAt = now
			s.devices[deviceID] = device
			status := s.runtimeStatuses[deviceID]
			status.DeviceID = deviceID
			status.DeviceEnabled = false
			status.NetworkEnabled = false
			status.LastReportAt = now
			s.runtimeStatuses[deviceID] = status
			affectedDevices = append(affectedDevices, device)
			affectedStatuses = append(affectedStatuses, status)
		}
		for key, networkDevice := range s.networkDevices {
			if networkDevice.OwnerUserID != user.UserID {
				continue
			}
			networkDevice.Enabled = false
			networkDevice.Status = "disabled"
			networkDevice.UpdatedAt = now
			s.networkDevices[key] = networkDevice
			affectedMemberships = append(affectedMemberships, networkDevice)
		}
	}
	if err := s.persistPostgresCustomerProfileUpdateTxLocked(ctx, postgresTx, user, disableCustomer, affectedDevices, affectedStatuses, affectedMemberships); err != nil {
		return CustomerProfile{}, err
	}
	postgresTx = nil
	return s.customerProfileLocked(user), nil
}

func (s *Store) ListOpsDevices() []OpsDeviceView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]OpsDeviceView, 0, len(s.devices))
	for _, device := range s.devices {
		out = append(out, s.opsDeviceViewLocked(device))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			return out[i].DeviceID < out[j].DeviceID
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}

func (s *Store) UpdateOpsDevice(deviceID, alias, status string, enabled *bool) (OpsDeviceView, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return OpsDeviceView{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return OpsDeviceView{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok {
		return OpsDeviceView{}, errNotFound
	}
	if strings.TrimSpace(alias) != "" {
		device.Alias = strings.TrimSpace(alias)
	}
	if strings.TrimSpace(status) != "" {
		device.Status = strings.TrimSpace(status)
	}
	now := time.Now().Unix()
	device.UpdatedAt = now
	s.devices[deviceID] = device
	if enabled != nil {
		runtime := s.runtimeStatuses[deviceID]
		runtime.DeviceID = deviceID
		runtime.DeviceEnabled = *enabled
		runtime.LastReportAt = now
		if !*enabled {
			runtime.NetworkEnabled = false
		}
		s.runtimeStatuses[deviceID] = runtime
		for key, networkDevice := range s.networkDevices {
			if networkDevice.DeviceID != deviceID {
				continue
			}
			networkDevice.Enabled = *enabled
			if *enabled {
				networkDevice.Status = "active"
			} else {
				networkDevice.Status = "disabled"
			}
			networkDevice.UpdatedAt = now
			s.networkDevices[key] = networkDevice
		}
	}
	runtime := s.runtimeStatuses[deviceID]
	memberships := make([]NetworkDevice, 0)
	for _, networkDevice := range s.networkDevices {
		if networkDevice.DeviceID == deviceID {
			memberships = append(memberships, networkDevice)
		}
	}
	if err := s.persistPostgresOpsDeviceUpdateTxLocked(ctx, postgresTx, device, runtime, memberships); err != nil {
		return OpsDeviceView{}, err
	}
	postgresTx = nil
	return s.opsDeviceViewLocked(device), nil
}

func (s *Store) DeleteOpsDevice(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeDeviceLocked(deviceID)
}

func (s *Store) AssignCustomerPlan(customerID, planCode string, expiresAt int64, amount float64, period, operatorEmail string) (CustomerProfile, Renewal, error) {
	customerID = strings.TrimSpace(customerID)
	planCode = strings.TrimSpace(planCode)
	if customerID == "" || planCode == "" {
		return CustomerProfile{}, Renewal{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[customerID]
	if !ok {
		return CustomerProfile{}, Renewal{}, errNotFound
	}
	if _, ok := s.opsPlans[planCode]; !ok {
		return CustomerProfile{}, Renewal{}, errNotFound
	}
	now := time.Now().Unix()
	if expiresAt == 0 {
		expiresAt = now + int64((365 * 24 * time.Hour).Seconds())
	}
	s.customerPlans[customerID] = CustomerPlanAssignment{UserID: customerID, PlanCode: planCode, ExpiresAt: expiresAt, UpdatedAt: now}
	renewal := Renewal{
		RenewalID:     fmt.Sprintf("renew-%06d", s.nextRenewalSeq),
		CustomerID:    customerID,
		CustomerEmail: user.Email,
		PlanCode:      planCode,
		Period:        defaultString(period, "manual"),
		Amount:        amount,
		Currency:      "CNY",
		PaidAt:        now,
		ValidUntil:    expiresAt,
		Source:        "manual",
		Operator:      operatorEmail,
	}
	s.nextRenewalSeq++
	s.renewals[renewal.RenewalID] = renewal
	return s.customerProfileLocked(user), renewal, nil
}

func (s *Store) opsDeviceViewLocked(device Device) OpsDeviceView {
	ownerEmail := ""
	if user, ok := s.users[device.OwnerID]; ok {
		ownerEmail = user.Email
	}
	runtime := s.runtimeStatuses[device.DeviceID]
	networkCount := 0
	for _, networkDevice := range s.networkDevices {
		if networkDevice.DeviceID == device.DeviceID {
			networkCount++
		}
	}
	return OpsDeviceView{
		DeviceID:        device.DeviceID,
		OwnerID:         device.OwnerID,
		OwnerEmail:      ownerEmail,
		Name:            device.Name,
		Alias:           device.Alias,
		Platform:        device.Platform,
		OSName:          device.OSName,
		OSVersion:       device.OSVersion,
		GlobalIP:        device.GlobalIP,
		GlobalName:      device.GlobalName,
		Status:          device.Status,
		HeartbeatOnline: runtime.HeartbeatOnline,
		NetworkEnabled:  runtime.NetworkEnabled,
		DeviceEnabled:   runtime.DeviceEnabled,
		RxBytesTotal:    runtime.RxBytesTotal,
		TxBytesTotal:    runtime.TxBytesTotal,
		NetworkCount:    networkCount,
		LastSeenAt:      runtime.LastSeenAt,
		LastReportAt:    runtime.LastReportAt,
		CreatedAt:       device.CreatedAt,
		UpdatedAt:       device.UpdatedAt,
	}
}

func (s *Store) ListOrders() []Order {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.orders, func(a, b Order) bool { return a.CreatedAt > b.CreatedAt })
}

func (s *Store) UpsertOrder(order Order) (Order, error) {
	order.CustomerEmail = strings.ToLower(strings.TrimSpace(order.CustomerEmail))
	order.ProductID = strings.TrimSpace(order.ProductID)
	order.PayStatus = defaultString(order.PayStatus, "pending")
	order.ProvisionStatus = defaultString(order.ProvisionStatus, "pending")
	order.Currency = defaultString(order.Currency, "CNY")
	order.Channel = defaultString(order.Channel, "manual")
	if order.CustomerEmail == "" || order.ProductID == "" {
		return Order{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.userByEmail[order.CustomerEmail]
	if !ok {
		return Order{}, errNotFound
	}
	product, ok := s.products[order.ProductID]
	if !ok {
		return Order{}, errNotFound
	}
	now := time.Now().Unix()
	if strings.TrimSpace(order.OrderID) == "" {
		order.OrderID = fmt.Sprintf("ord-%06d", s.nextOrderSeq)
		s.nextOrderSeq++
		order.CreatedAt = now
	} else {
		existing, ok := s.orders[order.OrderID]
		if !ok {
			return Order{}, errNotFound
		}
		if order.CreatedAt == 0 {
			order.CreatedAt = existing.CreatedAt
		}
		if order.PaidAt == 0 {
			order.PaidAt = existing.PaidAt
		}
		if order.ValidUntil == 0 {
			order.ValidUntil = existing.ValidUntil
		}
	}
	order.CustomerID = userID
	order.ProductName = product.Name
	order.ProductType = product.Type
	if order.Amount == 0 {
		order.Amount = product.SalePrice
	}
	if order.ValidUntil == 0 && product.ValidDays > 0 {
		order.ValidUntil = now + int64(time.Duration(product.ValidDays)*24*time.Hour/time.Second)
	}
	s.orders[order.OrderID] = order
	return order, nil
}

func (s *Store) ListRenewals() []Renewal {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.renewals, func(a, b Renewal) bool { return a.PaidAt > b.PaidAt })
}

func (s *Store) UpdateRenewal(renewal Renewal) (Renewal, error) {
	renewal.RenewalID = strings.TrimSpace(renewal.RenewalID)
	renewal.CustomerEmail = strings.ToLower(strings.TrimSpace(renewal.CustomerEmail))
	renewal.PlanCode = strings.TrimSpace(renewal.PlanCode)
	renewal.Period = defaultString(renewal.Period, "manual")
	renewal.Currency = defaultString(renewal.Currency, "CNY")
	renewal.Source = defaultString(renewal.Source, "manual")
	if renewal.RenewalID == "" || renewal.CustomerEmail == "" || renewal.PlanCode == "" {
		return Renewal{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.renewals[renewal.RenewalID]
	if !ok {
		return Renewal{}, errNotFound
	}
	userID, ok := s.userByEmail[renewal.CustomerEmail]
	if !ok {
		return Renewal{}, errNotFound
	}
	if _, ok := s.opsPlans[renewal.PlanCode]; !ok {
		return Renewal{}, errNotFound
	}
	if renewal.PaidAt == 0 {
		renewal.PaidAt = existing.PaidAt
	}
	if renewal.ValidUntil == 0 {
		renewal.ValidUntil = existing.ValidUntil
	}
	renewal.CustomerID = userID
	s.renewals[renewal.RenewalID] = renewal
	s.customerPlans[userID] = CustomerPlanAssignment{
		UserID:    userID,
		PlanCode:  renewal.PlanCode,
		ExpiresAt: renewal.ValidUntil,
		UpdatedAt: time.Now().Unix(),
	}
	return renewal, nil
}

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
	for _, node := range s.relayNodes {
		if node.Status != "active" || node.Health == "down" || wireNodeStale(node, now) {
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
			Address:    node.PublicAddr,
			RegionID:   node.Region,
			ClusterID:  node.Region,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EndpointID < out[j].EndpointID })
	return out
}
