use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct AttachAck {
    pub session_id: String,
    pub peer_participant_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ForwardAck {
    pub session_id: String,
    pub to_participant_id: String,
    pub bytes_forwarded: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct RelayPacketMessage {
    pub session_id: String,
    pub from_participant_id: String,
    pub payload_b64: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ErrorResponse {
    pub code: String,
    pub message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum ServerResponse {
    Pong,
    Attached(AttachAck),
    Forwarded(ForwardAck),
    Detached {
        session_id: String,
        participant_id: String,
    },
    Packet(RelayPacketMessage),
    Error(ErrorResponse),
}
