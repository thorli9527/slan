package biz

import "net/http"

// ExternalHTTPAPI 是 service-biz 对外 HTTP 能力边界。
// 具体实现负责注册路由；本文件中的 DTO 定义客户端、控制台和内部服务共用的稳定契约。
type ExternalHTTPAPI interface {
	Routes() http.Handler
}

// RegisterUserRequest 是用户注册请求。
type RegisterUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// AuthEnvelopeResponse 是把认证信息包装在 auth 字段下的通用响应。
type AuthEnvelopeResponse struct {
	Auth AuthResponse `json:"auth"`
}

// RegisterUserResponse 是注册成功响应，包含默认网络。
type RegisterUserResponse struct {
	Auth           AuthResponse `json:"auth"`
	DefaultNetwork Network      `json:"defaultNetwork"`
}

// LoginUserRequest 是用户登录请求。
type LoginUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LogoutUserRequest 是用户登出请求，可同时撤销设备 token。
type LogoutUserRequest struct {
	DeviceToken string `json:"deviceToken"`
}

// CreateConsoleLoginKeyRequest 是客户端为控制台扫码/跳转登录创建临时 key 的请求。
type CreateConsoleLoginKeyRequest struct {
	DeviceID string `json:"deviceId"`
}

// ConsoleLoginRequest 是控制台消费临时登录 key 的请求。
type ConsoleLoginRequest struct {
	LoginKey string `json:"loginKey"`
}

// DeviceIdentityRequest 是客户端设备身份信息的通用请求片段。
type DeviceIdentityRequest struct {
	DeviceID      string `json:"deviceId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	OSName        string `json:"osName"`
	OSVersion     string `json:"osVersion"`
	Alias         string `json:"alias"`
	PublicKey     string `json:"publicKey"`
	DeviceVersion string `json:"deviceVersion,omitempty"`
}

// PrepareDeviceLoginResponse 是设备登录准备阶段返回给客户端的信息。
type PrepareDeviceLoginResponse struct {
	DeviceID string          `json:"deviceId"`
	LoginURL string          `json:"loginUrl"`
	MQTT     *MQTTCredential `json:"mqtt,omitempty"`
}

// CompleteDeviceLoginRequest 是设备登录确认请求。
type CompleteDeviceLoginRequest struct {
	AccessToken string `json:"accessToken"`
	Token       string `json:"token"`
	Action      string `json:"action"`
}

// CompleteDeviceLoginResponse 是设备登录确认完成后的响应。
type CompleteDeviceLoginResponse struct {
	Status     string `json:"status"`
	DeviceID   string `json:"deviceId"`
	DeliveryID string `json:"deliveryId"`
}

// CreateDeviceBootstrapKeyRequest 是控制台创建设备引导密钥的请求。
type CreateDeviceBootstrapKeyRequest struct {
	UserID      string `json:"userId"`
	NetworkID   string `json:"networkId"`
	DeviceAlias string `json:"deviceAlias"`
	TTLSeconds  int64  `json:"ttlSeconds"`
}

// RevokeDeviceBootstrapKeyRequest 是撤销设备引导密钥的请求。
type RevokeDeviceBootstrapKeyRequest struct {
	UserID string `json:"userId"`
}

type CreateDeviceGroupRequest struct {
	Name string `json:"name"`
}

type UpdateDeviceGroupRequest struct {
	Name string `json:"name"`
}

type SetDeviceGroupsRequest struct {
	GroupIDs []string `json:"groupIds"`
}

// DeviceSessionBootstrapRequest 是设备使用引导密钥创建会话的请求。
type DeviceSessionBootstrapRequest struct {
	SessionKey string `json:"sessionKey"`
	DeviceIdentityRequest
}

// DeviceSessionBindRequest 是设备绑定到用户会话的请求。
type DeviceSessionBindRequest = DeviceIdentityRequest

// DeviceRuntimeCountersRequest 是设备上报运行开关和流量计数的请求片段。
type DeviceRuntimeCountersRequest struct {
	NetworkEnabled bool   `json:"networkEnabled"`
	RxBytesTotal   uint64 `json:"rxBytesTotal"`
	TxBytesTotal   uint64 `json:"txBytesTotal"`
}

// DeviceSessionResponse 是设备注册、绑定和续租接口返回的完整运行配置。
type DeviceSessionResponse struct {
	Device           Device                   `json:"device"`
	DeviceSession    DeviceSession            `json:"deviceSession"`
	MQTT             *MQTTCredential          `json:"mqtt,omitempty"`
	NetworkConfigs   ItemsResponse            `json:"networkConfigs"`
	RuntimeEndpoints RuntimeEndpointsResponse `json:"runtimeEndpoints"`
}

// RuntimeEndpointsResponse 是设备注册、绑定和续租时下发的运行端点总表。
// 客户端应以该响应覆盖本地 MQTT、UDP 打洞、UDP relay 和 TCP/DERP relay 参数。
type RuntimeEndpointsResponse struct {
	MQTT            *MQTTCredential          `json:"mqtt,omitempty"`
	PunchNodes      []RuntimePunchNode       `json:"punchNodes"`
	RelayCandidates []RelayCandidate         `json:"relayCandidates"`
	Networks        []RuntimeNetworkEndpoint `json:"networks"`
	RefreshedAt     int64                    `json:"refreshedAt"`
}

// RuntimePunchNode 是客户端发起 UDP 打洞协商时可访问的 punch-service 节点。
type RuntimePunchNode struct {
	NodeID        string `json:"nodeId"`
	Name          string `json:"name,omitempty"`
	Region        string `json:"region,omitempty"`
	Address       string `json:"address"`
	PublicUDPIP   string `json:"publicUdpIp"`
	PublicUDPPort int    `json:"publicUdpPort"`
}

// RuntimeNetworkEndpoint 是某个网络维度下的 relay 候选节点清单。
type RuntimeNetworkEndpoint struct {
	NetworkID       string           `json:"networkId"`
	RelayCandidates []RelayCandidate `json:"relayCandidates"`
}

// ItemsResponse 是列表接口的统一响应包装。
type ItemsResponse struct {
	Items any `json:"items"`
}

// StatusResponse 是只需要返回状态字符串的通用响应。
type StatusResponse struct {
	Status string `json:"status"`
}

// RelayCandidatesRequest 是客户端查询 relay 候选节点的请求。
type RelayCandidatesRequest struct {
	DeviceID string `json:"deviceId"`
}

// IssueRelayTicketRequest 是签发 relay/DERP 短期票据的请求。
type IssueRelayTicketRequest struct {
	NetworkID                 string   `json:"networkId"`
	SrcNodeID                 string   `json:"srcNodeId"`
	DstNodeID                 string   `json:"dstNodeId"`
	DERPClusterID             string   `json:"derpClusterId"`
	PreferredDERPNodeIDs      []string `json:"preferredDerpNodeIds"`
	PreferredRelayEndpointIDs []string `json:"preferredRelayEndpointIds"`
	Reason                    string   `json:"reason"`
	RelayRegionID             string   `json:"relayRegionId"`
}

// CreatePunchConnectSessionRequest 是创建 P2P 打洞协商会话的请求。
type CreatePunchConnectSessionRequest struct {
	RequesterNodeID string `json:"requesterNodeId"`
	PeerNodeID      string `json:"peerNodeId"`
	TTLSeconds      int    `json:"ttlSeconds"`
}

// ChangeUserPasswordRequest 是用户修改自己密码的请求。
type ChangeUserPasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// UpsertUserAliasRequest 是设置用户邮箱别名的请求。
type UpsertUserAliasRequest struct {
	OwnerUserID string `json:"ownerUserId"`
	Email       string `json:"email"`
	Alias       string `json:"alias"`
}

// RegisterDeviceRequest 是显式注册设备到用户账号的请求。
type RegisterDeviceRequest struct {
	UserID string `json:"userId"`
	DeviceIdentityRequest
}

// RenewDeviceRequest 是设备续租并上报运行计数的请求。
type RenewDeviceRequest struct {
	UserID string `json:"userId"`
	DeviceRuntimeCountersRequest
}

// UpdateDeviceAliasRequest 是用户修改设备别名的请求。
type UpdateDeviceAliasRequest struct {
	ActorUserID string `json:"actorUserId"`
	Alias       string `json:"alias"`
}

// DeleteDeviceRequest 是用户移除可见设备的请求。
type DeleteDeviceRequest struct {
	ActorUserID string `json:"actorUserId"`
}

// CreateNetworkRequest 是创建虚拟网络的请求。
type CreateNetworkRequest struct {
	OwnerUserID      string `json:"ownerUserId"`
	Name             string `json:"name"`
	Code             string `json:"code"`
	TemplateKey      string `json:"templateKey"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
}

// UpdateNetworkRequest 是更新虚拟网络基础信息的请求。
type UpdateNetworkRequest struct {
	Name             string `json:"name"`
	Code             string `json:"code"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
}

// CreateDeviceInviteRequest 是创建设备邀请的请求。
type CreateDeviceInviteRequest struct {
	InviterUserID string `json:"inviterUserId"`
	TTLSeconds    int64  `json:"ttlSeconds"`
}

// AcceptDeviceInviteRequest 是设备接受邀请的请求。
type AcceptDeviceInviteRequest struct {
	InviteCode  string `json:"inviteCode"`
	DeviceID    string `json:"deviceId"`
	ActorUserID string `json:"actorUserId"`
}

// AddNetworkDeviceRequest 是把设备加入虚拟网络的请求。
type AddNetworkDeviceRequest struct {
	DeviceID    string `json:"deviceId"`
	ActorUserID string `json:"actorUserId"`
	Alias       string `json:"alias"`
	Enabled     *bool  `json:"enabled"`
}

// UpdateNetworkDeviceRequest 是更新网络成员别名和启用状态的请求。
type UpdateNetworkDeviceRequest struct {
	Alias   string `json:"alias"`
	Enabled *bool  `json:"enabled"`
}

// DNSZoneRequest 是创建或更新 DNS Zone 的请求。
type DNSZoneRequest struct {
	ZoneName     string `json:"zoneName"`
	ExposeGlobal bool   `json:"exposeGlobal"`
}

// AddDNSRecordRequest 是创建 DNS 记录的请求。
type AddDNSRecordRequest struct {
	ZoneID string `json:"zoneId"`
	DNSRecordRequest
}

// DNSRecordRequest 是 DNS 记录可编辑字段。
type DNSRecordRequest struct {
	Name           string `json:"name"`
	RecordType     string `json:"recordType"`
	TargetDeviceID string `json:"targetDeviceId"`
	TargetIP       string `json:"targetIp"`
	CNAME          string `json:"cname"`
	Port           string `json:"port"`
	TTL            int    `json:"ttl"`
}

// PublicMappingRequest 是公网域名映射的可编辑字段。
type PublicMappingRequest struct {
	Alias        string `json:"alias"`
	PublicDomain string `json:"publicDomain"`
	SourceRecord string `json:"sourceRecord"`
	DeviceID     string `json:"deviceId"`
	Protocol     string `json:"protocol"`
	Port         string `json:"port"`
	ExternalPort string `json:"externalPort"`
	Status       string `json:"status"`
}

// CreateSecurityGroupRequest 是创建安全组的请求。
type CreateSecurityGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SecurityGroupRequest 是更新安全组可编辑字段的请求。
type SecurityGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SecurityRuleRequest 是创建或更新安全组规则的请求。
type SecurityRuleRequest struct {
	Direction   string `json:"direction"`
	Priority    int    `json:"priority"`
	Action      string `json:"action"`
	Protocol    string `json:"protocol"`
	PortFrom    int    `json:"portFrom"`
	PortTo      int    `json:"portTo"`
	PeerType    string `json:"peerType"`
	PeerValue   string `json:"peerValue"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

// OpsLoginRequest 是运营后台登录请求。
type OpsLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// OpsChangePasswordRequest 是运营账号修改自己密码的请求。
type OpsChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// OpsSetOperatorPasswordRequest 是管理员重置运营账号密码的请求。
type OpsSetOperatorPasswordRequest struct {
	NewPassword string `json:"newPassword"`
}

// OpsUpdateDeviceRequest 是运营后台更新设备状态的请求。
type OpsUpdateDeviceRequest struct {
	Alias   string `json:"alias"`
	Status  string `json:"status"`
	Enabled *bool  `json:"enabled"`
}

// OpsAssignCustomerPlanRequest 是运营后台给客户分配或续期套餐的请求。
type OpsAssignCustomerPlanRequest struct {
	PlanCode  string  `json:"planCode"`
	ExpiresAt int64   `json:"expiresAt"`
	Amount    float64 `json:"amount"`
	Period    string  `json:"period"`
}
