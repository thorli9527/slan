use std::io::{BufRead, BufReader, Write};
use std::net::{TcpListener, TcpStream};
use std::process::{Command, Stdio};
use std::thread;
use std::time::Duration;

use serde_json::{json, Value};

#[test]
fn helper_process_serializes_success_response_over_stdio() {
    let response = invoke_helper(
        "http://127.0.0.1:9",
        &json!({
            "method": "disconnect",
            "args": {}
        }),
    );

    assert_eq!(response["ok"], true);
    assert_eq!(response["result"], json!({}));
    assert_eq!(response.get("error"), None);
    assert_eq!(response.get("errorCode"), None);
    assert_eq!(response.get("errorMessage"), None);
}

#[test]
fn helper_process_serializes_send_success_response_over_stdio() {
    let response = invoke_helper_with_env(
        "http://127.0.0.1:9",
        &json!({
            "method": "send",
            "args": {
                "payload": "hello"
            }
        }),
        &[(
            "SLAN_APP_CORE_HELPER_TEST_SEND_RESULT",
            r#"{"status":"sent","bytesSent":5,"path":"relay"}"#,
        )],
    );

    assert_eq!(response["ok"], true);
    assert_eq!(response["result"]["status"], "sent");
    assert_eq!(response["result"]["bytesSent"], 5);
    assert_eq!(response["result"]["path"], "relay");
    assert_eq!(response.get("error"), None);
    assert_eq!(response.get("errorCode"), None);
    assert_eq!(response.get("errorMessage"), None);
}

#[test]
fn helper_process_serializes_probe_success_response_over_stdio() {
    let response = invoke_helper_with_env(
        "http://127.0.0.1:9",
        &json!({
            "method": "probe",
            "args": {
                "payload": "hello",
                "probeTimeoutMs": 7
            }
        }),
        &[(
            "SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT",
            r#"{"probeId":"probe-1","sampledAtMs":1,"activePath":{"relay":{"peer_node_id":"peer-1"}},"bytesSent":5,"replyObserved":true,"replyBytesReceived":5,"replySampledAtMs":3,"replyRttMs":2}"#,
        )],
    );

    assert_eq!(response["ok"], true);
    assert_eq!(response["result"]["probeId"], "probe-1");
    assert_eq!(response["result"]["bytesSent"], 5);
    assert_eq!(response["result"]["replyObserved"], true);
    assert_eq!(response["result"]["replyRttMs"], 2);
    assert_eq!(
        response["result"]["activePath"]["relay"]["peer_node_id"],
        "peer-1"
    );
    assert_eq!(response.get("error"), None);
    assert_eq!(response.get("errorCode"), None);
    assert_eq!(response.get("errorMessage"), None);
}

#[test]
fn helper_process_serializes_generic_error_response_over_stdio() {
    let response = invoke_helper(
        "http://127.0.0.1:9",
        &json!({
            "method": "unsupportedMethod",
            "args": {}
        }),
    );

    assert_eq!(response["ok"], false);
    assert_eq!(response["errorCode"], "app_core_helper_error");
    assert_eq!(
        response["errorMessage"],
        "unsupported method: unsupportedMethod"
    );
    assert_eq!(response["error"], "unsupported method: unsupportedMethod");
}

#[test]
fn helper_process_serializes_send_failed_response_over_stdio() {
    let response = invoke_helper(
        "http://127.0.0.1:9",
        &json!({
            "method": "send",
            "args": {
                "payload": "hello"
            }
        }),
    );

    assert_eq!(response["ok"], false);
    assert_eq!(response["errorCode"], "send_failed");
    assert_eq!(response["errorMessage"], "no active path selected");
    assert_eq!(response["error"], "send_failed: no active path selected");
}

#[test]
fn helper_process_serializes_probe_failed_response_over_stdio() {
    let response = invoke_helper(
        "http://127.0.0.1:9",
        &json!({
            "method": "probe",
            "args": {
                "payload": "hello"
            }
        }),
    );

    assert_eq!(response["ok"], false);
    assert_eq!(response["errorCode"], "probe_failed");
    assert_eq!(response["errorMessage"], "no active path selected");
    assert_eq!(response["error"], "probe_failed: no active path selected");
}

#[test]
fn helper_process_serializes_send_unsupported_response_over_stdio() {
    let response = invoke_helper_with_env(
        "http://127.0.0.1:9",
        &json!({
            "method": "send",
            "args": {
                "payload": "hello"
            }
        }),
        &[(
            "SLAN_APP_CORE_HELPER_TEST_SEND_ERROR",
            "send_unsupported_path: active path does not support send",
        )],
    );

    assert_eq!(response["ok"], false);
    assert_eq!(response["errorCode"], "send_unsupported_path");
    assert_eq!(
        response["errorMessage"],
        "active path does not support send"
    );
    assert_eq!(
        response["error"],
        "send_unsupported_path: active path does not support send"
    );
}

#[test]
fn helper_process_serializes_send_transport_response_over_stdio() {
    let response = invoke_helper_with_env(
        "http://127.0.0.1:9",
        &json!({
            "method": "send",
            "args": {
                "payload": "hello"
            }
        }),
        &[(
            "SLAN_APP_CORE_HELPER_TEST_SEND_ERROR",
            "send_transport_error: relay socket write failed",
        )],
    );

    assert_eq!(response["ok"], false);
    assert_eq!(response["errorCode"], "send_transport_error");
    assert_eq!(response["errorMessage"], "relay socket write failed");
    assert_eq!(
        response["error"],
        "send_transport_error: relay socket write failed"
    );
}

#[test]
fn helper_process_serializes_send_timeout_response_over_stdio() {
    let response = invoke_helper_with_env(
        "http://127.0.0.1:9",
        &json!({
            "method": "send",
            "args": {
                "payload": "hello"
            }
        }),
        &[(
            "SLAN_APP_CORE_HELPER_TEST_SEND_ERROR",
            "send_timeout: timed out waiting for send reply",
        )],
    );

    assert_eq!(response["ok"], false);
    assert_eq!(response["errorCode"], "send_timeout");
    assert_eq!(response["errorMessage"], "timed out waiting for send reply");
    assert_eq!(
        response["error"],
        "send_timeout: timed out waiting for send reply"
    );
}

#[test]
fn helper_process_serializes_probe_unsupported_response_over_stdio() {
    let response = invoke_helper_with_env(
        "http://127.0.0.1:9",
        &json!({
            "method": "probe",
            "args": {
                "payload": "hello"
            }
        }),
        &[(
            "SLAN_APP_CORE_HELPER_TEST_PROBE_ERROR",
            "probe_unsupported_path: active path does not support probe",
        )],
    );

    assert_eq!(response["ok"], false);
    assert_eq!(response["errorCode"], "probe_unsupported_path");
    assert_eq!(
        response["errorMessage"],
        "active path does not support probe"
    );
    assert_eq!(
        response["error"],
        "probe_unsupported_path: active path does not support probe"
    );
}

#[test]
fn helper_process_serializes_probe_timeout_response_over_stdio() {
    let response = invoke_helper_with_env(
        "http://127.0.0.1:9",
        &json!({
            "method": "probe",
            "args": {
                "payload": "hello"
            }
        }),
        &[(
            "SLAN_APP_CORE_HELPER_TEST_PROBE_ERROR",
            "probe_timeout: timed out waiting for probe reply",
        )],
    );

    assert_eq!(response["ok"], false);
    assert_eq!(response["errorCode"], "probe_timeout");
    assert_eq!(
        response["errorMessage"],
        "timed out waiting for probe reply"
    );
    assert_eq!(
        response["error"],
        "probe_timeout: timed out waiting for probe reply"
    );
}

#[test]
fn helper_process_serializes_probe_transport_response_over_stdio() {
    let response = invoke_helper_with_env(
        "http://127.0.0.1:9",
        &json!({
            "method": "probe",
            "args": {
                "payload": "hello"
            }
        }),
        &[(
            "SLAN_APP_CORE_HELPER_TEST_PROBE_ERROR",
            "probe_transport_error: relay socket read failed",
        )],
    );

    assert_eq!(response["ok"], false);
    assert_eq!(response["errorCode"], "probe_transport_error");
    assert_eq!(response["errorMessage"], "relay socket read failed");
    assert_eq!(
        response["error"],
        "probe_transport_error: relay socket read failed"
    );
}

#[test]
fn helper_process_serializes_success_response_over_tcp_host() {
    let bind = TcpListener::bind("127.0.0.1:0").expect("bind test port");
    let address = bind.local_addr().expect("tcp host local addr");
    drop(bind);

    let mut command = Command::new(env!("CARGO_BIN_EXE_app-core-helper"));
    command
        .arg("--tcp-host")
        .arg(address.to_string())
        .env("SLAN_CONTROL_BASE_URL", "http://127.0.0.1:9")
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    let mut child = command.spawn().expect("spawn tcp helper");

    let response = invoke_helper_over_tcp(
        address,
        &json!({
            "method": "disconnect",
            "args": {}
        }),
    );

    let _ = child.kill();
    let _ = child.wait();

    assert_eq!(response["ok"], true);
    assert_eq!(response["result"], json!({}));
}

fn invoke_helper(base_url: &str, request: &Value) -> Value {
    invoke_helper_with_env(base_url, request, &[])
}

fn invoke_helper_with_env(base_url: &str, request: &Value, envs: &[(&str, &str)]) -> Value {
    let mut command = Command::new(env!("CARGO_BIN_EXE_app-core-helper"));
    command
        .env("SLAN_CONTROL_BASE_URL", base_url)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped());
    for (key, value) in envs {
        command.env(key, value);
    }
    let mut child = command.spawn().expect("spawn app-core-helper");

    {
        let mut stdin = child.stdin.take().expect("child stdin");
        writeln!(
            stdin,
            "{}",
            serde_json::to_string(request).expect("serialize request")
        )
        .expect("write request");
    }

    let stdout = child.stdout.take().expect("child stdout");
    let mut reader = BufReader::new(stdout);
    let mut line = String::new();
    reader.read_line(&mut line).expect("read response line");
    assert!(
        !line.trim().is_empty(),
        "helper returned empty response line"
    );

    let _ = child.kill();
    let _ = child.wait();

    serde_json::from_str(line.trim()).expect("parse helper json response")
}

fn invoke_helper_over_tcp(address: std::net::SocketAddr, request: &Value) -> Value {
    for _ in 0..50 {
        match TcpStream::connect(address) {
            Ok(mut stream) => {
                writeln!(
                    stream,
                    "{}",
                    serde_json::to_string(request).expect("serialize request")
                )
                .expect("write tcp request");
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                reader.read_line(&mut line).expect("read tcp response line");
                assert!(
                    !line.trim().is_empty(),
                    "helper returned empty tcp response line"
                );
                return serde_json::from_str(line.trim()).expect("parse helper tcp response");
            }
            Err(_) => thread::sleep(Duration::from_millis(20)),
        }
    }
    panic!("timed out waiting for helper tcp host to accept connections");
}
