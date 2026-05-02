use std::{
    net::{SocketAddr, UdpSocket},
    sync::{
        atomic::{AtomicUsize, Ordering},
        Arc,
    },
    thread,
    time::Duration,
};

use crate::config::DaemonConfig;
use crate::mqtt::{current_timestamp_ms, publish_relay_heartbeat, RelayHeartbeatPayload};
use crate::protocol::{ClientRequest, ErrorResponse, ServerResponse};
use crate::runtime::RelayRuntime;

pub struct RelayDaemon {
    socket: UdpSocket,
    runtime: RelayRuntime,
    active_sessions: Arc<AtomicUsize>,
}

impl RelayDaemon {
    pub fn bind(config: DaemonConfig) -> Result<Self, String> {
        let socket = UdpSocket::bind(&config.udp_bind)
            .map_err(|err| format!("bind relay udp socket {}: {err}", config.udp_bind))?;
        let active_sessions = Arc::new(AtomicUsize::new(0));
        start_mqtt_heartbeat(
            config.clone(),
            active_sessions.clone(),
            socket.local_addr().ok(),
        );
        Ok(Self {
            socket,
            runtime: RelayRuntime::new(config.relay_url_prefix, config.ticket_signing_secret),
            active_sessions,
        })
    }

    pub fn local_addr(&self) -> Result<SocketAddr, String> {
        self.socket
            .local_addr()
            .map_err(|err| format!("read relay local addr: {err}"))
    }

    pub fn serve(&mut self) -> Result<(), String> {
        let mut buf = [0_u8; 64 * 1024];
        loop {
            let (len, source) = self
                .socket
                .recv_from(&mut buf)
                .map_err(|err| format!("recv relay udp packet: {err}"))?;
            let request = serde_json::from_slice::<ClientRequest>(&buf[..len])
                .map_err(|err| format!("decode relay request: {err}"))?;
            match self.runtime.handle_request(source, request) {
                Ok((response, maybe_peer_packet)) => {
                    self.active_sessions
                        .store(self.runtime.active_session_count(), Ordering::Relaxed);
                    if let Some((peer_addr, packet)) = maybe_peer_packet {
                        self.send_response(peer_addr, &packet)?;
                    }
                    self.send_response(source, &response)?;
                }
                Err(error) => {
                    self.send_response(
                        source,
                        &ServerResponse::Error(ErrorResponse {
                            code: error.code.to_string(),
                            message: error.message,
                        }),
                    )?;
                }
            }
        }
    }

    fn send_response(&self, target: SocketAddr, response: &ServerResponse) -> Result<(), String> {
        let payload =
            serde_json::to_vec(response).map_err(|err| format!("encode relay response: {err}"))?;
        self.socket
            .send_to(&payload, target)
            .map_err(|err| format!("send relay udp response: {err}"))?;
        Ok(())
    }
}

fn start_mqtt_heartbeat(
    config: DaemonConfig,
    active_sessions: Arc<AtomicUsize>,
    local_addr: Option<SocketAddr>,
) {
    let Some(mqtt) = config.mqtt.clone().filter(|value| value.enabled) else {
        return;
    };
    thread::spawn(move || {
        let interval = Duration::from_secs(mqtt.interval_seconds.unwrap_or(30).max(5));
        loop {
            let address = mqtt
                .address
                .clone()
                .or_else(|| local_addr.map(|value| value.to_string()))
                .unwrap_or_default();
            let payload = RelayHeartbeatPayload {
                node_id: mqtt.node_id.clone(),
                cluster_id: mqtt.cluster_id.clone(),
                country_code: mqtt.country_code.clone(),
                city_code: mqtt.city_code.clone(),
                transport: mqtt.transport.clone().unwrap_or_else(|| "udp".to_string()),
                address,
                healthy: true,
                active_sessions: active_sessions.load(Ordering::Relaxed),
                reported_at_ms: current_timestamp_ms(),
            };
            if let Err(err) = publish_relay_heartbeat(&mqtt, &payload) {
                eprintln!("server-relay mqtt heartbeat failed: {err}");
            }
            thread::sleep(interval);
        }
    });
}
