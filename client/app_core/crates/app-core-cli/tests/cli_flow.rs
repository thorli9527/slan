use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream, UdpSocket};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{SystemTime, UNIX_EPOCH};

use serde_json::{json, Value};

#[test]
fn cli_persists_login_to_bootstrap_connect_flow() {
    let relay = FakeRelayDaemon::start();
    let recorded_requests = Arc::new(Mutex::new(Vec::<RecordedRequest>::new()));
    let server = FakeControlServer::start(
        recorded_requests.clone(),
        vec![
            response(200, login_response()),
            response(200, device_response()),
            response(200, node_response()),
            response(200, bootstrap_response(&relay.relay_url)),
            response(200, relay_ticket_response(&relay.relay_url)),
            response(200, relay_ticket_response(&relay.relay_url)),
            response(200, relay_ticket_response(&relay.relay_url)),
        ],
    );
    let state_file = unique_temp_path("app-core-cli-state.json");

    let login = login_cli(&server.base_url, &state_file);
    assert_eq!(login["userId"], "user-1");
    assert_eq!(login["accessToken"], "token-1");

    let device = register_device_cli(&server.base_url, &state_file);
    assert_eq!(device["deviceId"], "dev-1");

    let node = register_node_cli(&server.base_url, &state_file);
    assert_eq!(node["nodeId"], "node-1");
    assert_eq!(node["deviceId"], "dev-1");

    let bootstrap = bootstrap_cli(&server.base_url, &state_file);
    assert_eq!(bootstrap["relay"]["defaultClusterId"], "cn-local-a");
    assert_eq!(bootstrap["networkMap"]["selfNodeId"], "node-1");

    let connect = run_cli(
        &server.base_url,
        &state_file,
        &["--json", "connect", "--peer-node-id", "peer-1"],
    );
    assert_eq!(connect["status"], "connected");
    assert_eq!(connect["path"], "relay");

    let send = run_cli(
        &server.base_url,
        &state_file,
        &[
            "--json",
            "send",
            "--peer-node-id",
            "peer-1",
            "--payload",
            "hello",
        ],
    );
    assert_eq!(send["status"], "sent");
    assert_eq!(send["bytesSent"], 5);
    assert_eq!(send["path"], "relay");

    let probe = run_cli(
        &server.base_url,
        &state_file,
        &[
            "--json",
            "probe",
            "--peer-node-id",
            "peer-1",
            "--payload",
            "hello",
            "--probe-timeout-ms",
            "0",
        ],
    );
    assert!(probe["probeId"]
        .as_str()
        .expect("probeId string")
        .starts_with("probe-"));
    assert!(probe["sampledAtMs"].as_u64().expect("sampledAtMs u64") > 0);
    assert_eq!(probe["bytesSent"], 5);
    assert_eq!(probe["replyObserved"], false);
    assert_eq!(probe["replyBytesReceived"], Value::Null);
    assert_eq!(probe["replySampledAtMs"], Value::Null);
    assert_eq!(probe["replyRttMs"], Value::Null);
    assert_eq!(probe["activePath"]["relay"]["peer_node_id"], "peer-1");
    assert_eq!(probe["tunnelPeerVirtualIp"], Value::Null);
    assert_eq!(probe["observedRttMs"], Value::Null);
    assert_eq!(probe["packetLossPpm"], Value::Null);
    assert_eq!(probe["pathScore"], Value::Null);
    assert_eq!(probe["derpClusterId"], Value::Null);
    assert_eq!(probe["derpNodeId"], Value::Null);

    let status = run_cli(&server.base_url, &state_file, &["--json", "status"]);
    assert_eq!(status["session"]["userId"], "user-1");
    assert_eq!(status["current_device"]["deviceId"], "dev-1");
    assert_eq!(status["current_node"]["nodeId"], "node-1");
    assert_eq!(status["current_network_id"], "net-1");
    assert_eq!(status["connection_state"]["connected"], "relay");
    assert_eq!(status["active_path"]["relay"]["peer_node_id"], "peer-1");
    assert_eq!(status["tunnel_peer_virtual_ip"], Value::Null);
    assert!(status["last_probe"]["probeId"]
        .as_str()
        .expect("last_probe.probeId string")
        .starts_with("probe-"));
    assert!(
        status["last_probe"]["sampledAtMs"]
            .as_u64()
            .expect("last_probe.sampledAtMs u64")
            > 0
    );
    assert_eq!(status["last_probe"]["bytesSent"], 5);
    assert_eq!(status["last_probe"]["replyObserved"], false);
    assert_eq!(status["last_probe"]["replyBytesReceived"], Value::Null);
    assert_eq!(status["last_probe"]["replySampledAtMs"], Value::Null);
    assert_eq!(status["last_probe"]["replyRttMs"], Value::Null);
    assert_eq!(
        status["last_probe"]["activePath"]["relay"]["peer_node_id"],
        "peer-1"
    );
    assert_eq!(status["last_probe"]["tunnelPeerVirtualIp"], Value::Null);
    assert_eq!(status["last_probe"]["observedRttMs"], Value::Null);
    assert_eq!(status["last_probe"]["packetLossPpm"], Value::Null);
    assert_eq!(status["last_probe"]["pathScore"], Value::Null);
    assert_eq!(status["last_probe"]["derpClusterId"], Value::Null);
    assert_eq!(status["last_probe"]["derpNodeId"], Value::Null);

    let disconnect = run_cli(&server.base_url, &state_file, &["--json", "disconnect"]);
    assert_eq!(disconnect["status"], "disconnected");

    let status_after_disconnect = run_cli(&server.base_url, &state_file, &["--json", "status"]);
    assert_eq!(status_after_disconnect["connection_state"], "disconnected");
    assert_eq!(status_after_disconnect["active_path"], Value::Null);
    assert_eq!(
        status_after_disconnect["tunnel_peer_virtual_ip"],
        Value::Null
    );
    assert_eq!(status_after_disconnect["last_probe"], Value::Null);

    server.join();
    relay.close();

    let requests = recorded_requests.lock().unwrap();
    assert_eq!(requests.len(), 7);
    assert_request(&requests[0], "POST", "/auth/login");
    assert_request(&requests[1], "POST", "/devices/register");
    assert_eq!(requests[1].authorization.as_deref(), Some("Bearer token-1"));
    assert_request(&requests[2], "POST", "/nodes/register");
    assert_request(&requests[3], "POST", "/bootstrap");
    assert_eq!(requests[3].json_body["nodeId"], "node-1");
    assert_eq!(requests[3].json_body["networkId"], "net-1");
    assert_relay_ticket_request(&requests[4], "node-1", "peer-1", "p2p_failed");
    assert_relay_ticket_request(&requests[5], "node-1", "peer-1", "p2p_failed");
    assert_relay_ticket_request(&requests[6], "node-1", "peer-1", "p2p_failed");

    let saved_state: Value =
        serde_json::from_slice(&fs::read(&state_file).expect("read state file")).unwrap();
    assert_disconnected_saved_state(&saved_state, "node-1", "net-1");

    let _ = fs::remove_file(&state_file);
}

#[test]
fn cli_rejects_device_register_without_session() {
    let state_file = unique_temp_path("app-core-cli-no-session.json");
    let failure = run_cli_failure(
        "http://127.0.0.1:9",
        &state_file,
        &[
            "device",
            "register",
            "--name",
            "thor-mac",
            "--platform",
            "macos",
            "--machine-id",
            "machine-1",
            "--public-key",
            "pubkey-1",
        ],
    );
    assert!(failure.contains("missing session, login first"));
}

#[test]
fn cli_rejects_bootstrap_without_node() {
    let recorded_requests = Arc::new(Mutex::new(Vec::<RecordedRequest>::new()));
    let server = FakeControlServer::start(
        recorded_requests.clone(),
        vec![response(200, login_response())],
    );
    let state_file = unique_temp_path("app-core-cli-no-node.json");

    let login = login_cli(&server.base_url, &state_file);
    assert_eq!(login["accessToken"], "token-1");

    let failure = run_cli_failure(
        &server.base_url,
        &state_file,
        &["bootstrap", "--network-id", "net-1"],
    );
    assert!(failure.contains("missing node id; pass --node-id or register a node first"));

    server.join();
    let requests = recorded_requests.lock().unwrap();
    assert_eq!(requests.len(), 1);
    let _ = fs::remove_file(&state_file);
}

#[test]
fn cli_rejects_connect_before_bootstrap() {
    let recorded_requests = Arc::new(Mutex::new(Vec::<RecordedRequest>::new()));
    let server = FakeControlServer::start(
        recorded_requests.clone(),
        vec![
            response(200, login_response()),
            response(200, device_response()),
            response(200, node_response()),
        ],
    );
    let state_file = unique_temp_path("app-core-cli-no-bootstrap.json");

    prepare_registered_node(&server.base_url, &state_file);

    let failure = run_cli_failure(
        &server.base_url,
        &state_file,
        &["connect", "--peer-node-id", "peer-1"],
    );
    assert!(failure.contains("missing network id; pass --network-id or run bootstrap first"));

    server.join();
    let requests = recorded_requests.lock().unwrap();
    assert_eq!(requests.len(), 3);
    let _ = fs::remove_file(&state_file);
}

#[test]
fn cli_surfaces_control_plane_non_2xx_errors() {
    let recorded_requests = Arc::new(Mutex::new(Vec::<RecordedRequest>::new()));
    let server = FakeControlServer::start(
        recorded_requests.clone(),
        vec![response(
            503,
            r#"{"error":"control plane unavailable"}"#.to_string(),
        )],
    );
    let state_file = unique_temp_path("app-core-cli-http-error.json");

    let failure = run_cli_failure(
        &server.base_url,
        &state_file,
        &[
            "auth",
            "login",
            "--email",
            "user@example.com",
            "--password",
            "password123",
        ],
    );
    assert!(failure.contains("unexpected http status: 503"));

    server.join();
    let requests = recorded_requests.lock().unwrap();
    assert_eq!(requests.len(), 1);
}

#[test]
fn cli_surfaces_control_plane_non_2xx_errors_for_auth_register() {
    let recorded_requests = Arc::new(Mutex::new(Vec::<RecordedRequest>::new()));
    let server = FakeControlServer::start(
        recorded_requests.clone(),
        vec![response(
            503,
            r#"{"error":"register unavailable"}"#.to_string(),
        )],
    );
    let state_file = unique_temp_path("app-core-cli-register-http-error.json");

    let failure = run_cli_failure(
        &server.base_url,
        &state_file,
        &[
            "auth",
            "register",
            "--email",
            "user@example.com",
            "--password",
            "password123",
        ],
    );
    assert!(failure.contains("unexpected http status: 503"));

    server.join();
    let requests = recorded_requests.lock().unwrap();
    assert_eq!(requests.len(), 1);
    assert_eq!(requests[0].path, "/auth/register");
}

#[test]
fn cli_rejects_network_create_without_session() {
    let state_file = unique_temp_path("app-core-cli-network-create-no-session.json");

    let failure = run_cli_failure(
        "http://127.0.0.1:9",
        &state_file,
        &[
            "network",
            "create",
            "--name",
            "home",
            "--cidr",
            "100.64.0.0/24",
        ],
    );
    assert!(failure.contains("missing session, login first"));
}

#[test]
fn cli_runs_network_join_alias_switch_and_deactivate_flow() {
    let recorded_requests = Arc::new(Mutex::new(Vec::<RecordedRequest>::new()));
    let server = FakeControlServer::start(
        recorded_requests.clone(),
        vec![
            response(200, login_response()),
            response(200, device_response()),
            response(
                200,
                network_join_response("net-key", "att-key", "100.64.0.11"),
            ),
            response(
                200,
                network_assignment_response("net-key", "att-key", "laptop"),
            ),
            response(
                200,
                network_join_response("net-key", "att-key", "100.64.0.11"),
            ),
            response(
                200,
                network_join_response("net-key", "att-key", "100.64.0.11"),
            ),
            response(200, "{}".to_string()),
        ],
    );
    let state_file = unique_temp_path("app-core-cli-network-lifecycle.json");

    prepare_registered_device(&server.base_url, &state_file);

    let key_join = run_cli(
        &server.base_url,
        &state_file,
        &[
            "--json",
            "network",
            "join-by-key",
            "--join-key",
            "join-key-1",
            "--alias",
            "laptop",
        ],
    );
    assert_eq!(key_join["networkId"], "net-key");
    assert_eq!(key_join["virtualIp"], "100.64.0.11");

    let activated = run_cli(
        &server.base_url,
        &state_file,
        &["--json", "network", "activate", "--network-id", "net-key"],
    );
    assert_eq!(activated["networkId"], "net-key");

    let status_after_activate = run_cli(&server.base_url, &state_file, &["--json", "status"]);
    assert_eq!(status_after_activate["current_network_id"], "net-key");

    let switched = run_cli(
        &server.base_url,
        &state_file,
        &["--json", "network", "switch", "--network-id", "net-key"],
    );
    assert_eq!(switched["networkId"], "net-key");

    let status_after_switch = run_cli(&server.base_url, &state_file, &["--json", "status"]);
    assert_eq!(status_after_switch["current_network_id"], "net-key");

    let deactivated = run_cli(
        &server.base_url,
        &state_file,
        &["--json", "network", "deactivate", "--network-id", "net-key"],
    );
    assert_eq!(deactivated["status"], "deactivated");

    let status_after_deactivate = run_cli(&server.base_url, &state_file, &["--json", "status"]);
    assert_eq!(status_after_deactivate["current_network_id"], Value::Null);

    server.join();
    let requests = recorded_requests.lock().unwrap();
    assert_eq!(requests.len(), 7);
    assert_request(&requests[0], "POST", "/auth/login");
    assert_request(&requests[1], "POST", "/devices/register");
    assert_request(&requests[2], "POST", "/networks/join-by-key");
    assert_eq!(requests[2].json_body["joinKey"], "join-key-1");
    assert_request(
        &requests[3],
        "PUT",
        "/networks/net-key/attachments/att-key/remark",
    );
    assert_eq!(requests[3].json_body["remark"], "laptop");
    assert_request(&requests[4], "POST", "/networks/net-key/activate");
    assert_request(&requests[5], "POST", "/networks/net-key/switch");
    assert_request(&requests[6], "POST", "/networks/net-key/deactivate");

    let _ = fs::remove_file(&state_file);
}

#[test]
fn cli_surfaces_control_plane_non_2xx_errors_for_network_create() {
    let recorded_requests = Arc::new(Mutex::new(Vec::<RecordedRequest>::new()));
    let server = FakeControlServer::start(
        recorded_requests.clone(),
        vec![
            response(200, login_response()),
            response(503, r#"{"error":"network create unavailable"}"#.to_string()),
        ],
    );
    let state_file = unique_temp_path("app-core-cli-network-create-http-error.json");

    let login = login_cli(&server.base_url, &state_file);
    assert_eq!(login["accessToken"], "token-1");

    let failure = run_cli_failure(
        &server.base_url,
        &state_file,
        &[
            "network",
            "create",
            "--name",
            "home",
            "--cidr",
            "100.64.0.0/24",
        ],
    );
    assert!(failure.contains("unexpected http status: 503"));

    server.join();
    let requests = recorded_requests.lock().unwrap();
    assert_eq!(requests.len(), 2);
    assert_request(&requests[1], "POST", "/networks");
    assert_eq!(requests[1].authorization.as_deref(), Some("Bearer token-1"));
}

#[test]
fn cli_rejects_relay_ticket_without_node() {
    let recorded_requests = Arc::new(Mutex::new(Vec::<RecordedRequest>::new()));
    let server = FakeControlServer::start(
        recorded_requests.clone(),
        vec![response(200, login_response())],
    );
    let state_file = unique_temp_path("app-core-cli-relay-ticket-no-node.json");

    let login = login_cli(&server.base_url, &state_file);
    assert_eq!(login["accessToken"], "token-1");

    let failure = run_cli_failure(
        &server.base_url,
        &state_file,
        &[
            "relay-ticket",
            "--network-id",
            "net-1",
            "--dst-node-id",
            "peer-1",
            "--reason",
            "manual_test",
        ],
    );
    assert!(failure.contains("missing node id; pass --node-id or register a node first"));

    server.join();
    let requests = recorded_requests.lock().unwrap();
    assert_eq!(requests.len(), 1);
}

#[test]
fn cli_surfaces_control_plane_non_2xx_errors_for_relay_ticket() {
    let recorded_requests = Arc::new(Mutex::new(Vec::<RecordedRequest>::new()));
    let server = FakeControlServer::start(
        recorded_requests.clone(),
        vec![
            response(200, login_response()),
            response(200, device_response()),
            response(200, node_response()),
            response(503, r#"{"error":"relay ticket unavailable"}"#.to_string()),
        ],
    );
    let state_file = unique_temp_path("app-core-cli-relay-ticket-http-error.json");

    prepare_registered_node(&server.base_url, &state_file);

    let failure = run_cli_failure(
        &server.base_url,
        &state_file,
        &[
            "relay-ticket",
            "--network-id",
            "net-1",
            "--dst-node-id",
            "peer-1",
            "--reason",
            "manual_test",
        ],
    );
    assert!(failure.contains("unexpected http status: 503"));

    server.join();
    let requests = recorded_requests.lock().unwrap();
    assert_eq!(requests.len(), 4);
    assert_request(&requests[3], "POST", "/relay/tickets");
    assert_eq!(requests[3].authorization.as_deref(), Some("Bearer token-1"));
    assert_relay_ticket_request(&requests[3], "node-1", "peer-1", "manual_test");
}

#[test]
fn cli_surfaces_send_failures_after_connect() {
    let relay = FakeRelayDaemon::start();
    let recorded_requests = Arc::new(Mutex::new(Vec::<RecordedRequest>::new()));
    let server = FakeControlServer::start(
        recorded_requests.clone(),
        vec![
            response(200, login_response()),
            response(200, device_response()),
            response(200, node_response()),
            response(200, bootstrap_response(&relay.relay_url)),
            response(200, relay_ticket_response(&relay.relay_url)),
        ],
    );
    let state_file = unique_temp_path("app-core-cli-send-failure.json");

    prepare_bootstrapped_node(&server.base_url, &state_file);

    let failure = run_cli_failure(
        &server.base_url,
        &state_file,
        &["send", "--peer-node-id", "peer-1", "--payload", ""],
    );
    assert!(failure.contains("cannot send empty packet"));

    server.join();
    relay.close();
    let requests = recorded_requests.lock().unwrap();
    assert_eq!(requests.len(), 5);
    assert_request(&requests[4], "POST", "/relay/tickets");
    assert_eq!(requests[4].authorization.as_deref(), Some("Bearer token-1"));

    let _ = fs::remove_file(&state_file);
}

fn run_cli(base_url: &str, state_file: &Path, args: &[&str]) -> Value {
    let output = std::process::Command::new(env!("CARGO_BIN_EXE_app-core-cli"))
        .arg("--control-base-url")
        .arg(base_url)
        .arg("--state-file")
        .arg(state_file)
        .args(args)
        .output()
        .expect("run app-core-cli");
    assert!(
        output.status.success(),
        "stdout:\n{}\nstderr:\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    serde_json::from_slice(&output.stdout).expect("parse cli stdout as json")
}

fn login_cli(base_url: &str, state_file: &Path) -> Value {
    run_cli(
        base_url,
        state_file,
        &[
            "--json",
            "auth",
            "login",
            "--email",
            "user@example.com",
            "--password",
            "password123",
        ],
    )
}

fn register_device_cli(base_url: &str, state_file: &Path) -> Value {
    run_cli(
        base_url,
        state_file,
        &[
            "--json",
            "device",
            "register",
            "--name",
            "thor-mac",
            "--platform",
            "macos",
            "--machine-id",
            "machine-1",
            "--public-key",
            "pubkey-1",
        ],
    )
}

fn register_node_cli(base_url: &str, state_file: &Path) -> Value {
    run_cli(
        base_url,
        state_file,
        &[
            "--json",
            "node",
            "register",
            "--node-id",
            "node-1",
            "--node-public-key",
            "node-pubkey-1",
        ],
    )
}

fn bootstrap_cli(base_url: &str, state_file: &Path) -> Value {
    run_cli(
        base_url,
        state_file,
        &["--json", "bootstrap", "--network-id", "net-1"],
    )
}

fn prepare_registered_node(base_url: &str, state_file: &Path) {
    prepare_registered_device(base_url, state_file);
    register_node_cli(base_url, state_file);
}

fn prepare_registered_device(base_url: &str, state_file: &Path) {
    login_cli(base_url, state_file);
    register_device_cli(base_url, state_file);
}

fn prepare_bootstrapped_node(base_url: &str, state_file: &Path) {
    prepare_registered_node(base_url, state_file);
    bootstrap_cli(base_url, state_file);
}

fn assert_request(request: &RecordedRequest, method: &str, path: &str) {
    assert_eq!(request.method, method);
    assert_eq!(request.path, path);
}

fn assert_relay_ticket_request(
    request: &RecordedRequest,
    src_node_id: &str,
    dst_node_id: &str,
    reason: &str,
) {
    assert_request(request, "POST", "/relay/tickets");
    assert_eq!(request.json_body["srcNodeId"], src_node_id);
    assert_eq!(request.json_body["dstNodeId"], dst_node_id);
    assert_eq!(request.json_body["reason"], reason);
}

fn assert_disconnected_saved_state(saved_state: &Value, _node_id: &str, _network_id: &str) {
    assert_eq!(saved_state["current_node"], Value::Null);
    assert_eq!(saved_state["current_network_id"], Value::Null);
    assert_eq!(saved_state["connection_state"], "disconnected");
    assert_eq!(saved_state["active_path"], Value::Null);
    assert_eq!(saved_state["tunnel_peer_virtual_ip"], Value::Null);
    assert_eq!(saved_state["last_probe"], Value::Null);
}

fn run_cli_failure(base_url: &str, state_file: &Path, args: &[&str]) -> String {
    let output = std::process::Command::new(env!("CARGO_BIN_EXE_app-core-cli"))
        .arg("--control-base-url")
        .arg(base_url)
        .arg("--state-file")
        .arg(state_file)
        .args(args)
        .output()
        .expect("run app-core-cli");
    assert!(
        !output.status.success(),
        "stdout:\n{}\nstderr:\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    String::from_utf8(output.stderr).expect("stderr utf8")
}

fn unique_temp_path(name: &str) -> PathBuf {
    let mut path = std::env::temp_dir();
    let stamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    path.push(format!("{name}.{stamp}.{}", std::process::id()));
    path
}

struct FakeControlServer {
    base_url: String,
    handle: thread::JoinHandle<()>,
}

impl FakeControlServer {
    fn start(
        recorded_requests: Arc<Mutex<Vec<RecordedRequest>>>,
        responses: Vec<FakeResponse>,
    ) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind fake server");
        let address = listener.local_addr().unwrap();
        let base_url = format!("http://{}", address);
        let handle = thread::spawn(move || {
            for response in responses {
                let (stream, _) = listener.accept().expect("accept request");
                handle_connection(stream, &recorded_requests, response);
            }
        });
        Self { base_url, handle }
    }

    fn join(self) {
        self.handle.join().expect("join fake server");
    }
}

struct FakeRelayDaemon {
    relay_url: String,
    stop: Arc<AtomicBool>,
    handle: Option<thread::JoinHandle<()>>,
}

impl FakeRelayDaemon {
    fn start() -> Self {
        let socket = UdpSocket::bind("127.0.0.1:0").expect("bind fake relay daemon");
        socket
            .set_read_timeout(Some(std::time::Duration::from_millis(200)))
            .expect("set fake relay timeout");
        let relay_url = format!("udp://{}", socket.local_addr().unwrap());
        let thread_socket = socket.try_clone().expect("clone fake relay socket");
        let stop = Arc::new(AtomicBool::new(false));
        let thread_stop = stop.clone();
        let handle = thread::spawn(move || {
            let mut buf = [0_u8; 8192];
            loop {
                if thread_stop.load(Ordering::SeqCst) {
                    break;
                }
                match thread_socket.recv_from(&mut buf) {
                    Ok((len, peer)) => {
                        let request: Value =
                            serde_json::from_slice(&buf[..len]).expect("decode relay request");
                        let response = match request["kind"].as_str() {
                            Some("attach") => json!({
                                "kind": "attached",
                                "session_id": request["ticket"]["sessionId"],
                                "peer_participant_id": request["ticket"]["dstNodeId"],
                            }),
                            Some("forward") => {
                                json!({
                                    "kind": "forwarded",
                                    "session_id": request["session_id"],
                                    "to_participant_id": "peer-1",
                                    "bytes_forwarded": 5,
                                })
                            }
                            Some("detach") => json!({
                                "kind": "detached",
                                "session_id": request["session_id"],
                                "participant_id": request["participant_id"],
                            }),
                            Some("ping") => json!({ "kind": "pong" }),
                            other => json!({
                                "kind": "error",
                                "message": format!("unsupported request kind: {:?}", other),
                            }),
                        };
                        let payload = serde_json::to_vec(&response).expect("encode relay response");
                        thread_socket
                            .send_to(&payload, peer)
                            .expect("send relay response");
                    }
                    Err(err)
                        if matches!(
                            err.kind(),
                            std::io::ErrorKind::WouldBlock | std::io::ErrorKind::TimedOut
                        ) => {}
                    Err(_) => break,
                }
            }
        });
        Self {
            relay_url,
            stop,
            handle: Some(handle),
        }
    }

    fn close(mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(handle) = self.handle.take() {
            let _ = handle.join();
        }
    }
}

fn handle_connection(
    mut stream: TcpStream,
    recorded_requests: &Arc<Mutex<Vec<RecordedRequest>>>,
    response: FakeResponse,
) {
    let mut buffer = Vec::new();
    stream.read_to_end(&mut buffer).expect("read request");
    let request = parse_request(&buffer);
    recorded_requests.lock().unwrap().push(request);

    let http_response = format!(
        "HTTP/1.1 {} {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
        response.status,
        reason_phrase(response.status),
        response.body.len(),
        response.body
    );
    stream
        .write_all(http_response.as_bytes())
        .expect("write response");
    stream.flush().expect("flush response");
}

fn login_response() -> String {
    r#"{
        "userId":"user-1",
        "accessToken":"token-1",
        "refreshToken":"refresh-1",
        "expiresIn":3600,
        "deviceId":null
    }"#
    .to_string()
}

fn device_response() -> String {
    r#"{
        "deviceId":"dev-1",
        "name":"thor-mac",
        "platform":"macos",
        "status":"online",
        "publicKey":"pubkey-1"
    }"#
    .to_string()
}

fn node_response() -> String {
    r#"{
        "nodeId":"node-1",
        "deviceId":"dev-1",
        "nodePublicKey":"node-pubkey-1",
        "networkIds":["net-1"],
        "capabilities":[]
    }"#
    .to_string()
}

fn network_join_response(network_id: &str, attachment_id: &str, virtual_ip: &str) -> String {
    format!(
        r#"{{
            "member":{{
                "memberId":"member-1",
                "networkId":"{network_id}",
                "deviceId":"dev-1",
                "role":"member",
                "status":"active"
            }},
            "attachment":{{
                "attachmentId":"{attachment_id}",
                "networkId":"{network_id}",
                "subnetId":"subnet-1",
                "deviceId":"dev-1",
                "virtualIp":"{virtual_ip}",
                "status":"active"
            }}
        }}"#
    )
}

fn network_assignment_response(network_id: &str, attachment_id: &str, remark: &str) -> String {
    format!(
        r#"{{
            "attachmentId":"{attachment_id}",
            "networkId":"{network_id}",
            "subnetId":"subnet-1",
            "deviceId":"dev-1",
            "deviceName":"device-1",
            "userId":"user-1",
            "userEmail":"user@example.com",
            "role":"member",
            "remark":"{remark}",
            "virtualIp":"100.64.0.10",
            "status":"active"
        }}"#
    )
}

fn bootstrap_response(relay_url: &str) -> String {
    r#"{
            "device":{
                "device":{"deviceId":"dev-1","name":"thor-mac","platform":"macos","status":"online","publicKey":"pubkey-1"},
                "attachments":[{
                    "attachmentId":"attach-1",
                    "networkId":"net-1",
                    "subnetId":"subnet-1",
                    "deviceId":"dev-1",
                    "virtualIp":"100.64.0.10",
                    "status":"active"
                }]
            },
            "networks":[{
                "networkId":"net-1",
                "name":"home",
                "defaultSubnetCidr":"100.64.0.0/24",
                "subnets":[{"networkId":"net-1","cidr":"100.64.0.0/24","isDefault":true}],
                "members":[{"deviceId":"dev-1","role":"owner"}]
            }],
            "controlPlane":{"wsUrl":"mqtt://127.0.0.1:1883","heartbeatSeconds":15},
            "stunServers":["stun:stun.l.google.com:19302"],
            "relay":{
                "defaultClusterId":"cn-local-a",
                "countries":[{
                    "countryCode":"CN",
                    "countryName":"China",
                    "cities":[{
                        "cityCode":"local",
                        "cityName":"Local",
                        "clusters":[{
                            "clusterId":"cn-local-a",
                            "clusterName":"CN Local A",
                            "nodes":[{"nodeId":"relay-cn-local-udp","transport":"udp","address":"__RELAY_ADDR__","priority":10}]
                        }]
                    }]
                }]
            },
            "networkMap":{
                "selfUserId":"user-1",
                "selfDeviceId":"dev-1",
                "selfNodeId":"node-1",
                "networkId":"net-1",
                "revision":1,
                "heartbeatSeconds":15,
                "stunServers":[],
                "peers":[{
                    "nodeId":"peer-1",
                    "deviceId":"dev-2",
                    "publicKey":"peer-pk",
                    "status":"online",
                    "relayAllowed":true,
                    "virtualIps":["100.64.0.2"],
                    "endpoints":[{"type":"udp","address":"198.51.100.10:41641","updatedAt":1}],
                    "allowedRoutes":[]
                }],
                "routes":[],
                "relayRegions":[],
                "dns":{"servers":[],"searchDomains":[]},
                "mtu":1280
            }
        }"#
    .replace("__RELAY_ADDR__", relay_url.trim_start_matches("udp://"))
}

fn relay_ticket_response(relay_url: &str) -> String {
    r#"{
        "ticketId":"ticket-1",
        "networkId":"net-1",
        "sessionId":"session-1",
        "srcNodeId":"node-1",
        "dstNodeId":"peer-1",
        "derpClusterId":"cn-local-a",
        "countryCode":"CN",
        "cityCode":"local",
        "allowedDerpNodeIds":["relay-cn-local-udp"],
        "relayUrl":"__RELAY_URL__",
        "expiresAt":"2099-01-01T00:00:00Z",
        "signature":"sig"
    }"#
    .replace("__RELAY_URL__", relay_url)
}

fn response(status: u16, body: String) -> FakeResponse {
    FakeResponse { status, body }
}

fn reason_phrase(status: u16) -> &'static str {
    match status {
        200 => "OK",
        401 => "Unauthorized",
        403 => "Forbidden",
        404 => "Not Found",
        500 => "Internal Server Error",
        503 => "Service Unavailable",
        _ => "Unknown",
    }
}

fn parse_request(raw: &[u8]) -> RecordedRequest {
    let raw_text = String::from_utf8(raw.to_vec()).expect("http request utf8");
    let (header_text, body_text) = raw_text.split_once("\r\n\r\n").expect("header/body");
    let mut lines = header_text.lines();
    let request_line = lines.next().expect("request line");
    let mut request_parts = request_line.split_whitespace();
    let method = request_parts.next().expect("method").to_string();
    let path = request_parts.next().expect("path").to_string();
    let mut authorization = None;
    for line in lines {
        if let Some(value) = line.strip_prefix("Authorization: ") {
            authorization = Some(value.to_string());
        }
    }
    let json_body = if body_text.trim().is_empty() {
        Value::Null
    } else {
        serde_json::from_str(body_text).expect("json body")
    };
    RecordedRequest {
        method,
        path,
        authorization,
        json_body,
    }
}

#[derive(Debug)]
struct RecordedRequest {
    method: String,
    path: String,
    authorization: Option<String>,
    json_body: Value,
}

struct FakeResponse {
    status: u16,
    body: String,
}
