use base64::{engine::general_purpose::STANDARD, Engine as _};
use udp_relay::RelayPacket;

use crate::errors::relay_runtime_error;
use crate::protocol::binary;
use crate::protocol::{ForwardAck, RelayPacketMessage, ServerResponse};

use super::{RelayEndpoint, RelayRuntime, RelayRuntimeError};

impl RelayRuntime {
    pub fn handle_binary_data_frame(
        &mut self,
        source: impl Into<RelayEndpoint>,
        frame: &[u8],
    ) -> Result<Option<(RelayEndpoint, Vec<u8>)>, RelayRuntimeError> {
        let source = source.into();
        let decoded = binary::decode_data_frame(frame)
            .map_err(|err| RelayRuntimeError::new("invalid_binary_frame", err))?;
        let (session_id, from_participant_id) =
            self.source_index.get(&source).cloned().ok_or_else(|| {
                RelayRuntimeError::new(
                    "participant_not_attached",
                    "binary relay traffic requires a prior attach from the same udp address",
                )
            })?;

        let forwarded = self
            .relay
            .forward(RelayPacket {
                session_id: session_id.clone(),
                from_device_id: from_participant_id,
                payload: decoded.payload.to_vec(),
            })
            .map_err(relay_runtime_error)?;
        let peer_addr = self
            .endpoints
            .get(&session_id)
            .and_then(|participants| participants.get(&forwarded.to_device_id))
            .copied()
            .ok_or_else(|| {
                RelayRuntimeError::new(
                    "peer_not_attached",
                    "target participant has not attached to relay session",
                )
            })?;
        let peer_frame = binary::encode_data_frame(
            decoded.seq,
            decoded.config_hash,
            forwarded.payload.as_slice(),
        )
        .map_err(|err| RelayRuntimeError::new("encode_binary_frame_failed", err))?;

        Ok(Some((peer_addr, peer_frame)))
    }

    pub(super) fn handle_forward(
        &mut self,
        source: RelayEndpoint,
        session_id: String,
        from_participant_id: String,
        payload_b64: String,
    ) -> Result<(ServerResponse, Option<(RelayEndpoint, ServerResponse)>), RelayRuntimeError> {
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
