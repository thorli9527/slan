use std::net::SocketAddr;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use udp_relay::RelayPacket;

use crate::errors::relay_runtime_error;
use crate::protocol::{ForwardAck, RelayPacketMessage, ServerResponse};

use super::{RelayRuntime, RelayRuntimeError};

impl RelayRuntime {
    pub(super) fn handle_forward(
        &mut self,
        source: SocketAddr,
        session_id: String,
        from_participant_id: String,
        payload_b64: String,
    ) -> Result<(ServerResponse, Option<(SocketAddr, ServerResponse)>), RelayRuntimeError> {
        let participants = self.endpoints.get(&session_id).ok_or_else(|| {
            RelayRuntimeError::new(
                "session_not_attached",
                "relay session has no attached participants",
            )
        })?;
        let bound_addr = participants.get(&from_participant_id).ok_or_else(|| {
            RelayRuntimeError::new(
                "participant_not_attached",
                "participant has not attached to relay session",
            )
        })?;
        if *bound_addr != source {
            return Err(RelayRuntimeError::new(
                "participant_address_mismatch",
                "participant is bound to a different udp address",
            ));
        }

        let payload = STANDARD.decode(payload_b64.as_bytes()).map_err(|err| {
            RelayRuntimeError::new("invalid_payload_b64", format!("invalid payload_b64: {err}"))
        })?;
        let forwarded = self
            .relay
            .forward(RelayPacket {
                session_id: session_id.clone(),
                from_device_id: from_participant_id.clone(),
                payload,
            })
            .map_err(relay_runtime_error)?;
        let peer_addr = participants
            .get(&forwarded.to_device_id)
            .copied()
            .ok_or_else(|| {
                RelayRuntimeError::new(
                    "peer_not_attached",
                    "target participant has not attached to relay session",
                )
            })?;
        let packet = ServerResponse::Packet(RelayPacketMessage {
            session_id: forwarded.session_id.clone(),
            from_participant_id,
            payload_b64: STANDARD.encode(&forwarded.payload),
        });

        Ok((
            ServerResponse::Forwarded(ForwardAck {
                session_id: forwarded.session_id,
                to_participant_id: forwarded.to_device_id,
                bytes_forwarded: forwarded.payload.len(),
            }),
            Some((peer_addr, packet)),
        ))
    }
}
