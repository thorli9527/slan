package biz

// OpsService 承载运营后台业务实现。
type OpsService struct {
	store BusinessStore
}

func (s OpsService) Login(req OpsLoginRequest, remoteIP string) (OperatorAuthResponse, error) {
	return s.store.LoginOperatorWithRateLimit(req.Email, req.Password, remoteIP)
}

func (s OpsService) OperatorByToken(token string) (OperatorUser, error) {
	return s.store.OperatorByToken(token)
}

func (s OpsService) ChangePassword(operatorID string, req OpsChangePasswordRequest) error {
	return s.store.ChangeOwnOperatorPassword(operatorID, req.OldPassword, req.NewPassword)
}

func (s OpsService) Dashboard() map[string]any {
	return s.store.OpsDashboard()
}

func (s OpsService) AuditEvents(filter AuditEventFilter) ([]AuditEvent, error) {
	return s.store.QueryAuditEvents(filter)
}

func (s OpsService) ListOperators() []OperatorUser {
	return s.store.ListOperators()
}

func (s OpsService) UpsertOperator(operator OperatorUser) (OperatorUser, error) {
	return s.store.UpsertOperator(operator)
}

func (s OpsService) SetOperatorPassword(operatorID string, req OpsSetOperatorPasswordRequest) error {
	return s.store.SetOperatorPassword(operatorID, req.NewPassword)
}

func (s OpsService) ListRelayNodes() []OpsRelayNode {
	return s.store.ListRelayNodes()
}

func (s OpsService) UpsertRelayNode(node OpsRelayNode) (OpsRelayNode, error) {
	return s.store.UpsertRelayNode(node)
}

func (s OpsService) UpdateRelayNodeStatus(nodeID string, req OpsNodeStatusRequest) (OpsRelayNode, error) {
	return s.store.UpdateRelayNodeStatus(nodeID, req)
}

func (s OpsService) DeleteRelayNode(nodeID string) error {
	return s.store.DeleteRelayNode(nodeID)
}

func (s OpsService) ListPunchNodes() []OpsPunchNode {
	return s.store.ListPunchNodes()
}

func (s OpsService) UpsertPunchNode(node OpsPunchNode) (OpsPunchNode, error) {
	return s.store.UpsertPunchNode(node)
}

func (s OpsService) UpdatePunchNodeStatus(nodeID string, req OpsNodeStatusRequest) (OpsPunchNode, error) {
	return s.store.UpdatePunchNodeStatus(nodeID, req)
}

func (s OpsService) DeletePunchNode(nodeID string) error {
	return s.store.DeletePunchNode(nodeID)
}

func (s OpsService) ListCustomers() []CustomerProfile {
	return s.store.ListCustomers()
}

func (s OpsService) UpdateCustomer(profile CustomerProfile) (CustomerProfile, error) {
	return s.store.UpdateCustomerProfile(profile)
}

func (s OpsService) ListDevices() []OpsDeviceView {
	return s.store.ListOpsDevices()
}

func (s OpsService) UpdateDevice(deviceID string, req OpsUpdateDeviceRequest) (OpsDeviceView, error) {
	return s.store.UpdateOpsDevice(deviceID, req.Alias, req.Status, req.Enabled)
}

func (s OpsService) DeleteDevice(deviceID string) error {
	return s.store.DeleteOpsDevice(deviceID)
}

func (s OpsService) AssignCustomerPlan(customerID string, req OpsAssignCustomerPlanRequest, operatorEmail string) (CustomerProfile, Renewal, error) {
	return s.store.AssignCustomerPlan(customerID, req.PlanCode, req.ExpiresAt, req.Amount, req.Period, operatorEmail)
}

func (s OpsService) ListPlans() []OpsPlan {
	return s.store.ListPlans()
}

func (s OpsService) UpsertPlan(plan OpsPlan) (OpsPlan, error) {
	return s.store.UpsertPlan(plan)
}

func (s OpsService) ListProducts() []Product {
	return s.store.ListProducts()
}

func (s OpsService) UpsertProduct(product Product) (Product, error) {
	return s.store.UpsertProduct(product)
}

func (s OpsService) ListOrders() []Order {
	return s.store.ListOrders()
}

func (s OpsService) UpsertOrder(order Order) (Order, error) {
	return s.store.UpsertOrder(order)
}

func (s OpsService) ListRenewals() []Renewal {
	return s.store.ListRenewals()
}

func (s OpsService) UpdateRenewal(renewal Renewal) (Renewal, error) {
	return s.store.UpdateRenewal(renewal)
}
