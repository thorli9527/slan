package biz

import "net/http"

// ExternalHTTPAPI is the service-biz external HTTP surface.
// Implementations provide routing; request/response DTOs below define the
// stable business contracts consumed by clients and consoles.
type ExternalHTTPAPI interface {
	Routes() http.Handler
}

type RegisterUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type AuthEnvelopeResponse struct {
	Auth AuthResponse `json:"auth"`
}

type RegisterUserResponse struct {
	Auth           AuthResponse `json:"auth"`
	DefaultNetwork Network      `json:"defaultNetwork"`
}

type LoginUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LogoutUserRequest struct {
	DeviceToken string `json:"deviceToken"`
}

type CreateConsoleLoginKeyRequest struct {
	DeviceID string `json:"deviceId"`
}

type ConsoleLoginRequest struct {
	LoginKey string `json:"loginKey"`
}

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

type PrepareDeviceLoginResponse struct {
	DeviceID string          `json:"deviceId"`
	LoginURL string          `json:"loginUrl"`
	MQTT     *MQTTCredential `json:"mqtt,omitempty"`
}

type CompleteDeviceLoginRequest struct {
	AccessToken string `json:"accessToken"`
	Token       string `json:"token"`
	Action      string `json:"action"`
}

type CompleteDeviceLoginResponse struct {
	Status     string `json:"status"`
	DeviceID   string `json:"deviceId"`
	DeliveryID string `json:"deliveryId"`
}

type CreateDeviceBootstrapKeyRequest struct {
	UserID      string `json:"userId"`
	NetworkID   string `json:"networkId"`
	DeviceAlias string `json:"deviceAlias"`
	TTLSeconds  int64  `json:"ttlSeconds"`
}

type RevokeDeviceBootstrapKeyRequest struct {
	UserID string `json:"userId"`
}

type DeviceSessionBootstrapRequest struct {
	SessionKey string `json:"sessionKey"`
	DeviceIdentityRequest
}

type DeviceSessionBindRequest = DeviceIdentityRequest

type DeviceRuntimeCountersRequest struct {
	NetworkEnabled bool   `json:"networkEnabled"`
	RxBytesTotal   uint64 `json:"rxBytesTotal"`
	TxBytesTotal   uint64 `json:"txBytesTotal"`
}

type DeviceSessionResponse struct {
	Device         Device          `json:"device"`
	DeviceSession  DeviceSession   `json:"deviceSession"`
	MQTT           *MQTTCredential `json:"mqtt,omitempty"`
	NetworkConfigs ItemsResponse   `json:"networkConfigs"`
}

type ItemsResponse struct {
	Items any `json:"items"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

type RelayCandidatesRequest struct {
	DeviceID string `json:"deviceId"`
}

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

type CreatePunchConnectSessionRequest struct {
	RequesterNodeID string `json:"requesterNodeId"`
	PeerNodeID      string `json:"peerNodeId"`
	TTLSeconds      int    `json:"ttlSeconds"`
}

type ChangeUserPasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type UpsertUserAliasRequest struct {
	OwnerUserID string `json:"ownerUserId"`
	Email       string `json:"email"`
	Alias       string `json:"alias"`
}

type RegisterDeviceRequest struct {
	UserID string `json:"userId"`
	DeviceIdentityRequest
}

type RenewDeviceRequest struct {
	UserID string `json:"userId"`
	DeviceRuntimeCountersRequest
}

type UpdateDeviceAliasRequest struct {
	ActorUserID string `json:"actorUserId"`
	Alias       string `json:"alias"`
}

type DeleteDeviceRequest struct {
	ActorUserID string `json:"actorUserId"`
}

type CreateNetworkRequest struct {
	OwnerUserID string `json:"ownerUserId"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	TemplateKey string `json:"templateKey"`
}

type UpdateNetworkRequest struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

type CreateDeviceInviteRequest struct {
	InviterUserID string `json:"inviterUserId"`
	TTLSeconds    int64  `json:"ttlSeconds"`
}

type AcceptDeviceInviteRequest struct {
	InviteCode  string `json:"inviteCode"`
	DeviceID    string `json:"deviceId"`
	ActorUserID string `json:"actorUserId"`
}

type AddNetworkDeviceRequest struct {
	DeviceID    string `json:"deviceId"`
	ActorUserID string `json:"actorUserId"`
	Alias       string `json:"alias"`
	Enabled     *bool  `json:"enabled"`
}

type UpdateNetworkDeviceRequest struct {
	Alias   string `json:"alias"`
	Enabled *bool  `json:"enabled"`
}

type DNSZoneRequest struct {
	ZoneName     string `json:"zoneName"`
	ExposeGlobal bool   `json:"exposeGlobal"`
}

type AddDNSRecordRequest struct {
	ZoneID string `json:"zoneId"`
	DNSRecordRequest
}

type DNSRecordRequest struct {
	Name           string `json:"name"`
	RecordType     string `json:"recordType"`
	TargetDeviceID string `json:"targetDeviceId"`
	TargetIP       string `json:"targetIp"`
	CNAME          string `json:"cname"`
	Port           string `json:"port"`
	TTL            int    `json:"ttl"`
}

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

type CreateSecurityGroupRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultPolicy string `json:"defaultPolicy"`
}

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

type OpsLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type OpsChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type OpsSetOperatorPasswordRequest struct {
	NewPassword string `json:"newPassword"`
}

type OpsUpdateDeviceRequest struct {
	Alias   string `json:"alias"`
	Status  string `json:"status"`
	Enabled *bool  `json:"enabled"`
}

type OpsAssignCustomerPlanRequest struct {
	PlanCode  string  `json:"planCode"`
	ExpiresAt int64   `json:"expiresAt"`
	Amount    float64 `json:"amount"`
	Period    string  `json:"period"`
}
