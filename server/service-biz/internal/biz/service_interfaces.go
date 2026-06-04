package biz

import "time"

// BusinessStore 是 service-biz HTTP/MQTT 入口依赖的业务与持久化边界。
// Server 只面向该接口编程；Store 是当前默认实现，内部组合内存状态和可选 Postgres 持久化。
type BusinessStore interface {
	AuthStore
	DeviceStore
	NetworkStore
	OpsStore
	DownloadStore
	PunchStore
	WireStore
	MQTTControlStore
	AuditStore
}

// AuthStore 定义用户登录、会话、控制台登录和设备引导相关能力。
type AuthStore interface {
	RegisterUser(email, password, name string) (AuthResponse, Network, error)
	LoginUserWithRateLimit(email, password, remoteIP string) (AuthResponse, error)
	AuthByToken(token string) (AuthResponse, error)
	DeviceAuthByToken(token string) (DeviceSession, error)
	LogoutSessions(accessToken, deviceToken string) error
	RenewUserSession(token string) (AuthResponse, error)
	CreateConsoleLoginKey(accessToken, deviceID string, ttl time.Duration) (ConsoleLoginKey, error)
	ConsumeConsoleLoginKey(loginKey string) (AuthResponse, error)
	CheckDeviceLoginPrepareRateLimit(deviceID, remoteIP string) error
	PrepareDeviceLoginDevice(deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, error)
	CompleteDeviceLoginForDevice(deviceID, accessToken, action string) (DeviceUserLoginPayload, error)
	CreateDeviceBootstrapKey(createdByUserID, networkID, deviceAlias string, ttlSeconds int64) (DeviceBootstrapKey, error)
	ListDeviceBootstrapKeys(createdByUserID string) []DeviceBootstrapKey
	RevokeDeviceBootstrapKey(keyID, actorUserID string) (DeviceBootstrapKey, error)
	ListUsers() []User
	ChangeUserPassword(userID, oldPassword, newPassword string) error
	ListUserAliases(ownerUserID string) []UserAlias
	UpsertUserAlias(ownerUserID, email, alias string) (UserAlias, error)
}

// DeviceStore 定义设备注册、续租、会话绑定、运行状态和可见性管理能力。
type DeviceStore interface {
	ListDevices(userID string) []Device
	ListVisibleDevices(userID string) []Device
	RegisterDevice(ownerID, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, NetworkDevice, error)
	RenewDevice(deviceID, userID string, networkEnabled bool, rxBytesTotal, txBytesTotal uint64) (Device, []NetworkConfig, error)
	BootstrapDeviceSession(sessionKey, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, DeviceSession, []NetworkConfig, error)
	BindDeviceSession(accessToken, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, DeviceSession, []NetworkConfig, error)
	RenewDeviceSession(deviceToken string, networkEnabled bool, rxBytesTotal, txBytesTotal uint64) (Device, DeviceSession, []NetworkConfig, error)
	NetworkConfigsForDevice(deviceID string) ([]NetworkConfig, error)
	GetDevice(deviceID string) (Device, error)
	DeviceQuota(userID string) (DeviceQuota, error)
	UpdateDeviceAlias(deviceID, actorUserID, alias string) (Device, error)
	RemoveVisibleDevice(deviceID, actorUserID string) error
	ReportDeviceRuntime(deviceID string, networkEnabled bool, rxBytesTotal, txBytesTotal uint64) DeviceRuntimeReportResult
	ReportDeviceEndpoint(networkID, deviceID string, endpoints []DeviceEndpoint) (bool, error)
}

// NetworkStore 定义虚拟网络、成员、DNS、安全组、公网映射和 relay ticket 相关能力。
type NetworkStore interface {
	CreateDeviceInvite(inviterUserID string, ttlSeconds int64) (DeviceInvite, error)
	ListDeviceInvites(inviterUserID string) []DeviceInvite
	AcceptDeviceInvite(inviteCode, deviceID, actorUserID string) (DeviceAccessGrant, DeviceInvite, error)
	CreateNetwork(ownerUserID, name, code, templateKey string) (Network, SecurityGroup, NetworkDNSZone, error)
	ListNetworks(userID string) []Network
	UpdateNetworkFull(networkID, name, code, status string) (Network, error)
	ListNetworkDevices(networkID string) []NetworkDevice
	ListNetworkDevicesForDevice(deviceID string) []NetworkDevice
	ListNetworkDevicesForUser(userID string) []NetworkDevice
	AddNetworkDevice(networkID, deviceID, actorUserID, alias string, enabled bool) (NetworkDevice, error)
	UpdateNetworkDevice(networkID, deviceID, alias string, enabled *bool) (NetworkDevice, error)
	RemoveNetworkDevice(networkID, deviceID string) error
	AddDNSZone(networkID, zoneName string, exposeGlobal bool) (NetworkDNSZone, error)
	UpdateDNSZone(networkID, zoneID, zoneName string, exposeGlobal bool) (NetworkDNSZone, error)
	DeleteDNSZone(networkID, zoneID string) error
	ListDNSZones(networkID string) []NetworkDNSZone
	AddDNSRecord(networkID, zoneID, name, recordType, targetDeviceID, targetIP, cname, port string, ttl int) (NetworkDNSRecord, error)
	UpdateDNSRecord(networkID, recordID, name, recordType, targetDeviceID, targetIP, cname, port string, ttl int) (NetworkDNSRecord, error)
	DeleteDNSRecord(networkID, recordID string) error
	ListDNSRecords(networkID string) []NetworkDNSRecord
	ListPublicMappings(networkID string) []PublicDomainMapping
	UpsertPublicMapping(mappingID, networkID, alias, publicDomain, sourceRecord, deviceID, protocol, port, externalPort, status string) (PublicDomainMapping, error)
	DeletePublicMapping(networkID, mappingID string) error
	CreateSecurityGroup(networkID, name, description, defaultPolicy string) (SecurityGroup, error)
	DeleteSecurityGroup(networkID, securityGroupID string) error
	ListSecurityGroups(networkID string) []SecurityGroup
	AddSecurityGroupRule(securityGroupID, direction, action, protocol, peerType, peerValue, description string, priority, portFrom, portTo int, enabled bool) (SecurityGroupRule, error)
	UpdateSecurityGroupRule(ruleID, direction, action, protocol, peerType, peerValue, description string, priority, portFrom, portTo int, enabled bool) (SecurityGroupRule, error)
	DeleteSecurityGroupRule(ruleID string) error
	SecurityGroupNetworkID(securityGroupID string) (string, error)
	GetSecurityRule(ruleID string) (SecurityGroupRule, error)
	ListSecurityGroupRules(securityGroupID string) []SecurityGroupRule
	NetworkConfig(networkID, deviceID string) (NetworkConfig, error)
	RelayCandidates(networkID, deviceID string) ([]RelayCandidate, error)
	IssueRelayTicket(networkID, srcNodeID, dstNodeID, derpClusterID string, preferredEndpointIDs []string) (RelayTicket, error)
}

// OpsStore 定义运营后台使用的账号、客户、商品、订单、节点和设备管理能力。
type OpsStore interface {
	LoginOperator(email, password string) (OperatorAuthResponse, error)
	LoginOperatorWithRateLimit(email, password, remoteIP string) (OperatorAuthResponse, error)
	OperatorByToken(token string) (OperatorUser, error)
	ChangeOwnOperatorPassword(operatorID, oldPassword, newPassword string) error
	OpsDashboard() map[string]any
	ListOperators() []OperatorUser
	UpsertOperator(operator OperatorUser) (OperatorUser, error)
	SetOperatorPassword(operatorID, newPassword string) error
	ListRelayNodes() []OpsRelayNode
	UpsertRelayNode(node OpsRelayNode) (OpsRelayNode, error)
	UpdateRelayNodeStatus(nodeID string, req OpsNodeStatusRequest) (OpsRelayNode, error)
	DeleteRelayNode(nodeID string) error
	ListPunchNodes() []OpsPunchNode
	ActivePunchNodes() []OpsPunchNode
	UpsertPunchNode(node OpsPunchNode) (OpsPunchNode, error)
	UpdatePunchNodeStatus(nodeID string, req OpsNodeStatusRequest) (OpsPunchNode, error)
	DeletePunchNode(nodeID string) error
	ListCustomers() []CustomerProfile
	UpdateCustomerProfile(profile CustomerProfile) (CustomerProfile, error)
	AssignCustomerPlan(customerID, planCode string, expiresAt int64, amount float64, period, operatorEmail string) (CustomerProfile, Renewal, error)
	ListOpsDevices() []OpsDeviceView
	UpdateOpsDevice(deviceID, alias, status string, enabled *bool) (OpsDeviceView, error)
	DeleteOpsDevice(deviceID string) error
	ListPlans() []OpsPlan
	UpsertPlan(plan OpsPlan) (OpsPlan, error)
	ListProducts() []Product
	UpsertProduct(product Product) (Product, error)
	ListOrders() []Order
	UpsertOrder(order Order) (Order, error)
	ListRenewals() []Renewal
	UpdateRenewal(renewal Renewal) (Renewal, error)
}

// DownloadStore 定义客户端安装包发布与下载记录管理能力。
type DownloadStore interface {
	ListClientDownloads(includeOffline bool) []ClientDownload
	UpsertClientDownload(input ClientDownload) (ClientDownload, error)
	GetClientDownload(downloadID string) (ClientDownload, error)
	DeleteClientDownload(downloadID string) error
}

// PunchStore 定义 biz 对 P2P 打洞请求的授权能力。
type PunchStore interface {
	AuthorizePunchConnect(networkID, requesterNodeID, peerNodeID string, auth punchDeviceAuth, mqtt MQTTConfig) error
}

// WireStore 定义 wire/relay/DERP 内部节点与拓扑查询接口。
type WireStore interface {
	WirePeerAuthz(peerID string) (map[string]any, error)
	WirePeerRuntimeConfig(peerID string) (map[string]any, error)
	WireNetworkTopology(networkID string) (map[string]any, error)
	UpsertWireNode(node OpsRelayNode) (OpsRelayNode, error)
	UpdateWireNodeHeartbeat(nodeID, transport string, req wireNodeHeartbeatRequest) (OpsRelayNode, error)
	UpdateWireNodeStatus(nodeID, transport string, req wireNodeStatusRequest) (OpsRelayNode, error)
	DeleteWireNode(nodeID, transport string) error
}

// MQTTControlStore 定义通过 MQTT 下发控制任务、确认结果和失败重试的能力。
type MQTTControlStore interface {
	ListActiveNetworkDeviceIDs(networkID string) []string
	HasActiveNetworkDevice(networkID, deviceID string) bool
	NextNetworkConfigVersion(networkID string, now int64) int64
	PrepareMQTTControlDelivery(deviceID, deliveryID, messageType, action string, payload any, createdAt, expiresAt int64) (MQTTControlDelivery, error)
	GetMQTTControlDeliveryForDevice(deviceID, deliveryID string) (MQTTControlDelivery, error)
	RecordMQTTControlAck(deviceID, deliveryID, taskID, action, status, errorText string, processedAtMs int64, now int64) (MQTTControlDelivery, error)
	RecordMQTTControlPublishResult(deviceID, deliveryID string, published bool, errorText string, now int64) (MQTTControlDelivery, error)
	ExpireMQTTControlDeliveries(now int64) int
	ListRetryableMQTTControlDeliveries(now int64, limit int) []MQTTControlDelivery
	ClaimMQTTControlDeliveryRetry(deviceID, deliveryID string, previousUpdatedAt, now int64) (bool, error)
}

// AuditStore 定义审计事件写入与查询能力。
type AuditStore interface {
	RecordAuditEvent(event AuditEvent) AuditEvent
	ListAuditEvents() []AuditEvent
	QueryAuditEvents(filter AuditEventFilter) ([]AuditEvent, error)
}

var _ BusinessStore = (*Store)(nil)
