use super::{
    android_data_plane_relay_candidate, apply_prepared_runtime_refresh,
    browser_login_requires_preparation, commit_control_network_activation, commit_logout,
    commit_network_deactivation, commit_prepared_login, connect_plan_content_matches,
    connectivity_recovery_allowed, data_plane_relay_candidate, diagnostic_connect_plan_summaries,
    filter_relay_sessions_for_transport, invalidate_runtime_session, local_status_active_path,
    maintenance_gap_is_resume, method_business_event_type, parse_rfc3339_utc_ms,
    path_diagnose_active_path_counts, path_diagnose_health, path_diagnose_resolver,
    peer_network_id, peer_path_configs, peer_reachability_reconfigure_reason,
    platform_resolver_config, publish_method_business_event,
    relay_candidate_matching_connect_plan_path, relay_maintenance_reconfigure_reason,
    relay_path_candidate_from_connect_plan, relay_reconfigure_backoff_applies,
    relay_reconfigure_bypasses_retry_window, relay_retry_backoff_ms,
    relay_session_from_connect_plan_ticket, relay_session_targets, relay_sessions_missing,
    relay_ticket_should_renew, relay_ticket_timing, relay_transport_for_path_type,
    request_is_watch, rotate_log_file, routes_with_peer_virtual_ips,
    select_relay_sessions_for_candidate, status_is_managed_disabled,
    valid_direct_candidate_address, ControlPeer, LocalRequestMetrics, PersistedConnectPlan,
    PersistedConnectPlanPath, PersistedConnectPlanStore, PreparedControlNetworkActivation,
    RelayMaintenanceState, LOCAL_REQUEST_CONCURRENCY_LIMIT, LOCAL_WATCH_CONCURRENCY_LIMIT,
    RELAY_NO_RX_RECONFIGURE_INTERVALS, RELAY_RESPONSE_GAP_DEGRADED_PACKETS,
};
use crate::control_plane::{
    DeviceNetworkConfig, DeviceNetworkPeer, DeviceResolverConfig, PunchConnectSession,
    PunchEndpoint,
};
use crate::{
    local_api::{
        request_correlation_id, response_with_correlation_id, LocalServiceMethod,
        BUSINESS_NETWORK_RUNTIME_CHANGED, BUSINESS_NETWORK_SWITCH_FAILED, BUSINESS_SESSION_CHANGED,
        BUSINESS_STATE_CHANGED,
    },
    merge_persisted_client_message_into_state, persist_last_client_message_payload,
    relay_candidates::select_relay_candidates,
    relay_models::{
        PathDiagnoseMtu, PathDiagnoseRelay, PathDiagnoseResolver, PersistedRelayCandidate,
        RelayCandidateSelection, RelayRuntimePeerStats, RelayRuntimeStats,
    },
    relay_store::{relay_only_path_policy_enabled, relay_runtime_failure_total},
    runtime_actor::RuntimeActorHandle,
    runtime_event_hub::RuntimeEventHub,
};
use client_core::{
    AssignedIpPayload, ClientCommand, ClientRuntime, ClientViewState, NetworkRuntimeState,
    PathKind, PeerPathRuntime, PlatformNetwork, PlatformNetworkDiagnostics, PlatformResolverConfig,
    RelayDataPlaneConfig, RelayPeerSession, RelayTicket, RouteSpec,
};
use client_core_platform::PlatformNetworkImpl;
use std::{
    fs,
    net::{TcpListener, UdpSocket},
};

#[test]
fn installation_bootstrap_retry_uses_bounded_exponential_backoff() {
    let mut delay = super::INSTALLATION_BOOTSTRAP_RETRY_MIN;
    assert_eq!(delay, std::time::Duration::from_secs(5));
    delay = super::next_installation_bootstrap_retry(delay);
    assert_eq!(delay, std::time::Duration::from_secs(10));
    delay = super::next_installation_bootstrap_retry(std::time::Duration::from_secs(40));
    assert_eq!(delay, super::INSTALLATION_BOOTSTRAP_RETRY_MAX);
    assert_eq!(
        super::next_installation_bootstrap_retry(delay),
        super::INSTALLATION_BOOTSTRAP_RETRY_MAX
    );
}

#[test]
fn browser_login_does_not_replace_an_existing_user_session() {
    let mut signed_in = ClientViewState::default();
    signed_in.signed_in = true;
    signed_in.user_label = Some("user@example.test".to_string());
    signed_in.device_id = Some("device-1".to_string());
    signed_in.network_enabled = true;
    signed_in.virtual_ip = Some("10.0.0.7".to_string());

    assert!(!browser_login_requires_preparation(&signed_in));
    assert!(browser_login_requires_preparation(
        &ClientViewState::default()
    ));
}

#[test]
fn local_request_limits_protect_commands_from_duplicate_watchers() {
    let metrics = LocalRequestMetrics::default();
    for _ in 0..LOCAL_REQUEST_CONCURRENCY_LIMIT {
        assert!(metrics.try_accept());
    }
    assert!(!metrics.try_accept());
    for _ in 0..LOCAL_REQUEST_CONCURRENCY_LIMIT {
        metrics.finish();
    }

    assert!(metrics.try_accept());
    for _ in 0..LOCAL_WATCH_CONCURRENCY_LIMIT {
        assert!(metrics.try_begin_watch());
    }
    assert!(!metrics.try_begin_watch());
    for _ in 0..LOCAL_WATCH_CONCURRENCY_LIMIT {
        metrics.end_watch();
    }
    metrics.finish();

    let diagnostics = metrics.diagnostics();
    assert_eq!(diagnostics.active, 0);
    assert_eq!(diagnostics.active_watches, 0);
    assert_eq!(diagnostics.accepted_total, 65);
    assert_eq!(diagnostics.completed_total, 65);
    assert_eq!(diagnostics.rejected_total, 1);
    assert_eq!(diagnostics.watch_accepted_total, 8);
    assert_eq!(diagnostics.watch_rejected_total, 1);
}

#[test]
fn local_request_watch_classification_only_matches_long_polls() {
    assert!(request_is_watch(
        r#"{"method":"localStateWatch","args":{}}"#
    ));
    assert!(request_is_watch(
        r#"{"method":"localBusinessEventWatch","args":{}}"#
    ));
    assert!(!request_is_watch(
        r#"{"method":"localNetworkActivate","args":{}}"#
    ));
    assert!(!request_is_watch("not-json"));
}

#[test]
fn service_log_rotation_keeps_bounded_backups() {
    let directory = std::env::temp_dir().join(format!(
        "slan-log-rotation-{}-{}",
        std::process::id(),
        crate::session_store::current_timestamp_ms()
    ));
    fs::create_dir_all(&directory).expect("create log rotation directory");
    let log = directory.join("client-core-service.log");

    for generation in 0..4 {
        fs::write(&log, format!("generation-{generation}")).expect("write service log generation");
        rotate_log_file(&log, 1, 3).expect("rotate service log");
    }

    assert!(!log.exists());
    assert_eq!(
        fs::read_to_string(log.with_extension("log.1")).expect("read newest backup"),
        "generation-3"
    );
    assert_eq!(
        fs::read_to_string(log.with_extension("log.3")).expect("read oldest backup"),
        "generation-1"
    );
    assert!(!log.with_extension("log.4").exists());
    let _ = fs::remove_dir_all(directory);
}

#[test]
fn local_request_correlation_uses_first_non_empty_supported_identity() {
    assert_eq!(
        request_correlation_id(&serde_json::json!({
            "requestId": " request-1 ",
            "messageId": "message-1"
        }))
        .as_deref(),
        Some("request-1")
    );
    assert_eq!(
        request_correlation_id(&serde_json::json!({
            "requestId": " ",
            "messageId": "message-1"
        }))
        .as_deref(),
        Some("message-1")
    );
    assert_eq!(request_correlation_id(&serde_json::json!({})), None);
}

#[test]
fn local_response_echoes_correlation_without_overwriting_existing_value() {
    assert_eq!(
        serde_json::from_str::<serde_json::Value>(&response_with_correlation_id(
            r#"{"ok":true}"#,
            Some("request-1")
        ))
        .expect("response json")["requestId"],
        "request-1"
    );
    assert_eq!(
        serde_json::from_str::<serde_json::Value>(&response_with_correlation_id(
            r#"{"requestId":"server-id"}"#,
            Some("request-1")
        ))
        .expect("response json")["requestId"],
        "server-id"
    );
    assert_eq!(
        response_with_correlation_id("plain", Some("request-1")),
        "plain"
    );
}

#[derive(Debug, Clone, Default)]
struct TestPlatformNetwork;

impl PlatformNetwork for TestPlatformNetwork {
    fn install_adapter(&self) -> anyhow::Result<()> {
        Ok(())
    }

    fn configure_ip(&self, _virtual_ip: &str, _prefix_len: u8) -> anyhow::Result<()> {
        Ok(())
    }

    fn configure_routes(&self, _routes: &[RouteSpec]) -> anyhow::Result<()> {
        Ok(())
    }

    fn configure_resolver(&self, _resolver: &PlatformResolverConfig) -> anyhow::Result<()> {
        Ok(())
    }

    fn disable_network(&self) -> anyhow::Result<()> {
        Ok(())
    }

    fn configure_relay(&self, _relay_config: Option<&RelayDataPlaneConfig>) -> anyhow::Result<()> {
        Ok(())
    }

    fn read_runtime_state(&self) -> anyhow::Result<NetworkRuntimeState> {
        Ok(NetworkRuntimeState::default())
    }
}

#[test]
fn online_presence_statuses_do_not_disable_local_network() {
    for status in [
        None,
        Some(""),
        Some("active"),
        Some("online"),
        Some("offline"),
    ] {
        assert!(!status_is_managed_disabled(status));
    }
}

#[test]
fn managed_disable_statuses_disable_local_network() {
    for status in [
        Some("disabled"),
        Some("suspended"),
        Some("blocked"),
        Some("revoked"),
        Some("deleted"),
        Some("removed"),
    ] {
        assert!(status_is_managed_disabled(status));
    }
}

#[test]
fn stale_network_activation_plan_cannot_restore_logged_out_runtime() {
    let mut session = crate::PersistedSession::empty();
    session.device_id = Some("device-1".to_string());
    session.virtual_ip = Some("10.0.0.2".to_string());
    let plan = PreparedControlNetworkActivation {
        session,
        relay_candidates: Vec::new(),
        network_configs: Vec::new(),
        prefix_len: 32,
        resolver: PlatformResolverConfig::default(),
        resolver_zones: Vec::new(),
        resolver_records: Vec::new(),
        routes: Vec::new(),
        relay_config: None,
    };
    let mut runtime = ClientRuntime::new(TestPlatformNetwork);

    let error = commit_control_network_activation(&mut runtime, plan)
        .expect_err("logged out runtime must reject stale activation plan");

    assert!(error.to_string().contains("session changed"));
    assert!(!runtime.state().signed_in);
    assert!(!runtime.state().network_enabled);
}

#[test]
fn stale_network_activation_result_preserves_current_runtime_state() {
    let mut session = crate::PersistedSession::empty();
    session.device_id = Some("old-device".to_string());
    session.virtual_ip = Some("10.0.0.2".to_string());
    let plan = PreparedControlNetworkActivation {
        session,
        relay_candidates: Vec::new(),
        network_configs: Vec::new(),
        prefix_len: 32,
        resolver: PlatformResolverConfig::default(),
        resolver_zones: Vec::new(),
        resolver_records: Vec::new(),
        routes: Vec::new(),
        relay_config: None,
    };
    let mut runtime = ClientRuntime::new(TestPlatformNetwork);
    runtime
        .dispatch(ClientCommand::ApplyDeviceUserLogin(
            client_core::AuthPayload {
                user_authenticated: Some(true),
                access_token: "new-token".to_string(),
                refresh_token: None,
                user_id: "new-user".to_string(),
                user_label: "new@example.test".to_string(),
                device_id: Some("new-device".to_string()),
                active_network_id: None,
                virtual_ip: Some("10.0.0.9".to_string()),
                expires_in: None,
            },
        ))
        .expect("apply current login");

    let committed = super::commit_control_network_activation_result(&mut runtime, Ok(plan));
    let state = committed.state;

    assert_eq!(state.device_id.as_deref(), Some("new-device"));
    assert_eq!(state.virtual_ip, None);
    assert!(!state.network_enabled);
    assert!(state.error.is_none());
    assert!(!committed.rollback_platform);
}

#[test]
fn failed_network_activation_preflight_does_not_request_platform_rollback() {
    let mut runtime = ClientRuntime::new(TestPlatformNetwork);

    let committed = super::commit_control_network_activation_result(
        &mut runtime,
        Err(anyhow::anyhow!("activation preflight failed")),
    );

    assert!(committed.state.error.is_some());
    assert!(!committed.rollback_platform);
}

#[test]
fn stale_prepared_login_cannot_replace_current_runtime_session() {
    let mut runtime = ClientRuntime::new(TestPlatformNetwork);
    runtime
        .dispatch(ClientCommand::ApplyDeviceUserLogin(
            client_core::AuthPayload {
                user_authenticated: Some(true),
                access_token: "new-token".to_string(),
                refresh_token: None,
                user_id: "new-user".to_string(),
                user_label: "new@example.test".to_string(),
                device_id: Some("new-device".to_string()),
                active_network_id: None,
                virtual_ip: Some("10.0.0.9".to_string()),
                expires_in: None,
            },
        ))
        .expect("apply current login");
    let mut stale_session = crate::PersistedSession::empty();
    stale_session.device_id = Some("old-device".to_string());
    stale_session.user_label = "old@example.test".to_string();

    let state = commit_prepared_login(
        &mut runtime,
        Ok(crate::PreparedSession::from_session(stale_session)),
        Some("old-device".to_string()),
        true,
        "login failed",
    );

    assert_eq!(state.device_id.as_deref(), Some("new-device"));
    assert_eq!(state.user_label.as_deref(), Some("new@example.test"));
    assert!(state.signed_in);
}

#[test]
fn stale_invalid_session_result_cannot_logout_new_runtime_session() {
    let runtime = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
    let stale_revision = runtime.snapshot().revision;
    runtime
        .call_named("test.login", None, |runtime| {
            runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(
                client_core::AuthPayload {
                    user_authenticated: Some(true),
                    access_token: "new-token".to_string(),
                    refresh_token: None,
                    user_id: "new-user".to_string(),
                    user_label: "new@example.test".to_string(),
                    device_id: Some("new-device".to_string()),
                    active_network_id: None,
                    virtual_ip: None,
                    expires_in: None,
                },
            ))
        })
        .expect("apply newer runtime session");

    let invalidated =
        invalidate_runtime_session(&runtime, "test.stale.logout", stale_revision, None, false)
            .expect("ignore stale invalid session result");

    assert!(invalidated.is_none());
    let state = runtime.snapshot().state;
    assert!(state.signed_in);
    assert_eq!(state.device_id.as_deref(), Some("new-device"));
}

#[test]
fn stale_logout_cannot_clear_current_runtime_session() {
    let mut runtime = ClientRuntime::new(TestPlatformNetwork);
    runtime
        .dispatch(ClientCommand::ApplyDeviceUserLogin(
            client_core::AuthPayload {
                user_authenticated: Some(true),
                access_token: "new-token".to_string(),
                refresh_token: None,
                user_id: "new-user".to_string(),
                user_label: "new@example.test".to_string(),
                device_id: Some("new-device".to_string()),
                active_network_id: None,
                virtual_ip: Some("10.0.0.9".to_string()),
                expires_in: None,
            },
        ))
        .expect("apply current login");

    let state = commit_logout(&mut runtime, Some("old-device".to_string()), true);

    assert_eq!(state.device_id.as_deref(), Some("new-device"));
    assert_eq!(state.user_label.as_deref(), Some("new@example.test"));
    assert!(state.signed_in);
}

#[test]
fn prepared_runtime_refresh_applies_platform_snapshot_without_platform_read() {
    let mut runtime = ClientRuntime::new(TestPlatformNetwork);
    runtime
        .dispatch(ClientCommand::ApplyDeviceUserLogin(
            client_core::AuthPayload {
                user_authenticated: Some(true),
                access_token: "token".to_string(),
                refresh_token: None,
                user_id: "user".to_string(),
                user_label: "user@example.test".to_string(),
                device_id: Some("device-1".to_string()),
                active_network_id: None,
                virtual_ip: None,
                expires_in: None,
            },
        ))
        .expect("apply login");
    let prepared = NetworkRuntimeState {
        network_enabled: true,
        virtual_ip: Some("10.0.0.8".to_string()),
        ..NetworkRuntimeState::default()
    };

    let state = apply_prepared_runtime_refresh(&mut runtime, Ok(Some(prepared)));

    assert!(state.network_enabled);
    assert_eq!(state.virtual_ip.as_deref(), Some("10.0.0.8"));
}

#[test]
fn prepared_runtime_refresh_preserves_enabled_network_on_transient_disabled_snapshot() {
    let mut runtime = ClientRuntime::new(TestPlatformNetwork);
    runtime
        .dispatch(ClientCommand::ApplyDeviceUserLogin(
            client_core::AuthPayload {
                user_authenticated: Some(true),
                access_token: "token".to_string(),
                refresh_token: None,
                user_id: "user".to_string(),
                user_label: "user@example.test".to_string(),
                device_id: Some("device-1".to_string()),
                active_network_id: Some("network-1".to_string()),
                virtual_ip: Some("10.0.0.8".to_string()),
                expires_in: None,
            },
        ))
        .expect("apply login");
    runtime.apply_network_enabled_state("10.0.0.8".to_string());

    let state =
        apply_prepared_runtime_refresh(&mut runtime, Ok(Some(NetworkRuntimeState::default())));

    assert!(state.network_enabled);
    assert_eq!(state.virtual_ip.as_deref(), Some("10.0.0.8"));
}

#[test]
fn prepared_runtime_refresh_can_keep_disabled_network_disabled() {
    let mut runtime = ClientRuntime::new(TestPlatformNetwork);

    let state =
        apply_prepared_runtime_refresh(&mut runtime, Ok(Some(NetworkRuntimeState::default())));

    assert!(!state.network_enabled);
    assert!(state.virtual_ip.is_none());
}

#[test]
fn dispatch_business_event_type_matches_command_semantics() {
    let state = ClientViewState::default();
    assert_eq!(
        method_business_event_type(
            LocalServiceMethod::Dispatch,
            Some("loginWithPassword"),
            Some(&state),
        ),
        BUSINESS_SESSION_CHANGED
    );
    assert_eq!(
        method_business_event_type(
            LocalServiceMethod::Dispatch,
            Some("syncAssignedIp"),
            Some(&state),
        ),
        BUSINESS_NETWORK_RUNTIME_CHANGED
    );
    assert_eq!(
        method_business_event_type(LocalServiceMethod::Dispatch, Some("refresh"), Some(&state),),
        BUSINESS_STATE_CHANGED
    );

    let mut failed = state.clone();
    failed.error = Some("expected failure".to_string());
    assert_eq!(
        method_business_event_type(
            LocalServiceMethod::Dispatch,
            Some("enableNetwork"),
            Some(&failed),
        ),
        BUSINESS_NETWORK_SWITCH_FAILED
    );
}

#[test]
fn retried_local_request_id_publishes_one_business_event() {
    let hub = RuntimeEventHub::with_capacity(4);
    let request = serde_json::json!({
        "method": "dispatch",
        "args": {
            "type": "refresh",
            "requestId": "local-request-1",
        },
    })
    .to_string();
    let response = serde_json::to_string(&ClientViewState::default()).expect("encode state");

    publish_method_business_event(&hub, &request, &response);
    publish_method_business_event(&hub, &request, &response);

    assert_eq!(hub.latest_revision(), 1);
    let event = hub.next_after(0).expect("published event");
    assert_eq!(event.event_id, "local-request-1");
    assert_eq!(event.business_type, BUSINESS_STATE_CHANGED);
}

#[test]
fn failed_platform_deactivation_preserves_enabled_runtime_state() {
    let mut runtime = ClientRuntime::new(TestPlatformNetwork);
    runtime.apply_network_enabled_state("10.0.0.8".to_string());

    let state = commit_network_deactivation(
        &mut runtime,
        Err(anyhow::anyhow!("platform disable failed")),
    );

    assert!(state.network_enabled);
    assert_eq!(state.virtual_ip.as_deref(), Some("10.0.0.8"));
    assert_eq!(state.error.as_deref(), Some("platform disable failed"));
}

#[test]
fn disabled_network_hides_assigned_device_ip() {
    let mut runtime = ClientRuntime::new(TestPlatformNetwork);
    runtime.apply_assigned_ip_state(AssignedIpPayload {
        virtual_ip: "10.0.0.2".to_string(),
        prefix_len: Some(32),
    });

    let state = runtime.apply_network_disabled_state();

    assert!(!state.network_enabled);
    assert_eq!(state.virtual_ip, None);

    let state = runtime
        .dispatch(ClientCommand::ApplyPlatformRuntimeState(
            NetworkRuntimeState::default(),
        ))
        .expect("apply disabled platform runtime state");
    assert!(!state.network_enabled);
    assert_eq!(state.virtual_ip, None);
}

#[test]
fn sync_assigned_ip_does_not_create_empty_session() {
    let _lock = crate::test_env_lock();
    let state_dir = std::env::temp_dir().join(format!(
        "slan-sync-assigned-ip-test-{}",
        crate::session_store::current_timestamp_ms()
    ));
    let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
    std::env::set_var("SLAN_STATE_DIR", &state_dir);
    let _ = fs::remove_dir_all(&state_dir);

    let mut runtime = ClientRuntime::new(TestPlatformNetwork);
    let payload = AssignedIpPayload {
        virtual_ip: "10.0.0.2".to_string(),
        prefix_len: Some(20),
    };
    assert!(super::prepare_assigned_ip_session(&payload)
        .expect("prepare assigned IP without session")
        .is_none());
    let state = runtime.apply_assigned_ip_state(payload);

    assert_eq!(state.virtual_ip, None);
    assert!(
        crate::session_store::load_session().is_err(),
        "SyncAssignedIp must not create an empty persisted session"
    );

    if let Some(value) = previous_state_dir {
        std::env::set_var("SLAN_STATE_DIR", value);
    } else {
        std::env::remove_var("SLAN_STATE_DIR");
    }
    let _ = fs::remove_dir_all(&state_dir);
}

#[test]
fn persisted_last_client_message_can_restore_view_state_fields() {
    let _lock = crate::test_env_lock();
    let state_dir = std::env::temp_dir().join(format!(
        "slan-last-client-message-test-{}",
        crate::session_store::current_timestamp_ms()
    ));
    let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
    std::env::set_var("SLAN_STATE_DIR", &state_dir);
    let _ = fs::remove_dir_all(&state_dir);

    persist_last_client_message_payload(&serde_json::json!({
        "type": "client_message",
        "payload": {
            "messageId": "client-msg-restore-1",
            "fromDeviceId": "peer-device",
            "body": "hello restore"
        }
    }))
    .expect("persist last client message");

    let mut state = client_core::ClientViewState::default();
    merge_persisted_client_message_into_state(&mut state);

    assert_eq!(
        state.last_client_message_id.as_deref(),
        Some("client-msg-restore-1")
    );
    assert_eq!(
        state.last_client_message_from_device_id.as_deref(),
        Some("peer-device")
    );
    assert_eq!(
        state.last_client_message_body.as_deref(),
        Some("hello restore")
    );

    if let Some(value) = previous_state_dir {
        std::env::set_var("SLAN_STATE_DIR", value);
    } else {
        std::env::remove_var("SLAN_STATE_DIR");
    }
    let _ = fs::remove_dir_all(&state_dir);
}

#[test]
fn relay_selection_prefers_reachable_low_score_candidate() {
    let relay = UdpSocket::bind("127.0.0.1:0").expect("bind test relay udp socket");
    let address = relay.local_addr().unwrap().to_string();
    std::thread::spawn(move || {
        let mut buffer = [0_u8; 512];
        if let Ok((_, peer)) = relay.recv_from(&mut buffer) {
            let _ = relay.send_to(br#"{"kind":"pong"}"#, peer);
        }
    });
    let selections = select_relay_candidates(&[PersistedRelayCandidate {
        endpoint_id: "relay-udp".to_string(),
        transport: "udp".to_string(),
        address,
        country_code: Some("CN".to_string()),
        region_id: None,
        cluster_id: None,
        reachable_hint: false,
        observed_rtt_ms_hint: None,
        path_score_hint: None,
        selected_hint: false,
    }]);
    assert_eq!(selections[0].endpoint_id, "relay-udp");
    assert!(selections[0].selected);
}

#[test]
fn android_data_plane_does_not_select_non_udp_relay() {
    let selected = android_data_plane_relay_candidate(&[PersistedRelayCandidate {
        endpoint_id: "relay-tcp".to_string(),
        transport: "tcp".to_string(),
        address: "127.0.0.1:9001".to_string(),
        country_code: Some("CN".to_string()),
        region_id: None,
        cluster_id: None,
        reachable_hint: false,
        observed_rtt_ms_hint: None,
        path_score_hint: None,
        selected_hint: false,
    }]);

    assert!(selected.is_none());
}

#[test]
fn android_data_plane_selects_derp_when_udp_is_unavailable() {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind derp probe listener");
    let address = listener.local_addr().expect("derp listener address");
    let selected = android_data_plane_relay_candidate(&[PersistedRelayCandidate {
        endpoint_id: "derp-local".to_string(),
        transport: "derp_tcp_tls_443".to_string(),
        address: format!("derp://{address}"),
        country_code: Some("CN".to_string()),
        region_id: Some("dev".to_string()),
        cluster_id: Some("dev".to_string()),
        reachable_hint: false,
        observed_rtt_ms_hint: None,
        path_score_hint: None,
        selected_hint: false,
    }]);

    assert_eq!(
        selected.map(|candidate| candidate.transport),
        Some("derp_tcp_tls_443".to_string())
    );
}

#[test]
fn macos_data_plane_uses_derp_candidate_when_probe_fails() {
    let selected = data_plane_relay_candidate(&[PersistedRelayCandidate {
        endpoint_id: "derp-remote".to_string(),
        transport: "derp_tcp_tls_443".to_string(),
        address: "derp://203.0.113.10:29120".to_string(),
        country_code: Some("CN".to_string()),
        region_id: Some("dev".to_string()),
        cluster_id: Some("dev".to_string()),
        reachable_hint: false,
        observed_rtt_ms_hint: None,
        path_score_hint: None,
        selected_hint: false,
    }]);

    let selected = selected.expect("DERP candidate should be retained after probe failure");
    assert_eq!(selected.endpoint_id, "derp-remote");
    assert_eq!(selected.transport, "derp_tcp_tls_443");
    assert!(!selected.reachable);
    assert!(selected.selected);
}

#[test]
fn relay_selection_preserves_server_selected_hint_when_candidates_are_not_probeable() {
    let selections = select_relay_candidates(&[
        PersistedRelayCandidate {
            endpoint_id: "relay-selected".to_string(),
            transport: "udp".to_string(),
            address: "203.0.113.10:29110".to_string(),
            country_code: Some("CN".to_string()),
            region_id: Some("sha".to_string()),
            cluster_id: Some("cn-a".to_string()),
            reachable_hint: true,
            observed_rtt_ms_hint: Some(18),
            path_score_hint: Some(48),
            selected_hint: true,
        },
        PersistedRelayCandidate {
            endpoint_id: "relay-other".to_string(),
            transport: "udp".to_string(),
            address: "203.0.113.11:29110".to_string(),
            country_code: Some("CN".to_string()),
            region_id: Some("pek".to_string()),
            cluster_id: Some("cn-b".to_string()),
            reachable_hint: true,
            observed_rtt_ms_hint: Some(20),
            path_score_hint: Some(50),
            selected_hint: false,
        },
    ]);

    assert_eq!(selections[0].endpoint_id, "relay-selected");
    assert!(selections[0].selected);
}

#[test]
fn relay_session_targets_keep_selected_probe_fallback() {
    let mut derp = test_relay_selection(
        "derp-fallback",
        "derp_tcp_tls_443",
        "derp://203.0.113.10:29120",
    );
    derp.reachable = false;

    let targets = relay_session_targets(Some(&derp), &[], false);

    assert_eq!(targets.len(), 1);
    assert_eq!(targets[0].endpoint_id, "derp-fallback");
    assert_eq!(targets[0].transport, "derp_tcp_tls_443");
    assert!(!targets[0].reachable);
}

#[test]
fn relay_session_targets_include_udp_and_derp_candidates() {
    let udp = test_relay_selection("relay-udp", "udp", "203.0.113.10:29110");
    let derp = test_relay_selection(
        "relay-derp",
        "derp_tcp_tls_443",
        "derp://203.0.113.10:29120",
    );

    let targets = relay_session_targets(Some(&udp), &[udp.clone(), derp], false);

    let endpoint_ids = targets
        .iter()
        .map(|target| target.endpoint_id.as_str())
        .collect::<Vec<_>>();
    assert_eq!(endpoint_ids, vec!["relay-udp", "relay-derp"]);
}

#[test]
fn relay_session_targets_keep_one_candidate_per_transport() {
    let selected = test_relay_selection(
        "derp-selected",
        "derp_tcp_tls_443",
        "derp://203.0.113.10:29120",
    );
    let alternate = test_relay_selection(
        "derp-alternate",
        "derp_tcp_tls_443",
        "derp://203.0.113.11:29120",
    );

    let targets = relay_session_targets(Some(&selected), &[alternate], false);

    assert_eq!(targets.len(), 1);
    assert_eq!(targets[0].endpoint_id, "derp-selected");
}

#[test]
fn relay_session_filter_rejects_ticket_for_another_transport() {
    let mut ticket = test_relay_ticket("net-1", "node-local", "node-peer");
    ticket.relay_url = "udp://relay.example:29110".to_string();
    let sessions = vec![RelayPeerSession {
        session_id: ticket.session_id.clone(),
        peer_node_id: "node-peer".to_string(),
        peer_virtual_ips: vec!["10.0.0.9".to_string()],
        ticket,
    }];

    assert!(filter_relay_sessions_for_transport(&sessions, "derp_tcp_tls_443").is_empty());
}

#[test]
fn relay_session_filter_keeps_ticket_for_requested_transport() {
    let mut ticket = test_relay_ticket("net-1", "node-local", "node-peer");
    ticket.relay_url = "derp://relay.example:29120".to_string();
    let sessions = vec![RelayPeerSession {
        session_id: ticket.session_id.clone(),
        peer_node_id: "node-peer".to_string(),
        peer_virtual_ips: vec!["10.0.0.9".to_string()],
        ticket,
    }];

    assert_eq!(
        filter_relay_sessions_for_transport(&sessions, "derp_tcp_tls_443").len(),
        1
    );
}

#[test]
fn relay_session_selection_keeps_one_selected_candidate_per_peer() {
    let selected = test_relay_selection("relay-a", "udp", "relay-a.example:29110");
    let mut alternate_ticket = test_relay_ticket("net-1", "node-local", "node-peer");
    alternate_ticket.session_id = "session-alternate".to_string();
    alternate_ticket.relay_url = "udp://relay-b.example:29110".to_string();
    let mut selected_ticket = test_relay_ticket("net-1", "node-local", "node-peer");
    selected_ticket.session_id = "session-selected".to_string();
    selected_ticket.relay_url = format!("udp://{}", selected.address);
    let sessions = vec![
        RelayPeerSession {
            session_id: alternate_ticket.session_id.clone(),
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            ticket: alternate_ticket,
        },
        RelayPeerSession {
            session_id: selected_ticket.session_id.clone(),
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            ticket: selected_ticket,
        },
    ];

    let actual = select_relay_sessions_for_candidate(&sessions, "udp", &selected);

    assert_eq!(actual.len(), 1);
    assert_eq!(actual[0].session_id, "session-selected");
}

#[test]
fn connect_plan_content_ignores_refresh_timestamp() {
    let first = PersistedConnectPlan {
        peer_node_id: "node-peer".to_string(),
        prefer_direct: true,
        paths: vec![PersistedConnectPlanPath {
            path_type: "relay_udp".to_string(),
            endpoint: "udp://relay.example:3478".to_string(),
            priority: 10,
        }],
        relay_ticket: Some(test_relay_ticket("net-1", "node-local", "node-peer")),
        updated_at_ms: 1,
    };
    let mut refreshed = first.clone();
    refreshed.updated_at_ms = 2;

    assert!(connect_plan_content_matches(&first, &refreshed));
    refreshed.paths[0].priority = 20;
    assert!(!connect_plan_content_matches(&first, &refreshed));
}

#[test]
fn connect_plan_relay_path_becomes_path_candidate() {
    let candidate = relay_path_candidate_from_connect_plan(
        &PersistedConnectPlanPath {
            path_type: "relay_udp".to_string(),
            endpoint: "udp://relay.example:3478".to_string(),
            priority: 42,
        },
        &[],
        &test_relay_selection("relay-other", "udp", "127.0.0.1:3478"),
        "node-peer",
    )
    .expect("relay_udp connect plan path should become candidate");

    assert_eq!(candidate.kind, PathKind::RelayUdp);
    assert_eq!(candidate.address.as_deref(), Some("relay.example:3478"));
    assert_eq!(candidate.transport.as_deref(), Some("udp"));
    assert_eq!(candidate.session_id, None);
    assert_eq!(candidate.path_score, Some(42));
}

#[test]
fn connect_plan_relay_path_binds_only_matching_session() {
    let mut ticket = test_relay_ticket("net-1", "node-local", "node-peer");
    ticket.relay_url = "udp://relay.example:3478".to_string();
    let sessions = vec![RelayPeerSession {
        session_id: "session-udp".to_string(),
        peer_node_id: "node-peer".to_string(),
        peer_virtual_ips: vec!["10.0.0.9".to_string()],
        ticket,
    }];

    let candidate = relay_path_candidate_from_connect_plan(
        &PersistedConnectPlanPath {
            path_type: "relay_udp".to_string(),
            endpoint: "udp://relay.example:3478".to_string(),
            priority: 42,
        },
        &sessions,
        &test_relay_selection("relay-other", "udp", "127.0.0.1:3478"),
        "node-peer",
    )
    .expect("relay_udp connect plan path should become candidate");

    assert_eq!(candidate.session_id.as_deref(), Some("session-udp"));

    let derp_candidate = relay_path_candidate_from_connect_plan(
        &PersistedConnectPlanPath {
            path_type: "derp_tcp_tls_443".to_string(),
            endpoint: "derp://relay.example:443".to_string(),
            priority: 42,
        },
        &sessions,
        &test_relay_selection("derp-other", "derp_tcp_tls_443", "relay.example:443"),
        "node-peer",
    )
    .expect("derp connect plan path should become candidate");

    assert_eq!(derp_candidate.session_id, None);
}

#[test]
fn connect_plan_rejects_relay_protocol_aliases() {
    assert!(relay_path_candidate_from_connect_plan(
        &PersistedConnectPlanPath {
            path_type: "relay_udp".to_string(),
            endpoint: "tcp://relay.example:443".to_string(),
            priority: 1,
        },
        &[],
        &test_relay_selection("relay-other", "udp", "127.0.0.1:3478"),
        "node-peer",
    )
    .is_none());
    assert_eq!(relay_transport_for_path_type("relay_udp"), Some("udp"));
    assert_eq!(relay_transport_for_path_type("relay_http3"), None);
    assert_eq!(relay_transport_for_path_type("h3"), None);
    assert_eq!(relay_transport_for_path_type("quic"), None);
}

#[test]
fn connect_plan_path_selects_matching_reachable_relay_candidate() {
    let selected = relay_candidate_matching_connect_plan_path(
        &PersistedConnectPlanPath {
            path_type: "relay_udp".to_string(),
            endpoint: "relay+udp://relay.example:3478".to_string(),
            priority: 1,
        },
        &[test_relay_selection(
            "relay-udp",
            "udp",
            "relay.example:3478",
        )],
    )
    .expect("connect_plan relay path should match candidate");

    assert_eq!(selected.endpoint_id, "relay-udp");
    assert_eq!(selected.transport, "udp");
}

#[test]
fn connect_plan_path_ignores_unreachable_relay_candidate() {
    let mut candidate = test_relay_selection("relay-udp", "udp", "relay.example:3478");
    candidate.reachable = false;

    assert!(relay_candidate_matching_connect_plan_path(
        &PersistedConnectPlanPath {
            path_type: "relay_udp".to_string(),
            endpoint: "udp://relay.example:3478".to_string(),
            priority: 1,
        },
        &[candidate],
    )
    .is_none());
}

#[test]
fn connect_plan_relay_ticket_becomes_peer_session() {
    let peer = test_peer("node-peer", &["10.0.0.9"]);
    let plan = PersistedConnectPlan {
        peer_node_id: peer.node_id.clone(),
        prefer_direct: false,
        paths: Vec::new(),
        relay_ticket: Some(test_relay_ticket("net-1", "node-local", "node-peer")),
        updated_at_ms: 0,
    };

    let session = relay_session_from_connect_plan_ticket(&plan, "net-1", "node-local", &peer)
        .expect("valid connect_plan ticket should become relay session");

    assert_eq!(session.session_id, "session-1");
    assert_eq!(session.peer_node_id, "node-peer");
    assert_eq!(session.peer_virtual_ips, vec!["10.0.0.9".to_string()]);
}

#[test]
fn connect_plan_relay_ticket_must_match_peer() {
    let peer = test_peer("node-peer", &["10.0.0.9"]);
    let plan = PersistedConnectPlan {
        peer_node_id: peer.node_id.clone(),
        prefer_direct: false,
        paths: Vec::new(),
        relay_ticket: Some(test_relay_ticket("net-1", "node-local", "other-peer")),
        updated_at_ms: 0,
    };

    assert!(relay_session_from_connect_plan_ticket(&plan, "net-1", "node-local", &peer).is_none());
}

#[test]
fn punch_connect_session_peer_endpoint_becomes_direct_udp_candidate() {
    let mut peer = test_peer("node-peer", &["10.0.0.9"]);
    peer.endpoints.push(crate::control_plane::ControlEndpoint {
        endpoint_type: "lan_udp".to_string(),
        address: "192.168.1.20:49152".to_string(),
        updated_at: 0,
    });
    let mut punch_sessions = std::collections::BTreeMap::new();
    punch_sessions.insert(
        peer.node_id.clone(),
        PunchConnectSession {
            session_id: "punch-1".to_string(),
            network_id: "net-1".to_string(),
            requester_node_id: "node-local".to_string(),
            peer_node_id: peer.node_id.clone(),
            punch_node_id: "punch-node-1".to_string(),
            requester: None,
            peer: Some(PunchEndpoint {
                network_id: "net-1".to_string(),
                node_id: peer.node_id.clone(),
                endpoint_type: "reflexive".to_string(),
                address: "10.1.1.20:49152".to_string(),
                reflexive: "203.0.113.20:49152".to_string(),
                nat_type: "unknown".to_string(),
            }),
        },
    );

    let paths = peer_path_configs(
        &[peer],
        "node-local",
        &test_relay_selection("relay-udp", "udp", "relay.example:3478"),
        &[],
        &[],
        Some(std::collections::BTreeMap::new()),
        Some(punch_sessions),
    );

    assert_eq!(paths.len(), 1);
    assert_eq!(paths[0].candidates[0].kind, PathKind::LanUdp);
    assert_eq!(
        paths[0].candidates[0].address.as_deref(),
        Some("192.168.1.20:49152")
    );
    assert_eq!(
        paths[0].candidates[1].address.as_deref(),
        Some("203.0.113.20:49152")
    );
}

#[test]
fn peer_endpoint_type_overrides_stale_connect_plan_type() {
    let mut peer = test_peer("node-peer", &["10.0.0.9"]);
    peer.endpoints.push(crate::control_plane::ControlEndpoint {
        endpoint_type: "lan_udp".to_string(),
        address: "192.168.1.20:49152".to_string(),
        updated_at: 0,
    });
    let mut connect_plans = std::collections::BTreeMap::new();
    connect_plans.insert(
        peer.node_id.clone(),
        PersistedConnectPlan {
            peer_node_id: peer.node_id.clone(),
            prefer_direct: true,
            paths: vec![PersistedConnectPlanPath {
                path_type: "direct_udp".to_string(),
                endpoint: "192.168.1.20:49152".to_string(),
                priority: 100,
            }],
            relay_ticket: None,
            updated_at_ms: 1,
        },
    );

    let paths = peer_path_configs(
        &[peer],
        "node-local",
        &test_relay_selection("relay-udp", "udp", "relay.example:3478"),
        &[],
        &[],
        Some(connect_plans),
        Some(std::collections::BTreeMap::new()),
    );

    assert_eq!(paths.len(), 1);
    assert_eq!(paths[0].candidates.len(), 1);
    assert_eq!(paths[0].candidates[0].kind, PathKind::LanUdp);
    assert_eq!(
        paths[0].candidates[0].address.as_deref(),
        Some("192.168.1.20:49152")
    );
}

#[test]
fn peer_path_configs_ignore_zero_port_direct_candidates() {
    let mut peer = test_peer("node-peer", &["10.0.0.9"]);
    peer.endpoints.push(crate::control_plane::ControlEndpoint {
        endpoint_type: "direct_udp".to_string(),
        address: "10.0.0.9:0".to_string(),
        updated_at: 0,
    });
    peer.endpoints.push(crate::control_plane::ControlEndpoint {
        endpoint_type: "direct_udp".to_string(),
        address: "203.0.113.20:49152".to_string(),
        updated_at: 0,
    });
    let mut connect_plans = std::collections::BTreeMap::new();
    connect_plans.insert(
        peer.node_id.clone(),
        PersistedConnectPlan {
            peer_node_id: peer.node_id.clone(),
            prefer_direct: true,
            paths: vec![PersistedConnectPlanPath {
                path_type: "direct_udp".to_string(),
                endpoint: "10.0.0.9:0".to_string(),
                priority: 100,
            }],
            relay_ticket: None,
            updated_at_ms: 1,
        },
    );

    let paths = peer_path_configs(
        &[peer],
        "node-local",
        &test_relay_selection("relay-udp", "udp", "relay.example:3478"),
        &[],
        &[],
        Some(connect_plans),
        Some(std::collections::BTreeMap::new()),
    );

    assert_eq!(paths.len(), 1);
    assert_eq!(paths[0].candidates.len(), 1);
    assert_eq!(
        paths[0].candidates[0].address.as_deref(),
        Some("203.0.113.20:49152")
    );
}

#[test]
fn relay_only_path_policy_enabled_reads_env() {
    let _guard = crate::test_env_lock();
    unsafe {
        std::env::set_var("SLAN_FORCE_RELAY_ONLY", "1");
    }
    assert!(relay_only_path_policy_enabled());
    unsafe {
        std::env::remove_var("SLAN_FORCE_RELAY_ONLY");
    }
}

#[test]
fn local_status_prefers_canonical_platform_path_over_relay_transport() {
    assert_eq!(
        local_status_active_path(Some(&PathKind::DirectUdp), Some("udp".to_string())),
        Some(serde_json::json!("direct_udp"))
    );
    assert_eq!(
        local_status_active_path(None, Some("udp".to_string())),
        Some(serde_json::json!("udp"))
    );
}

#[test]
fn valid_direct_candidate_address_rejects_zero_port() {
    assert!(valid_direct_candidate_address("203.0.113.20:49152"));
    assert!(valid_direct_candidate_address("udp://203.0.113.20:49152"));
    assert!(!valid_direct_candidate_address(
        "relay+udp://203.0.113.20:49152"
    ));
    assert!(!valid_direct_candidate_address("203.0.113.20:0"));
    assert!(!valid_direct_candidate_address("udp://203.0.113.20:0"));
}

#[test]
fn routes_include_peer_virtual_ip_host_routes_for_multi_device_mesh() {
    let network_configs = vec![test_network_config(&[("a", "10.0.0.2"), ("b", "10.0.0.9")])];
    let routes = routes_with_peer_virtual_ips(
        vec![RouteSpec {
            destination: "10.0.0.0/24".to_string(),
            gateway: None,
        }],
        &[
            test_peer("node-a", &["10.0.0.2/32"]),
            test_peer("node-b", &["10.0.0.9"]),
        ],
        &network_configs,
        "10.0.0.2",
    );

    assert!(routes
        .iter()
        .any(|route| route.destination == "10.0.0.0/24"));
    assert!(routes
        .iter()
        .any(|route| route.destination == "10.0.0.9/32"));
    assert!(!routes
        .iter()
        .any(|route| route.destination == "10.0.0.2/32"));
}

#[test]
fn routes_do_not_duplicate_existing_peer_host_routes() {
    let network_configs = vec![test_network_config(&[("b", "10.0.0.9")])];
    let routes = routes_with_peer_virtual_ips(
        vec![RouteSpec {
            destination: "10.0.0.9/32".to_string(),
            gateway: None,
        }],
        &[test_peer("node-b", &["10.0.0.9"])],
        &network_configs,
        "10.0.0.2",
    );

    assert_eq!(
        routes
            .iter()
            .filter(|route| route.destination == "10.0.0.9/32")
            .count(),
        1
    );
}

#[test]
fn routes_use_authoritative_network_ip_instead_of_transport_candidate() {
    let network_configs = vec![test_network_config(&[("peer", "10.0.0.3")])];
    let routes = routes_with_peer_virtual_ips(
        vec![RouteSpec {
            destination: "100.101.50.145/32".to_string(),
            gateway: None,
        }],
        &[test_peer("node-peer", &["100.101.50.145", "10.0.0.3"])],
        &network_configs,
        "10.0.0.2",
    );

    assert_eq!(routes.len(), 1);
    assert_eq!(routes[0].destination, "10.0.0.3/32");
}

#[test]
fn parses_rfc3339_utc_ticket_expiration() {
    assert_eq!(parse_rfc3339_utc_ms("1970-01-01T00:00:01Z"), Some(1_000));
    assert_eq!(
        parse_rfc3339_utc_ms("1970-01-01T00:00:01.500Z"),
        Some(1_000)
    );
    assert_eq!(parse_rfc3339_utc_ms("not-a-date"), None);
}

#[test]
fn relay_ticket_renews_inside_expiration_window() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();

    assert!(relay_ticket_should_renew(now, Some("2026-05-03T10:01:30Z")));
    assert!(relay_ticket_should_renew(now, Some("2026-05-03T10:05:00Z")));
    assert!(!relay_ticket_should_renew(
        now,
        Some("2026-05-03T10:05:01Z")
    ));
}

#[test]
fn relay_ticket_timing_reports_remaining_time_and_due_state() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();

    let timing = relay_ticket_timing(now, Some("2026-05-03T10:01:30Z"));
    assert_eq!(timing.expires_in_ms, Some(90_000));
    assert!(timing.renew_due);

    let expired = relay_ticket_timing(now, Some("2026-05-03T09:59:59Z"));
    assert_eq!(expired.expires_in_ms, Some(-1_000));
    assert!(expired.renew_due);

    let missing = relay_ticket_timing(now, None);
    assert_eq!(missing.expires_in_ms, None);
    assert!(!missing.renew_due);
}

#[test]
fn relay_maintenance_reconfigures_immediately_when_ticket_is_expiring() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let stats = test_relay_stats("2026-05-03T10:01:00Z", now);
    let mut maintenance = RelayMaintenanceState::default();

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        Some("ticket_expiring")
    );
}

#[test]
fn relay_maintenance_marks_expired_ticket_as_urgent() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let stats = test_relay_stats("2026-05-03T09:59:59Z", now);
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(10_000),
        last_failure_total: 0,
        last_attach_failures: 0,
        last_connect_plan_ms: 0,
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        Some("ticket_expired")
    );
    assert!(!relay_reconfigure_backoff_applies("ticket_expired"));
}

#[test]
fn relay_attach_retry_uses_bounded_exponential_backoff() {
    assert_eq!(relay_retry_backoff_ms(1), 30_000);
    assert_eq!(relay_retry_backoff_ms(2), 60_000);
    assert_eq!(relay_retry_backoff_ms(3), 120_000);
    assert_eq!(relay_retry_backoff_ms(10), 600_000);
}

#[test]
fn relay_maintenance_reconfigures_when_peer_sessions_are_missing() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.requested_relay_session_count = 2;
    stats.relay_session_count = 1;
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(5 * 60 * 1000),
        last_failure_total: 0,
        last_attach_failures: 0,
        last_connect_plan_ms: 0,
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        Some("relay_session_missing")
    );
}

#[test]
fn relay_session_missing_uses_attached_peer_sessions_not_transport_total() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.requested_relay_session_count = 2;
    stats.relay_session_count = 2;
    stats.attached_peer_session_count = 1;
    stats.attached_transport_count = 3;

    assert!(relay_sessions_missing(&stats));

    stats.attached_peer_session_count = 2;
    assert!(!relay_sessions_missing(&stats));
}

#[test]
fn relay_maintenance_reconfigures_when_attach_failures_increase() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.relay_attach_failures = 2;
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(5 * 60 * 1000),
        last_failure_total: 0,
        last_attach_failures: 1,
        last_connect_plan_ms: 0,
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        Some("relay_attach_failure")
    );
    assert_eq!(maintenance.last_attach_failures, 2);
}

#[test]
fn relay_maintenance_reconfigures_when_connect_plan_is_newer() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(5 * 60 * 1000),
        last_failure_total: 0,
        last_attach_failures: 0,
        last_connect_plan_ms: now.saturating_sub(60 * 1000),
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, now),
        Some("connect_plan_updated")
    );
    assert_eq!(maintenance.last_connect_plan_ms, now);
}

#[test]
fn connectivity_supervisor_detects_a_resume_sized_scheduler_gap() {
    assert!(!maintenance_gap_is_resume(std::time::Duration::from_secs(
        49
    )));
    assert!(maintenance_gap_is_resume(std::time::Duration::from_secs(
        50
    )));
}

#[test]
fn connectivity_recovery_coalesces_duplicate_signals_during_cooldown() {
    assert!(connectivity_recovery_allowed(100_000, 0));
    assert!(!connectivity_recovery_allowed(129_999, 100_000));
    assert!(connectivity_recovery_allowed(130_000, 100_000));
    assert!(relay_reconfigure_bypasses_retry_window(
        "network_path_changed"
    ));
    assert!(!relay_reconfigure_backoff_applies("network_path_changed"));
}

#[test]
fn peer_reachability_reconfigures_only_after_a_majority_stalls_repeatedly() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.requested_relay_session_count = 3;
    stats.attached_peer_session_count = 3;
    stats.peers = ["node-a", "node-b", "node-c"]
        .into_iter()
        .map(|peer_node_id| RelayRuntimePeerStats {
            peer_node_id: peer_node_id.to_string(),
            session_id: format!("session-{peer_node_id}"),
            peer_virtual_ips: Vec::new(),
            attached: true,
            attach_error: None,
            tun_packets_sent: 0,
            relay_packets_received: 0,
            relay_errors: 0,
            last_relay_error: None,
            last_send_path: Some("relay_udp".to_string()),
            path_downgrades: 0,
            path_upgrades: 0,
            last_path_change: None,
            replayed_frames: 0,
            config_hash_mismatches: 0,
            last_rx_seq: 0,
            send_failures: 0,
            receive_failures: 0,
            wintun_write_failures: 0,
        })
        .collect();
    let mut maintenance = RelayMaintenanceState::default();

    for interval in 1..=2 {
        stats.peers[0].tun_packets_sent = interval;
        stats.peers[1].tun_packets_sent = interval;
        assert_eq!(
            peer_reachability_reconfigure_reason(&stats, &mut maintenance),
            None
        );
    }
    stats.peers[0].tun_packets_sent = 3;
    stats.peers[1].tun_packets_sent = 3;
    assert_eq!(
        peer_reachability_reconfigure_reason(&stats, &mut maintenance),
        Some("peer_reachability_majority_stalled")
    );
}

#[test]
fn peer_reachability_does_not_reconfigure_for_one_stalled_peer_in_a_group() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.requested_relay_session_count = 3;
    stats.attached_peer_session_count = 3;
    stats.peers = ["node-a", "node-b", "node-c"]
        .into_iter()
        .map(|peer_node_id| RelayRuntimePeerStats {
            peer_node_id: peer_node_id.to_string(),
            attached: true,
            ..serde_json::from_value(serde_json::json!({})).unwrap()
        })
        .collect();
    let mut maintenance = RelayMaintenanceState::default();

    for interval in 1..=3 {
        stats.peers[0].tun_packets_sent = interval;
        assert_eq!(
            peer_reachability_reconfigure_reason(&stats, &mut maintenance),
            None
        );
    }
}

#[test]
fn relay_maintenance_does_not_restart_when_peer_stops_returning_packets() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.tun_packets_sent = 10;
    stats.relay_packets_received = 8;
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(5 * 60 * 1000),
        last_tun_packets_sent: 5,
        last_relay_packets_received: 8,
        no_rx_intervals: RELAY_NO_RX_RECONFIGURE_INTERVALS - 1,
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        None
    );
    assert_eq!(
        maintenance.no_rx_intervals,
        RELAY_NO_RX_RECONFIGURE_INTERVALS
    );
}

#[test]
fn relay_maintenance_does_not_count_stall_when_replies_progress() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.tun_packets_sent = 10;
    stats.relay_packets_received = 9;
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(5 * 60 * 1000),
        last_tun_packets_sent: 5,
        last_relay_packets_received: 8,
        no_rx_intervals: RELAY_NO_RX_RECONFIGURE_INTERVALS - 1,
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        None
    );
    assert_eq!(maintenance.no_rx_intervals, 0);
}

#[test]
fn relay_failure_total_excludes_local_packet_noise() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.oversized_tun_packets = 3;
    stats.relay_decode_failures = 2;
    stats.unroutable_tun_packets = 1;

    assert_eq!(relay_runtime_failure_total(&stats), 2);
}

#[test]
fn path_diagnose_resolver_reports_missing_expected_servers() {
    let resolver = path_diagnose_resolver(
        &["10.0.0.1".to_string(), "8.8.8.8".to_string()],
        &["10.0.0.1".to_string()],
    );

    assert!(resolver.checked);
    assert_eq!(resolver.ok, Some(false));
    assert_eq!(resolver.missing_servers, vec!["8.8.8.8".to_string()]);
}

#[test]
fn platform_resolver_uses_client_owned_service_address_for_managed_domains() {
    let resolver = platform_resolver_config(
        "net-1",
        &DeviceResolverConfig {
            servers: vec!["8.8.8.8".to_string()],
            split_domains: vec!["tt.com".to_string()],
            ..DeviceResolverConfig::default()
        },
        &[],
    );

    assert_eq!(resolver.servers, vec![client_core::SLAN_DNS_SERVICE_IP]);
    assert_eq!(resolver.split_domains, vec!["tt.com"]);
    assert!(!resolver.fallback_to_system_resolvers);
}

#[test]
fn path_diagnose_counts_active_paths_by_peer() {
    let counts = path_diagnose_active_path_counts(&[
        test_peer_path("node-a", Some(PathKind::DirectUdp)),
        test_peer_path("node-b", Some(PathKind::RelayUdp)),
        test_peer_path("node-c", Some(PathKind::DirectUdp)),
        test_peer_path("node-d", None),
    ]);

    assert_eq!(counts.len(), 3);
    assert_eq!(counts[0].path_type, "direct_udp");
    assert_eq!(counts[0].count, 2);
    assert!(counts
        .iter()
        .any(|item| item.path_type == "direct_udp" && item.count == 2));
    assert!(counts
        .iter()
        .any(|item| item.path_type == "relay_udp" && item.count == 1));
    assert!(counts
        .iter()
        .any(|item| item.path_type == "unknown" && item.count == 1));
}

#[test]
fn path_diagnose_health_fails_on_missing_attached_peer_sessions() {
    let relay = PathDiagnoseRelay {
        address: "127.0.0.1:3478".to_string(),
        transport: Some("udp".to_string()),
        active_path: Some("relay_udp".to_string()),
        requested_relay_session_count: 2,
        relay_session_count: 2,
        attached_peer_session_count: 1,
        attached_transport_count: 3,
        ticket_expires_at: None,
        ticket_expires_in_ms: Some(60_000),
        ticket_renew_due: false,
        relay_attach_failures: 1,
        last_relay_attach_error: Some("attach failed".to_string()),
        peers: Vec::new(),
        relay_mtu: Some(1280),
        max_frame_payload: Some(1200),
        tun_packets_sent: 0,
        relay_packets_received: 0,
        relay_error_responses: 0,
        relay_config_hash_mismatches: 0,
        last_relay_error: None,
        failures: 0,
        unroutable_tun_packets: 0,
        last_unroutable_destination: None,
        oversized_tun_packets: 0,
        last_oversized_tun_packet_size: None,
        started_at_ms: 0,
        last_tun_packet_at_ms: None,
        last_relay_packet_at_ms: None,
        last_relay_keepalive_at_ms: None,
        updated_at_ms: 0,
        stale: false,
    };
    let relay_candidates = vec![test_relay_selection("relay-udp", "udp", "127.0.0.1:3478")];
    let peer_paths = vec![test_peer_path("node-peer", Some(PathKind::RelayUdp))];

    let health = path_diagnose_health(
        Some(&relay),
        &PathDiagnoseMtu::default(),
        &PathDiagnoseResolver::default(),
        &PlatformNetworkDiagnostics::default(),
        &relay_candidates,
        &peer_paths,
    );

    assert_eq!(health.status, "failed");
    assert!(health
        .reasons
        .iter()
        .any(|reason| reason.code == "relay_peer_sessions_not_attached"));
}

#[test]
fn path_diagnose_health_reports_ok_for_clean_relay() {
    let relay = PathDiagnoseRelay {
        address: "127.0.0.1:3478".to_string(),
        transport: Some("udp".to_string()),
        active_path: Some("relay_udp".to_string()),
        requested_relay_session_count: 1,
        relay_session_count: 1,
        attached_peer_session_count: 1,
        attached_transport_count: 1,
        ticket_expires_at: None,
        ticket_expires_in_ms: Some(60_000),
        ticket_renew_due: false,
        relay_attach_failures: 0,
        last_relay_attach_error: None,
        peers: Vec::new(),
        relay_mtu: Some(1280),
        max_frame_payload: Some(1200),
        tun_packets_sent: 0,
        relay_packets_received: 0,
        relay_error_responses: 0,
        relay_config_hash_mismatches: 0,
        last_relay_error: None,
        failures: 0,
        unroutable_tun_packets: 0,
        last_unroutable_destination: None,
        oversized_tun_packets: 0,
        last_oversized_tun_packet_size: None,
        started_at_ms: 0,
        last_tun_packet_at_ms: None,
        last_relay_packet_at_ms: None,
        last_relay_keepalive_at_ms: None,
        updated_at_ms: 0,
        stale: false,
    };
    let relay_candidates = vec![test_relay_selection("relay-udp", "udp", "127.0.0.1:3478")];
    let peer_paths = vec![test_peer_path("node-peer", Some(PathKind::RelayUdp))];

    let health = path_diagnose_health(
        Some(&relay),
        &PathDiagnoseMtu::default(),
        &PathDiagnoseResolver::default(),
        &PlatformNetworkDiagnostics::default(),
        &relay_candidates,
        &peer_paths,
    );

    assert_eq!(health.status, "ok");
    assert!(health.reasons.is_empty());
}

#[test]
fn path_diagnose_health_reports_degraded_for_relay_response_gap() {
    let relay = PathDiagnoseRelay {
        address: "127.0.0.1:3478".to_string(),
        transport: Some("udp".to_string()),
        active_path: Some("relay_udp".to_string()),
        requested_relay_session_count: 1,
        relay_session_count: 1,
        attached_peer_session_count: 1,
        attached_transport_count: 1,
        ticket_expires_at: None,
        ticket_expires_in_ms: Some(60_000),
        ticket_renew_due: false,
        relay_attach_failures: 0,
        last_relay_attach_error: None,
        peers: Vec::new(),
        relay_mtu: Some(1280),
        max_frame_payload: Some(1200),
        tun_packets_sent: RELAY_RESPONSE_GAP_DEGRADED_PACKETS + 5,
        relay_packets_received: 5,
        relay_error_responses: 0,
        relay_config_hash_mismatches: 0,
        last_relay_error: None,
        failures: 0,
        unroutable_tun_packets: 0,
        last_unroutable_destination: None,
        oversized_tun_packets: 0,
        last_oversized_tun_packet_size: None,
        started_at_ms: 0,
        last_tun_packet_at_ms: Some(2),
        last_relay_packet_at_ms: Some(1),
        last_relay_keepalive_at_ms: None,
        updated_at_ms: 0,
        stale: false,
    };
    let relay_candidates = vec![test_relay_selection("relay-udp", "udp", "127.0.0.1:3478")];
    let peer_paths = vec![test_peer_path("node-peer", Some(PathKind::RelayUdp))];

    let health = path_diagnose_health(
        Some(&relay),
        &PathDiagnoseMtu::default(),
        &PathDiagnoseResolver::default(),
        &PlatformNetworkDiagnostics::default(),
        &relay_candidates,
        &peer_paths,
    );

    assert_eq!(health.status, "degraded");
    assert!(health
        .reasons
        .iter()
        .any(|reason| reason.code == "relay_response_gap"));
}

#[test]
fn path_diagnose_health_accepts_runtime_peer_paths_without_stats() {
    let relay_candidates = vec![test_relay_selection("relay-udp", "udp", "127.0.0.1:3478")];
    let peer_paths = vec![test_peer_path("node-peer", Some(PathKind::RelayUdp))];

    let health = path_diagnose_health(
        None,
        &PathDiagnoseMtu::default(),
        &PathDiagnoseResolver::default(),
        &PlatformNetworkDiagnostics::default(),
        &relay_candidates,
        &peer_paths,
    );

    assert_eq!(health.status, "ok");
    assert!(health.reasons.is_empty());
}

#[test]
fn diagnostic_connect_plan_summary_redacts_ticket_secret_fields() {
    let store = PersistedConnectPlanStore {
        plans: vec![PersistedConnectPlan {
            peer_node_id: "node-peer".to_string(),
            prefer_direct: true,
            paths: vec![PersistedConnectPlanPath {
                path_type: "relay_udp".to_string(),
                endpoint: "udp://relay.example:3478".to_string(),
                priority: 10,
            }],
            relay_ticket: Some(test_relay_ticket("net-1", "node-local", "node-peer")),
            updated_at_ms: 1_000,
        }],
    };

    let summaries = diagnostic_connect_plan_summaries(&store, 2_000);
    let encoded = serde_json::to_string(&summaries).expect("encode connect plan summaries");

    assert_eq!(summaries.len(), 1);
    assert!(encoded.contains("relayTicketExpiresAt"));
    assert!(encoded.contains("hasRelayTicket"));
    assert!(!encoded.contains("session-key"));
    assert!(!encoded.contains("signature"));
    assert!(!encoded.contains("ticket-1"));
}

fn test_peer(node_id: &str, virtual_ips: &[&str]) -> ControlPeer {
    ControlPeer {
        node_id: node_id.to_string(),
        virtual_ips: virtual_ips.iter().map(|value| value.to_string()).collect(),
        relay_allowed: true,
        endpoints: Vec::new(),
    }
}

fn test_network_config(peers: &[(&str, &str)]) -> DeviceNetworkConfig {
    DeviceNetworkConfig {
        peers: peers
            .iter()
            .map(|(device_id, global_ip)| DeviceNetworkPeer {
                device_id: (*device_id).to_string(),
                global_ip: Some((*global_ip).to_string()),
                ..DeviceNetworkPeer::default()
            })
            .collect(),
        ..DeviceNetworkConfig::default()
    }
}

#[test]
fn peer_network_id_uses_the_network_containing_the_peer() {
    let mut primary = test_network_config(&[("peer-primary", "10.0.0.2")]);
    primary.network_id = "network-primary".to_string();
    let mut shared = test_network_config(&[("peer-shared", "10.0.0.3")]);
    shared.network_id = "network-shared".to_string();
    let configs = vec![primary, shared];

    assert_eq!(
        peer_network_id(
            "network-primary",
            &test_peer("peer-shared", &["10.0.0.3"]),
            &configs,
        ),
        "network-shared",
    );
    assert_eq!(
        peer_network_id(
            "network-primary",
            &test_peer("peer-unknown", &["10.0.0.4"]),
            &configs,
        ),
        "network-primary",
    );
}

fn test_peer_path(node_id: &str, active_path: Option<PathKind>) -> PeerPathRuntime {
    PeerPathRuntime {
        peer_node_id: node_id.to_string(),
        peer_virtual_ips: Vec::new(),
        active_path,
        candidates: Vec::new(),
    }
}

fn test_relay_selection(
    endpoint_id: &str,
    transport: &str,
    address: &str,
) -> RelayCandidateSelection {
    RelayCandidateSelection {
        endpoint_id: endpoint_id.to_string(),
        transport: transport.to_string(),
        address: address.to_string(),
        country_code: None,
        region_id: None,
        cluster_id: None,
        reachable: true,
        rtt_ms: None,
        path_score: 0,
        selected: true,
    }
}

fn test_relay_ticket(network_id: &str, src_node_id: &str, dst_node_id: &str) -> RelayTicket {
    RelayTicket {
        ticket_id: "ticket-1".to_string(),
        network_id: network_id.to_string(),
        session_id: "session-1".to_string(),
        src_node_id: src_node_id.to_string(),
        dst_node_id: dst_node_id.to_string(),
        derp_cluster_id: Some("cluster-1".to_string()),
        country_code: None,
        city_code: None,
        allowed_derp_node_ids: vec!["relay-1".to_string()],
        relay_url: "udp://relay.example:3478".to_string(),
        expires_at: "2099-01-01T00:00:00Z".to_string(),
        session_key: "session-key".to_string(),
        signature: "signature".to_string(),
    }
}

fn test_relay_stats(ticket_expires_at: &str, updated_at_ms: u64) -> RelayRuntimeStats {
    RelayRuntimeStats {
        relay_address: "127.0.0.1:3478".to_string(),
        relay_transport: Some("udp".to_string()),
        active_path: Some("relay_udp".to_string()),
        requested_relay_session_count: 1,
        relay_session_count: 1,
        attached_peer_session_count: 1,
        attached_transport_count: 1,
        ticket_expires_at: Some(ticket_expires_at.to_string()),
        ticket_expires_in_ms: None,
        ticket_renew_due: false,
        relay_attach_failures: 0,
        last_relay_attach_error: None,
        peers: Vec::new(),
        relay_mtu: Some(1280),
        max_frame_payload: Some(1200),
        tun_packets_sent: 0,
        relay_packets_received: 0,
        relay_decode_failures: 0,
        relay_config_hash_mismatches: 0,
        relay_error_responses: 0,
        last_relay_error: None,
        relay_send_failures: 0,
        relay_receive_failures: 0,
        unroutable_tun_packets: 0,
        last_unroutable_destination: None,
        oversized_tun_packets: 0,
        last_oversized_tun_packet_size: None,
        wintun_write_failures: 0,
        started_at_ms: updated_at_ms,
        last_tun_packet_at_ms: None,
        last_relay_packet_at_ms: None,
        last_relay_keepalive_at_ms: None,
        updated_at_ms,
    }
}
