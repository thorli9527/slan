use std::time::{SystemTime, UNIX_EPOCH};

use crate::facade::{DataPlaneError, DataPlaneErrorCode};
use relay_client::PathManagerError;

pub fn classify_path_manager_error(error: PathManagerError) -> DataPlaneError {
    classify_data_plane_error_parts(error.code.as_deref(), error.to_string())
}

fn classify_data_plane_error_parts(code: Option<&str>, error: String) -> DataPlaneError {
    if let Some(code) = code {
        match code {
            "ticket_expired" | "invalid_signature" | "invalid_ticket" | "invalid_timestamp"
            | "unauthorized_peer" | "participant_not_in_ticket" | "clock_skew" => {
                return DataPlaneError::new(DataPlaneErrorCode::RelayAuth, error);
            }
            "session_not_found" | "session_already_exists" | "session_conflict"
            | "session_not_attached" | "participant_not_attached"
            | "participant_address_mismatch" | "peer_not_attached" => {
                return DataPlaneError::new(DataPlaneErrorCode::RelaySession, error);
            }
            "empty_payload" => {
                return DataPlaneError::new(DataPlaneErrorCode::UnsupportedPath, error);
            }
            "invalid_payload_b64" | "store_error" => {
                return DataPlaneError::new(DataPlaneErrorCode::RelayProtocol, error);
            }
            _ => {}
        }
    }
    let normalized = error.to_ascii_lowercase();
    if normalized.contains("timed out") || normalized.contains("timeout") {
        return DataPlaneError::new(DataPlaneErrorCode::Timeout, error);
    }
    if normalized.contains("unsupported") {
        return DataPlaneError::new(DataPlaneErrorCode::UnsupportedPath, error);
    }
    if normalized.contains("transport")
        || normalized.contains("socket")
        || normalized.contains("connection")
    {
        return DataPlaneError::new(DataPlaneErrorCode::Transport, error);
    }
    DataPlaneError::new(DataPlaneErrorCode::Unknown, error)
}

pub fn now_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64
}
