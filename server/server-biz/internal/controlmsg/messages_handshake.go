package controlmsg

// Envelope 是控制通道使用的通用消息封装，当前通过 MQTT 传输。
type Envelope struct {
	// Type 标识负载类型，例如 node_hello、network_map_response 或 connect_plan。
	Type string `json:"type"`
	// RequestID 在需要时用于关联请求和响应。
	RequestID string `json:"requestId,omitempty"`
	// MessageID 是服务端下行消息的可靠投递标识。
	MessageID string `json:"messageId,omitempty"`
	// Payload 承载具体消息体。
	Payload interface{} `json:"payload,omitempty"`
}

// NodeHello 是节点在控制通道上的第一条握手消息。
type NodeHello struct {
	// UserID 是所属用户 ID。
	UserID string `json:"userId"`
	// DeviceID 是业务设备 ID。
	DeviceID string `json:"deviceId"`
	// NodeID 是节点 ID。
	NodeID string `json:"nodeId"`
	// NetworkID 是目标网络 ID。
	NetworkID string `json:"networkId"`
	// SessionToken 是控制通道鉴权令牌。
	SessionToken string `json:"sessionToken"`
	// NodePublicKey 是节点公钥。
	NodePublicKey string `json:"nodePublicKey"`
	// Capabilities 是节点声明的能力集合。
	Capabilities []string `json:"capabilities,omitempty"`
}

// NodeHelloAck 用于确认节点握手。
type NodeHelloAck struct {
	// ControlSessionID 是控制通道会话 ID。
	ControlSessionID string `json:"controlSessionId"`
	// HeartbeatSeconds 是建议心跳间隔。
	HeartbeatSeconds int `json:"heartbeatSeconds"`
	// NetworkRevision 是当前网络地图版本号。
	NetworkRevision uint64 `json:"networkRevision"`
}

// Ping 是保活探测消息。
type Ping struct {
	// Timestamp 是发送时间戳。
	Timestamp int64 `json:"timestamp"`
}

// Pong 是保活应答消息。
type Pong struct {
	// Timestamp 是回显时间戳。
	Timestamp int64 `json:"timestamp"`
}

// NetworkMapRequest 用于请求完整网络地图。
type NetworkMapRequest struct {
	// NetworkID 是目标网络 ID。
	NetworkID string `json:"networkId"`
	// LastRevision 是客户端本地已知的地图版本。
	LastRevision uint64 `json:"lastRevision"`
}

// ErrorMessage 是控制通道错误消息。
type ErrorMessage struct {
	// Code 是稳定错误码。
	Code string `json:"code"`
	// Message 是人类可读错误信息。
	Message string `json:"message"`
}
