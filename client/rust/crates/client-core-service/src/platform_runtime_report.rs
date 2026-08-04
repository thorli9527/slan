use client_core::{ClientViewState, TrafficStatsPayload};
use serde_json::{Map, Value};
use std::sync::{Mutex, OnceLock};

use crate::{
    control_plane::ControlPlaneClient,
    session_store::{
        current_timestamp_ms, load_session, lock_session_runtime_epoch, session_device_api_token,
    },
};

pub(crate) fn report_platform_runtime(
    session_epoch: u64,
    state: ClientViewState,
    platform: Option<String>,
    runtime_state: Value,
    traffic: Option<Value>,
    reported_at_ms: Option<u64>,
) {
    let Ok(_session_guard) = lock_session_runtime_epoch(session_epoch) else {
        return;
    };
    let Ok(session) = load_session() else {
        return;
    };
    let Some(device_id) = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
    else {
        return;
    };
    let body = runtime_report_body(
        &device_id,
        session.active_network_id.as_deref(),
        &state,
        platform.as_deref(),
        &runtime_state,
        traffic.as_ref(),
        reported_at_ms,
    );
    let client = ControlPlaneClient::from_env();
    if let Err(error) =
        client.report_device_runtime(session_device_api_token(&session), &device_id, body)
    {
        eprintln!("SLAN_PLATFORM_RUNTIME_REPORT_FAILED deviceId={device_id} error={error:#}");
    }
}

pub(crate) fn platform_traffic_stats_payload(
    session_epoch: u64,
    traffic: Option<&Value>,
    reported_at_ms: Option<u64>,
) -> Option<TrafficStatsPayload> {
    let traffic = traffic?;
    let updated_at_ms = reported_at_ms
        .or_else(|| value_u64(traffic, &["updatedAtMs", "reportedAtMs"]))
        .unwrap_or_else(current_timestamp_ms);
    let tx_bytes = value_u64(
        traffic,
        &["txBytes", "outboundBytes", "sentBytes", "bytesRead"],
    )
    .unwrap_or_default();
    let rx_bytes = value_u64(
        traffic,
        &[
            "rxBytes",
            "inboundBytes",
            "receivedBytes",
            "bytesWritten",
            "tunBytesWritten",
        ],
    )
    .unwrap_or_default();
    let previous = {
        let mut guard = last_traffic_sample()
            .lock()
            .expect("platform traffic sample mutex poisoned");
        let previous = *guard;
        *guard = Some(TrafficSample {
            session_epoch,
            tx_bytes,
            rx_bytes,
            updated_at_ms,
        });
        previous
    };
    let (tx_bytes_per_minute, rx_bytes_per_minute) =
        traffic_rates(previous, session_epoch, tx_bytes, rx_bytes, updated_at_ms);
    Some(TrafficStatsPayload {
        tx_bytes,
        rx_bytes,
        tx_bytes_per_minute,
        rx_bytes_per_minute,
        updated_at_ms,
    })
}

#[derive(Debug, Clone, Copy)]
struct TrafficSample {
    session_epoch: u64,
    tx_bytes: u64,
    rx_bytes: u64,
    updated_at_ms: u64,
}

fn last_traffic_sample() -> &'static Mutex<Option<TrafficSample>> {
    static LAST_TRAFFIC_SAMPLE: OnceLock<Mutex<Option<TrafficSample>>> = OnceLock::new();
    LAST_TRAFFIC_SAMPLE.get_or_init(|| Mutex::new(None))
}

fn traffic_rates(
    previous: Option<TrafficSample>,
    session_epoch: u64,
    tx_bytes: u64,
    rx_bytes: u64,
    updated_at_ms: u64,
) -> (u64, u64) {
    previous
        .filter(|sample| {
            sample.session_epoch == session_epoch && updated_at_ms > sample.updated_at_ms
        })
        .map(|sample| {
            let elapsed_ms = updated_at_ms.saturating_sub(sample.updated_at_ms).max(1);
            (
                bytes_per_minute(tx_bytes.saturating_sub(sample.tx_bytes), elapsed_ms),
                bytes_per_minute(rx_bytes.saturating_sub(sample.rx_bytes), elapsed_ms),
            )
        })
        .unwrap_or((0, 0))
}

fn bytes_per_minute(delta_bytes: u64, elapsed_ms: u64) -> u64 {
    ((delta_bytes as u128).saturating_mul(60_000) / elapsed_ms as u128) as u64
}

fn runtime_report_body(
    device_id: &str,
    session_network_id: Option<&str>,
    state: &ClientViewState,
    platform: Option<&str>,
    runtime_state: &Value,
    traffic: Option<&Value>,
    reported_at_ms: Option<u64>,
) -> Value {
    let runtime_path = runtime_state.get("runtimePath").unwrap_or(&Value::Null);
    let reported_at_ms = reported_at_ms
        .or_else(|| value_u64(runtime_state, &["reportedAtMs"]))
        .or_else(|| traffic.and_then(|value| value_u64(value, &["updatedAtMs"])))
        .unwrap_or_else(current_timestamp_ms);
    let mut body = Map::new();
    insert_string(&mut body, "deviceId", Some(device_id));
    insert_u64(&mut body, "reportedAtMs", Some(reported_at_ms));
    insert_u64(
        &mut body,
        "lastSeenAt",
        value_u64(runtime_state, &["lastSeenAt"]).or(Some(reported_at_ms / 1_000)),
    );
    insert_string(
        &mut body,
        "status",
        Some(if state.network_enabled {
            "active"
        } else {
            "inactive"
        }),
    );
    insert_u64(
        &mut body,
        "rxBytesTotal",
        value_u64(runtime_state, &["rxBytesTotal"])
            .or_else(|| traffic.and_then(|value| value_u64(value, &["bytesRead"])))
            .or(state.traffic_rx_bytes),
    );
    insert_u64(
        &mut body,
        "txBytesTotal",
        value_u64(runtime_state, &["txBytesTotal"])
            .or_else(|| traffic.and_then(|value| value_u64(value, &["bytesWritten"])))
            .or(state.traffic_tx_bytes),
    );
    insert_string(&mut body, "platform", platform);
    insert_string(
        &mut body,
        "deviceVersion",
        first_string(&[(runtime_state, "deviceVersion")]),
    );
    insert_string(
        &mut body,
        "networkId",
        first_string(&[(runtime_state, "networkId"), (runtime_path, "networkId")])
            .or(session_network_id),
    );
    for (key, candidates) in [
        (
            "natType",
            vec![(runtime_state, "natType"), (runtime_path, "natType")],
        ),
        (
            "activePath",
            vec![
                (runtime_state, "activePath"),
                (runtime_state, "pathType"),
                (runtime_path, "activePath"),
            ],
        ),
        (
            "relayTransport",
            vec![
                (runtime_state, "relayTransport"),
                (runtime_path, "relayTransport"),
            ],
        ),
        (
            "relayEndpoint",
            vec![
                (runtime_state, "relayEndpoint"),
                (runtime_state, "relayAddress"),
                (runtime_state, "endpoint"),
                (runtime_path, "relayEndpoint"),
                (runtime_path, "relayAddress"),
            ],
        ),
        (
            "derpNodeId",
            vec![
                (runtime_state, "derpNodeId"),
                (runtime_state, "relayEndpointId"),
                (runtime_state, "endpointId"),
                (runtime_path, "derpNodeId"),
                (runtime_path, "relayEndpointId"),
                (runtime_path, "endpointId"),
            ],
        ),
        (
            "peerNodeId",
            vec![(runtime_state, "peerNodeId"), (runtime_path, "peerNodeId")],
        ),
        (
            "ticketExpiresAt",
            vec![
                (runtime_state, "ticketExpiresAt"),
                (runtime_path, "ticketExpiresAt"),
            ],
        ),
        (
            "lastPathChange",
            vec![
                (runtime_state, "lastPathChange"),
                (runtime_path, "lastPathChange"),
            ],
        ),
    ] {
        insert_string(&mut body, key, first_string(&candidates));
    }
    for (key, candidates) in [
        (
            "pathObservedAt",
            vec![
                (runtime_state, "pathObservedAt"),
                (runtime_state, "observedAt"),
                (runtime_path, "observedAt"),
                (runtime_path, "pathObservedAt"),
            ],
        ),
        (
            "pathScore",
            vec![(runtime_state, "pathScore"), (runtime_path, "pathScore")],
        ),
        (
            "observedRttMs",
            vec![
                (runtime_state, "observedRttMs"),
                (runtime_state, "rttMs"),
                (runtime_path, "observedRttMs"),
            ],
        ),
        (
            "packetLossPpm",
            vec![
                (runtime_state, "packetLossPpm"),
                (runtime_path, "packetLossPpm"),
            ],
        ),
        (
            "relayMtu",
            vec![(runtime_state, "relayMtu"), (runtime_path, "relayMtu")],
        ),
        (
            "maxFramePayload",
            vec![
                (runtime_state, "maxFramePayload"),
                (runtime_path, "maxFramePayload"),
            ],
        ),
        (
            "pathDowngrades",
            vec![
                (runtime_state, "pathDowngrades"),
                (runtime_path, "pathDowngrades"),
            ],
        ),
        (
            "pathUpgrades",
            vec![
                (runtime_state, "pathUpgrades"),
                (runtime_path, "pathUpgrades"),
            ],
        ),
    ] {
        insert_u64(&mut body, key, first_u64(&candidates));
    }
    if let Some(value) = first_bool(&[
        (runtime_state, "ticketRenewDue"),
        (runtime_path, "ticketRenewDue"),
    ]) {
        body.insert("ticketRenewDue".to_string(), Value::Bool(value));
    }
    Value::Object(body)
}

fn first_string<'a>(candidates: &[(&'a Value, &str)]) -> Option<&'a str> {
    candidates.iter().find_map(|(value, key)| {
        value
            .get(*key)
            .and_then(Value::as_str)
            .map(str::trim)
            .filter(|value| !value.is_empty())
    })
}

fn first_u64(candidates: &[(&Value, &str)]) -> Option<u64> {
    candidates
        .iter()
        .find_map(|(value, key)| value_u64(value, &[*key]))
}

fn first_bool(candidates: &[(&Value, &str)]) -> Option<bool> {
    candidates.iter().find_map(|(value, key)| {
        value.get(*key).and_then(|value| {
            value.as_bool().or_else(|| {
                value
                    .as_str()
                    .and_then(|value| value.trim().parse::<bool>().ok())
            })
        })
    })
}

fn value_u64(value: &Value, keys: &[&str]) -> Option<u64> {
    keys.iter().find_map(|key| {
        value.get(*key).and_then(|value| {
            value
                .as_u64()
                .or_else(|| value.as_i64().and_then(|value| u64::try_from(value).ok()))
                .or_else(|| value.as_str().and_then(|value| value.trim().parse().ok()))
        })
    })
}

fn insert_string(body: &mut Map<String, Value>, key: &str, value: Option<&str>) {
    if let Some(value) = value.map(str::trim).filter(|value| !value.is_empty()) {
        body.insert(key.to_string(), Value::String(value.to_string()));
    }
}

fn insert_u64(body: &mut Map<String, Value>, key: &str, value: Option<u64>) {
    if let Some(value) = value {
        body.insert(key.to_string(), Value::from(value));
    }
}

#[cfg(test)]
mod tests {
    use super::{runtime_report_body, traffic_rates, TrafficSample};
    use client_core::ClientViewState;

    #[test]
    fn runtime_report_preserves_extended_path_fields() {
        let mut state = ClientViewState::default();
        state.network_enabled = true;
        let body = runtime_report_body(
            "device-1",
            Some("network-session"),
            &state,
            Some("android"),
            &serde_json::json!({
                "networkEnabled": true,
                "runtimePath": {
                    "networkId": "network-runtime",
                    "relayTransport": "derp_tcp_tls_443",
                    "pathScore": 91,
                    "ticketRenewDue": true
                }
            }),
            Some(&serde_json::json!({"bytesRead": 12, "bytesWritten": 34})),
            Some(1_780_000_000_000),
        );

        assert_eq!(body["networkId"], "network-runtime");
        assert_eq!(body["relayTransport"], "derp_tcp_tls_443");
        assert_eq!(body["pathScore"], 91);
        assert_eq!(body["ticketRenewDue"], true);
        assert_eq!(body["rxBytesTotal"], 12);
        assert_eq!(body["txBytesTotal"], 34);
    }

    #[test]
    fn traffic_rate_sample_is_scoped_to_session_epoch() {
        let previous = TrafficSample {
            session_epoch: 7,
            tx_bytes: 100,
            rx_bytes: 200,
            updated_at_ms: 1_000,
        };
        let current_epoch = 8;
        let updated_at_ms = 2_000;

        assert_eq!(
            traffic_rates(Some(previous), current_epoch, 300, 500, updated_at_ms),
            (0, 0)
        );
        assert_eq!(
            traffic_rates(Some(previous), 7, 300, 500, updated_at_ms),
            (12_000, 18_000)
        );
    }
}
