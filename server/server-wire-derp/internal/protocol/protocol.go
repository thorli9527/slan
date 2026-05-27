package protocol

import "time"

type DerpTicket struct {
	TicketID           string    `json:"ticketId"`
	PeerID             string    `json:"peerId"`
	NetworkID          string    `json:"networkId,omitempty"`
	Path               string    `json:"path"`
	RegionID           string    `json:"regionId"`
	NodeID             string    `json:"nodeId"`
	SessionID          string    `json:"sessionId,omitempty"`
	SrcNodeID          string    `json:"srcNodeId,omitempty"`
	DstNodeID          string    `json:"dstNodeId,omitempty"`
	RelayURL           string    `json:"relayUrl,omitempty"`
	SessionKey         string    `json:"sessionKey,omitempty"`
	AllowedDERPNodeIDs []string  `json:"allowedDerpNodeIds,omitempty"`
	ExpiresAt          time.Time `json:"expiresAt"`
	Signature          string    `json:"signature"`
}

type ClientMessage struct {
	Kind         string      `json:"kind"`
	SessionID    string      `json:"sessionId,omitempty"`
	PeerID       string      `json:"peerId,omitempty"`
	NodeID       string      `json:"nodeId,omitempty"`
	RegionID     string      `json:"regionId,omitempty"`
	TargetPeerID string      `json:"targetPeerId,omitempty"`
	Payload      []byte      `json:"payload,omitempty"`
	Ticket       *DerpTicket `json:"ticket,omitempty"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ServerMessage struct {
	Kind           string         `json:"kind"`
	SessionID      string         `json:"sessionId,omitempty"`
	PeerID         string         `json:"peerId,omitempty"`
	SourcePeerID   string         `json:"sourcePeerId,omitempty"`
	RegionID       string         `json:"regionId,omitempty"`
	NodeID         string         `json:"nodeId,omitempty"`
	BytesForwarded int            `json:"bytesForwarded,omitempty"`
	RenewAfterMs   int64          `json:"renewAfterMs,omitempty"`
	Payload        []byte         `json:"payload,omitempty"`
	Error          *ErrorResponse `json:"error,omitempty"`
}
