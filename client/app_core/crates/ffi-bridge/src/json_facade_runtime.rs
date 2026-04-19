use serde::Deserialize;
use serde_json::{json, Value};
use slan_app_core::ConnectionState;

use crate::facade::{DataPlaneError, DataPlaneErrorCode};

pub fn parse_args<T>(args: Value) -> Result<T, String>
where
    T: for<'de> Deserialize<'de>,
{
    serde_json::from_value(args).map_err(|err| err.to_string())
}

pub fn to_value<T>(value: T) -> Result<Value, String>
where
    T: serde::Serialize,
{
    serde_json::to_value(value).map_err(|err| err.to_string())
}

pub fn connection_state_value(state: ConnectionState) -> Value {
    match state {
        ConnectionState::Disconnected => json!({ "status": "disconnected" }),
        ConnectionState::Connecting => json!({ "status": "connecting" }),
        ConnectionState::Connected(path) => json!({
            "status": "connected",
            "path": match path {
                slan_app_core::ConnectionPath::P2P => "p2p",
                slan_app_core::ConnectionPath::Relay => "relay",
                slan_app_core::ConnectionPath::Derp => "derp",
            },
        }),
        ConnectionState::Failed(reason) => json!({
            "status": "failed",
            "reason": reason,
        }),
    }
}

pub fn data_plane_error_string(operation: &str, error: DataPlaneError) -> String {
    let code = match (operation, error.code) {
        ("send", DataPlaneErrorCode::Timeout) => "send_timeout",
        ("send", DataPlaneErrorCode::Transport) => "send_transport_error",
        ("send", DataPlaneErrorCode::RelayAuth) => "send_relay_auth_error",
        ("send", DataPlaneErrorCode::RelaySession) => "send_relay_session_error",
        ("send", DataPlaneErrorCode::RelayProtocol) => "send_relay_protocol_error",
        ("send", DataPlaneErrorCode::UnsupportedPath) => "send_unsupported_path",
        ("send", DataPlaneErrorCode::Unknown) => "send_failed",
        (_, code) => code.as_str(),
    };
    format!("{code}: {}", error.message)
}
