use serde::{Deserialize, Serialize};

use crate::protocol::RelayTicketWire;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum ClientRequest {
    Ping,
    Attach {
        participant_id: String,
        ticket: RelayTicketWire,
    },
    Forward {
        session_id: String,
        from_participant_id: String,
        payload_b64: String,
    },
    Detach {
        session_id: String,
        participant_id: String,
    },
}
