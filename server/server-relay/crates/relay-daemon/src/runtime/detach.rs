use crate::errors::relay_runtime_error;
use crate::protocol::ServerResponse;

use super::{RelayEndpoint, RelayRuntime, RelayRuntimeError};

impl RelayRuntime {
    pub(super) fn handle_detach(
        &mut self,
        source: RelayEndpoint,
        session_id: String,
        participant_id: String,
    ) -> Result<(ServerResponse, Option<(RelayEndpoint, ServerResponse)>), RelayRuntimeError> {
        let should_remove_session = {
            let participants = self.endpoints.get_mut(&session_id).ok_or_else(|| {
                RelayRuntimeError::new(
                    "session_not_attached",
                    "relay session has no attached participants",
                )
            })?;
            let bound_addr = participants.get(&participant_id).ok_or_else(|| {
                RelayRuntimeError::new(
                    "participant_not_attached",
                    "participant has not attached to relay session",
                )
            })?;
            if *bound_addr != source {
                return Err(RelayRuntimeError::new(
                    "participant_address_mismatch",
                    "participant is bound to a different relay endpoint",
                ));
            }
            participants.remove(&participant_id);
            self.source_index.remove(&source);
            participants.is_empty()
        };

        if should_remove_session {
            self.endpoints.remove(&session_id);
            self.relay
                .detach(&session_id)
                .map_err(relay_runtime_error)?;
        }

        Ok((
            ServerResponse::Detached {
                session_id,
                participant_id,
            },
            None,
        ))
    }
}
