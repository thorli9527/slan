use std::{
    io,
    net::{SocketAddr, ToSocketAddrs, UdpSocket},
    time::Duration,
};

use anyhow::{anyhow, Context, Result};

use crate::dns_runtime_state::RuntimeDnsState;

const DEFAULT_DNS_PORT: u16 = 53;
const FORWARD_TIMEOUT: Duration = Duration::from_secs(2);
const MAX_DNS_PACKET_SIZE: usize = 4096;

pub(crate) fn forward_dns_query(dns: &RuntimeDnsState, raw_query: &[u8]) -> Result<Vec<u8>> {
    let upstream = dns
        .upstream_servers
        .iter()
        .find_map(|value| parse_upstream_socket_addr(value).ok())
        .ok_or_else(|| anyhow!("no upstream dns server configured"))?;
    forward_dns_query_to_addr(upstream, raw_query)
}

pub(crate) fn forward_dns_query_to_addr(upstream: SocketAddr, raw_query: &[u8]) -> Result<Vec<u8>> {
    let socket = UdpSocket::bind("0.0.0.0:0").context("bind temporary dns forward udp socket")?;
    socket
        .set_read_timeout(Some(FORWARD_TIMEOUT))
        .context("set dns forward read timeout")?;
    socket
        .set_write_timeout(Some(FORWARD_TIMEOUT))
        .context("set dns forward write timeout")?;
    socket
        .send_to(raw_query, upstream)
        .with_context(|| format!("send dns query to upstream {upstream}"))?;

    let mut buffer = vec![0_u8; MAX_DNS_PACKET_SIZE];
    let (size, source) = socket
        .recv_from(&mut buffer)
        .with_context(|| format!("receive dns response from upstream {upstream}"))?;
    if source.ip() != upstream.ip() {
        return Err(anyhow!(
            "dns response source mismatch: expected {} got {}",
            upstream.ip(),
            source.ip()
        ));
    }
    buffer.truncate(size);
    Ok(buffer)
}

pub(crate) fn parse_upstream_socket_addr(value: &str) -> io::Result<SocketAddr> {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        return Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            "empty upstream dns server",
        ));
    }
    let candidate = if trimmed.contains(':') {
        trimmed.to_string()
    } else {
        format!("{trimmed}:{DEFAULT_DNS_PORT}")
    };
    candidate
        .to_socket_addrs()?
        .next()
        .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "resolve upstream dns server"))
}

#[cfg(test)]
mod tests {
    use super::parse_upstream_socket_addr;

    #[test]
    fn parses_host_without_explicit_port() {
        let addr = parse_upstream_socket_addr("127.0.0.1").expect("parse localhost");
        assert_eq!(addr.port(), 53);
    }
}
