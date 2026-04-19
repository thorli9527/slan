use std::net::{SocketAddr, UdpSocket};

use crate::config::DaemonConfig;
use crate::protocol::{ClientRequest, ErrorResponse, ServerResponse};
use crate::runtime::RelayRuntime;

pub struct RelayDaemon {
    socket: UdpSocket,
    runtime: RelayRuntime,
}

impl RelayDaemon {
    pub fn bind(config: DaemonConfig) -> Result<Self, String> {
        let socket = UdpSocket::bind(&config.udp_bind)
            .map_err(|err| format!("bind relay udp socket {}: {err}", config.udp_bind))?;
        Ok(Self {
            socket,
            runtime: RelayRuntime::new(config.relay_url_prefix, config.ticket_signing_secret),
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
