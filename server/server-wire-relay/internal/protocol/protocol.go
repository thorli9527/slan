package protocol

import (
	"encoding/json"
	"strings"
	"time"
)

type RelayTicket struct {
	TicketID     string    `json:"ticketId"`
	PeerID       string    `json:"peerId"`
	SessionID    string    `json:"sessionId"`
	Path         string    `json:"path"`
	ExpiresAt    time.Time `json:"expiresAt"`
	ExpiresAtRaw string    `json:"-"`
	Signature    string    `json:"signature"`
	NetworkID    string    `json:"networkId,omitempty"`
	SrcNodeID    string    `json:"srcNodeId,omitempty"`
	DstNodeID    string    `json:"dstNodeId,omitempty"`
}

func (t *RelayTicket) UnmarshalJSON(data []byte) error {
	var raw struct {
		TicketID       string `json:"ticketId"`
		TicketIDSnake  string `json:"ticket_id"`
		PeerID         string `json:"peerId"`
		PeerIDSnake    string `json:"peer_id"`
		SessionID      string `json:"sessionId"`
		SessionIDSnake string `json:"session_id"`
		Path           string `json:"path"`
		ExpiresAt      string `json:"expiresAt"`
		ExpiresAtSnake string `json:"expires_at"`
		Signature      string `json:"signature"`
		NetworkID      string `json:"networkId"`
		NetworkIDSnake string `json:"network_id"`
		SrcNodeID      string `json:"srcNodeId"`
		SrcNodeIDSnake string `json:"src_node_id"`
		DstNodeID      string `json:"dstNodeId"`
		DstNodeIDSnake string `json:"dst_node_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	t.TicketID = firstNonEmpty(raw.TicketID, raw.TicketIDSnake)
	t.PeerID = firstNonEmpty(raw.PeerID, raw.PeerIDSnake)
	t.SessionID = firstNonEmpty(raw.SessionID, raw.SessionIDSnake)
	t.Path = firstNonEmpty(raw.Path, "relay_udp")
	t.ExpiresAtRaw = firstNonEmpty(raw.ExpiresAt, raw.ExpiresAtSnake)
	t.Signature = raw.Signature
	t.NetworkID = firstNonEmpty(raw.NetworkID, raw.NetworkIDSnake)
	t.SrcNodeID = firstNonEmpty(raw.SrcNodeID, raw.SrcNodeIDSnake)
	t.DstNodeID = firstNonEmpty(raw.DstNodeID, raw.DstNodeIDSnake)
	if t.ExpiresAtRaw != "" {
		if expiresAt, err := time.Parse(time.RFC3339Nano, t.ExpiresAtRaw); err == nil {
			t.ExpiresAt = expiresAt
		}
	}
	return nil
}

type ClientMessage struct {
	Kind          string       `json:"kind"`
	SessionID     string       `json:"sessionId,omitempty"`
	ParticipantID string       `json:"participantId,omitempty"`
	Transport     string       `json:"transport,omitempty"`
	Payload       []byte       `json:"payload,omitempty"`
	Ticket        *RelayTicket `json:"ticket,omitempty"`
}

func (m *ClientMessage) UnmarshalJSON(data []byte) error {
	var raw struct {
		Kind               string       `json:"kind"`
		SessionID          string       `json:"sessionId"`
		SessionIDSnake     string       `json:"session_id"`
		ParticipantID      string       `json:"participantId"`
		ParticipantIDSnake string       `json:"participant_id"`
		Transport          string       `json:"transport"`
		Payload            []byte       `json:"payload"`
		Ticket             *RelayTicket `json:"ticket"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Kind = raw.Kind
	m.SessionID = firstNonEmpty(raw.SessionID, raw.SessionIDSnake)
	m.ParticipantID = firstNonEmpty(raw.ParticipantID, raw.ParticipantIDSnake)
	m.Transport = raw.Transport
	m.Payload = raw.Payload
	m.Ticket = raw.Ticket
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ServerMessage struct {
	Kind              string         `json:"kind"`
	SessionID         string         `json:"sessionId,omitempty"`
	ParticipantID     string         `json:"participantId,omitempty"`
	PeerParticipantID string         `json:"peerParticipantId,omitempty"`
	BytesForwarded    int            `json:"bytesForwarded,omitempty"`
	Payload           []byte         `json:"payload,omitempty"`
	Error             *ErrorResponse `json:"error,omitempty"`
}
