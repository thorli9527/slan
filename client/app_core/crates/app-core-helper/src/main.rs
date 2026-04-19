use std::io::{self, BufRead, Write};
use std::path::PathBuf;
use std::sync::Arc;

use controller_client::{HttpControllerClient, TcpJsonHttpTransport};
use ffi_bridge::{
    DefaultAppCoreFacade, FileTunnelKeyProvider, JsonAppCoreFacade,
};
use p2p::SocketP2PConnector;
use relay_client::{InMemoryDerpPool, InMemoryPathManager, SocketRelayClient};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use tunnel::InMemoryTunnelManager;

fn main() {
    if let Err(err) = run() {
        let _ = writeln!(io::stderr(), "{err}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let base_url = std::env::var("SLAN_CONTROL_BASE_URL")
        .map_err(|_| "missing SLAN_CONTROL_BASE_URL for app-core-helper".to_string())?;
    let derp_pool = Arc::new(InMemoryDerpPool::default());
    let relay_client = Arc::new(SocketRelayClient::default());
    let p2p_connector = Arc::new(SocketP2PConnector::default());
    let tunnel_manager = Arc::new(InMemoryTunnelManager::default());
    let key_provider: Box<dyn ffi_bridge::TunnelKeyProvider> =
        match std::env::var("SLAN_APP_CORE_TUNNEL_KEY_FILE") {
            Ok(path) => Box::new(FileTunnelKeyProvider::new(PathBuf::from(path))),
            Err(_) => Box::new(ffi_bridge::InMemoryTunnelKeyProvider),
        };
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new_with_tunnel_key_provider(
        HttpControllerClient::new(base_url, TcpJsonHttpTransport::default()),
        p2p_connector.clone(),
        relay_client.clone(),
        derp_pool.clone(),
        InMemoryPathManager::new(derp_pool, relay_client, p2p_connector),
        tunnel_manager,
        key_provider,
    ));

    let stdin = io::stdin();
    let mut stdout = io::stdout().lock();
    for line in stdin.lock().lines() {
        let line = line.map_err(|err| err.to_string())?;
        if line.trim().is_empty() {
            continue;
        }
        let request: RpcRequest = serde_json::from_str(&line).map_err(|err| err.to_string())?;
        let response = build_rpc_response(
            &request.method,
            maybe_test_override_result(&request.method)
                .unwrap_or_else(|| facade.invoke(&request.method, request.args)),
        );
        serde_json::to_writer(&mut stdout, &response).map_err(|err| err.to_string())?;
        stdout.write_all(b"\n").map_err(|err| err.to_string())?;
        stdout.flush().map_err(|err| err.to_string())?;
    }
    Ok(())
}

#[derive(Debug, Deserialize)]
struct RpcRequest {
    method: String,
    #[serde(default)]
    args: Value,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct RpcResponse {
    ok: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    result: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error_code: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error_message: Option<String>,
}

fn build_rpc_response(method: &str, result: Result<Value, String>) -> RpcResponse {
    match result {
        Ok(result) => RpcResponse {
            ok: true,
            result: Some(result),
            error: None,
            error_code: None,
            error_message: None,
        },
        Err(error) => {
            let (error_code, error_message) = classify_helper_error(method, &error);
            RpcResponse {
                ok: false,
                result: None,
                error: Some(error.clone()),
                error_code: Some(error_code),
                error_message: Some(error_message),
            }
        }
    }
}

fn maybe_test_override_result(method: &str) -> Option<Result<Value, String>> {
    let (result_key, error_key) = match method {
        "send" => (
            "SLAN_APP_CORE_HELPER_TEST_SEND_RESULT",
            "SLAN_APP_CORE_HELPER_TEST_SEND_ERROR",
        ),
        "probe" => (
            "SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT",
            "SLAN_APP_CORE_HELPER_TEST_PROBE_ERROR",
        ),
        _ => return None,
    };
    if let Ok(result) = std::env::var(result_key) {
        let parsed = serde_json::from_str(&result)
            .map_err(|err| format!("invalid helper test result json in {result_key}: {err}"));
        return Some(parsed);
    }
    std::env::var(error_key).ok().map(Err)
}

fn classify_helper_error(method: &str, error: &str) -> (String, String) {
    if method != "probe" && method != "send" {
        return ("app_core_helper_error".to_string(), error.to_string());
    }
    if let Some(result) = classify_data_plane_error(error) {
        return result;
    }
    match method {
        "send" => ("send_failed".to_string(), error.to_string()),
        _ => ("probe_failed".to_string(), error.to_string()),
    }
}

fn classify_data_plane_error(error: &str) -> Option<(String, String)> {
    if let Some((code, message)) = error.split_once(": ") {
        if code.starts_with("probe_") || code.starts_with("send_") {
            return Some((code.to_string(), message.to_string()));
        }
    }
    None
}

#[cfg(test)]
mod tests {
    use super::{
        build_rpc_response, classify_data_plane_error, classify_helper_error,
        maybe_test_override_result,
    };
    use serde_json::json;

    #[test]
    fn classifies_probe_timeout_errors() {
        let (code, message) =
            classify_helper_error("probe", "probe_timeout: timed out waiting for probe reply");
        assert_eq!(code, "probe_timeout");
        assert_eq!(message, "timed out waiting for probe reply");
    }

    #[test]
    fn classifies_probe_transport_errors() {
        let (code, _) = classify_helper_error(
            "probe",
            "probe_transport_error: read relay udp socket failed: connection reset",
        );
        assert_eq!(code, "probe_transport_error");
    }

    #[test]
    fn classifies_probe_unsupported_errors() {
        let (code, message) = classify_helper_error(
            "probe",
            "probe_unsupported_path: active path does not support probe",
        );
        assert_eq!(code, "probe_unsupported_path");
        assert_eq!(message, "active path does not support probe");
    }

    #[test]
    fn leaves_non_probe_errors_generic() {
        let (code, _) = classify_helper_error("connect", "missing bootstrap config");
        assert_eq!(code, "app_core_helper_error");
    }

    #[test]
    fn classifies_send_errors_from_probe_prefix() {
        let (code, message) =
            classify_helper_error("send", "send_timeout: timed out waiting for send reply");
        assert_eq!(code, "send_timeout");
        assert_eq!(message, "timed out waiting for send reply");
    }

    #[test]
    fn classifies_send_unsupported_errors() {
        let (code, message) = classify_helper_error(
            "send",
            "send_unsupported_path: active path does not support send",
        );
        assert_eq!(code, "send_unsupported_path");
        assert_eq!(message, "active path does not support send");
    }

    #[test]
    fn falls_back_to_send_failed_for_untyped_send_errors() {
        let (code, message) = classify_helper_error("send", "relay client exploded");
        assert_eq!(code, "send_failed");
        assert_eq!(message, "relay client exploded");
    }

    #[test]
    fn still_accepts_legacy_probe_prefix_for_send_errors() {
        let (code, message) =
            classify_helper_error("send", "probe_timeout: timed out waiting for probe reply");
        assert_eq!(code, "probe_timeout");
        assert_eq!(message, "timed out waiting for probe reply");
    }

    #[test]
    fn classifies_data_plane_error_prefixes_directly() {
        let (code, message) = classify_data_plane_error(
            "send_unsupported_path: active path does not support send",
        )
        .expect("typed data-plane error");
        assert_eq!(code, "send_unsupported_path");
        assert_eq!(message, "active path does not support send");
    }

    #[test]
    fn rpc_response_serializes_send_unsupported_error_fields() {
        let response = build_rpc_response(
            "send",
            Err("send_unsupported_path: active path does not support send".to_string()),
        );
        let json = serde_json::to_value(response).expect("serialize rpc response");
        assert_eq!(json["ok"], false);
        assert_eq!(
            json["error"],
            "send_unsupported_path: active path does not support send"
        );
        assert_eq!(json["errorCode"], "send_unsupported_path");
        assert_eq!(json["errorMessage"], "active path does not support send");
    }

    #[test]
    fn rpc_response_serializes_probe_unsupported_error_fields() {
        let response = build_rpc_response(
            "probe",
            Err("probe_unsupported_path: active path does not support probe".to_string()),
        );
        let json = serde_json::to_value(response).expect("serialize rpc response");
        assert_eq!(json["ok"], false);
        assert_eq!(
            json["error"],
            "probe_unsupported_path: active path does not support probe"
        );
        assert_eq!(json["errorCode"], "probe_unsupported_path");
        assert_eq!(json["errorMessage"], "active path does not support probe");
    }

    #[test]
    fn rpc_response_serializes_success_without_error_fields() {
        let response = build_rpc_response("send", Ok(json!({ "bytesSent": 5 })));
        let json = serde_json::to_value(response).expect("serialize rpc response");
        assert_eq!(json["ok"], true);
        assert_eq!(json["result"]["bytesSent"], 5);
        assert_eq!(json.get("error"), None);
        assert_eq!(json.get("errorCode"), None);
        assert_eq!(json.get("errorMessage"), None);
    }

    #[test]
    fn test_override_returns_send_error_from_env() {
        unsafe {
            std::env::set_var(
                "SLAN_APP_CORE_HELPER_TEST_SEND_ERROR",
                "send_unsupported_path: active path does not support send",
            );
        }
        let result = maybe_test_override_result("send");
        unsafe {
            std::env::remove_var("SLAN_APP_CORE_HELPER_TEST_SEND_ERROR");
        }
        assert_eq!(
            result,
            Some(Err(
                "send_unsupported_path: active path does not support send".to_string()
            ))
        );
    }

    #[test]
    fn test_override_returns_probe_result_from_env() {
        unsafe {
            std::env::set_var(
                "SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT",
                r#"{"probeId":"probe-1","bytesSent":5}"#,
            );
        }
        let result = maybe_test_override_result("probe");
        unsafe {
            std::env::remove_var("SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT");
        }
        assert_eq!(
            result,
            Some(Ok(json!({
                "probeId": "probe-1",
                "bytesSent": 5,
            })))
        );
    }

    #[test]
    fn test_override_returns_invalid_json_error_for_probe_result_env() {
        unsafe {
            std::env::set_var("SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT", "{");
        }
        let result = maybe_test_override_result("probe");
        unsafe {
            std::env::remove_var("SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT");
        }
        assert!(
            result
                .expect("override result")
                .err()
                .expect("override should return parse error")
                .starts_with("invalid helper test result json in SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT:")
        );
    }
}
