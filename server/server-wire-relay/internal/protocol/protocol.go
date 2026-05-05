package protocol

import "time"

type RelayTicket struct {
	TicketID  string    `json:"ticketId"`
	PeerID    string    `json:"peerId"`
	SessionID string    `json:"sessionId"`
	Path      string    `json:"path"`
	ExpiresAt time.Time `json:"expiresAt"`
	Signature string    `json:"signature"`
}

type ClientMessage struct {
	Kind          string       `json:"kind"`
	SessionID     string       `json:"sessionId,omitempty"`
	ParticipantID string       `json:"participantId,omitempty"`
	Transport     string       `json:"transport,omitempty"`
	Payload       []byte       `json:"payload,omitempty"`
	Ticket        *RelayTicket `json:"ticket,omitempty"`
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
