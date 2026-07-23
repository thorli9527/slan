use std::{
    net::{Ipv4Addr, Ipv6Addr, SocketAddr, UdpSocket},
    sync::{Mutex, OnceLock},
    time::Duration,
};

use anyhow::{anyhow, Context, Result};

use crate::{
    network_runtime_state::RuntimeNetworkState,
    resolver_authority::{
        resolve_authoritative, route_resolver_query, ResolveAuthoritativeResult,
        ResolverRouteDecision,
    },
    resolver_forwarder::forward_resolver_query,
    resolver_runtime_state::RuntimeResolverState,
    session_store::current_timestamp_ms,
};

const DNS_FLAG_RESPONSE: u16 = 0x8000;
const DNS_FLAG_AUTHORITATIVE: u16 = 0x0400;
const DNS_FLAG_RECURSION_DESIRED: u16 = 0x0100;
const DNS_FLAG_RECURSION_AVAILABLE: u16 = 0x0080;
const DNS_RCODE_NXDOMAIN: u16 = 0x0003;
const DNS_TYPE_A: u16 = 1;
const DNS_TYPE_CNAME: u16 = 5;
const DNS_TYPE_PTR: u16 = 12;
const DNS_TYPE_TXT: u16 = 16;
const DNS_TYPE_AAAA: u16 = 28;
const DNS_TYPE_SRV: u16 = 33;
const DNS_CLASS_IN: u16 = 1;
const DNS_HEADER_SIZE: usize = 12;
const DNS_POINTER_MASK: u8 = 0xC0;
const MAX_DNS_PACKET_SIZE: usize = 4096;
const DNS_SERVER_READ_TIMEOUT: Duration = Duration::from_secs(1);
const DEFAULT_LOCAL_DNS_BIND_ADDR: &str = "127.0.0.1:53";

pub(crate) struct ResolverServer {
    socket: UdpSocket,
}

#[derive(Debug, Clone, Default)]
pub(crate) struct LocalResolverServerStatus {
    pub enabled: bool,
    pub listening: bool,
    pub bind_addr: Option<String>,
    pub requester_device_id: Option<String>,
    pub last_error: Option<String>,
    pub last_query_at_ms: Option<u64>,
    pub desired_enabled: Option<bool>,
    pub desired_signed_in: Option<bool>,
    pub desired_network_enabled: Option<bool>,
    pub desired_has_requester_device_id: Option<bool>,
    pub desired_has_resolver_data: Option<bool>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct ParsedQuestion {
    qname: String,
    qtype: u16,
    qclass: u16,
    question_end: usize,
}

static LOCAL_DNS_SERVER_STATUS: OnceLock<Mutex<LocalResolverServerStatus>> = OnceLock::new();

pub(crate) fn local_resolver_server_status() -> &'static Mutex<LocalResolverServerStatus> {
    LOCAL_DNS_SERVER_STATUS.get_or_init(|| Mutex::new(LocalResolverServerStatus::default()))
}

pub(crate) fn set_local_resolver_server_status(status: LocalResolverServerStatus) {
    let mut current = local_resolver_server_status()
        .lock()
        .expect("local resolver server status mutex poisoned");
    *current = status;
}

pub(crate) fn local_resolver_server_last_query_at_ms() -> Option<u64> {
    local_resolver_server_status()
        .try_lock()
        .ok()
        .and_then(|status| status.last_query_at_ms)
}

pub(crate) fn desired_local_resolver_bind_addr() -> String {
    DEFAULT_LOCAL_DNS_BIND_ADDR.to_string()
}

impl ResolverServer {
    pub(crate) fn bind(bind_addr: &str) -> Result<Self> {
        let socket = UdpSocket::bind(bind_addr)
            .with_context(|| format!("bind local resolver udp socket {bind_addr}"))?;
        socket
            .set_read_timeout(Some(DNS_SERVER_READ_TIMEOUT))
            .context("set local resolver udp read timeout")?;
        Ok(Self { socket })
    }

    pub(crate) fn local_addr(&self) -> Result<SocketAddr> {
        self.socket
            .local_addr()
            .context("read local resolver udp socket addr")
    }

    pub(crate) fn serve_once(
        &self,
        runtime: &RuntimeNetworkState,
        dns: &mut RuntimeResolverState,
        requester_device_id: &str,
    ) -> Result<bool> {
        let mut buffer = [0_u8; MAX_DNS_PACKET_SIZE];
        let (size, peer) = match self.socket.recv_from(&mut buffer) {
            Ok(value) => value,
            Err(error)
                if matches!(
                    error.kind(),
                    std::io::ErrorKind::WouldBlock | std::io::ErrorKind::TimedOut
                ) =>
            {
                return Ok(false);
            }
            Err(error) => return Err(error).context("receive dns query"),
        };
        let response =
            self.handle_query_packet(runtime, dns, requester_device_id, &buffer[..size])?;
        self.socket
            .send_to(&response, peer)
            .with_context(|| format!("send dns response to {peer}"))?;
        let mut status = local_resolver_server_status()
            .lock()
            .expect("local resolver server status mutex poisoned");
        status.last_query_at_ms = Some(current_timestamp_ms());
        status.last_error = None;
        Ok(true)
    }

    pub(crate) fn handle_query_packet(
        &self,
        runtime: &RuntimeNetworkState,
        dns: &mut RuntimeResolverState,
        requester_device_id: &str,
        raw_query: &[u8],
    ) -> Result<Vec<u8>> {
        let question = parse_question(raw_query)?;
        match route_resolver_query(dns, &question.qname) {
            ResolverRouteDecision::Authoritative => {
                let qtype_name = dns_type_name(question.qtype);
                let result = resolve_authoritative(
                    runtime,
                    dns,
                    requester_device_id,
                    &question.qname,
                    qtype_name,
                );
                match result {
                    ResolveAuthoritativeResult::AnswerA { ttl, ips } => {
                        build_a_response(raw_query, &question, ttl, &ips)
                    }
                    ResolveAuthoritativeResult::AnswerAaaa { ttl, ips } => {
                        build_aaaa_response(raw_query, &question, ttl, &ips)
                    }
                    ResolveAuthoritativeResult::AnswerCname { ttl, cname } => {
                        build_cname_response(raw_query, &question, ttl, &cname)
                    }
                    ResolveAuthoritativeResult::AnswerPtr { ttl, name } => {
                        build_ptr_response(raw_query, &question, ttl, &name)
                    }
                    ResolveAuthoritativeResult::AnswerTxt { ttl, texts } => {
                        build_txt_response(raw_query, &question, ttl, &texts)
                    }
                    ResolveAuthoritativeResult::AnswerSrv { ttl, port, target } => {
                        build_srv_response(raw_query, &question, ttl, port, &target)
                    }
                    ResolveAuthoritativeResult::NxDomain => {
                        build_error_response(raw_query, &question, DNS_RCODE_NXDOMAIN)
                    }
                    ResolveAuthoritativeResult::NoData | ResolveAuthoritativeResult::NotManaged => {
                        build_empty_response(raw_query, &question)
                    }
                }
            }
            ResolverRouteDecision::ForwardUpstream => forward_resolver_query(dns, raw_query),
            ResolverRouteDecision::NoData => build_empty_response(raw_query, &question),
        }
    }
}

fn parse_question(raw_query: &[u8]) -> Result<ParsedQuestion> {
    if raw_query.len() < DNS_HEADER_SIZE {
        return Err(anyhow!("dns packet too short"));
    }
    let qdcount = u16::from_be_bytes([raw_query[4], raw_query[5]]);
    if qdcount != 1 {
        return Err(anyhow!("dns packet must contain exactly one question"));
    }
    let mut offset = DNS_HEADER_SIZE;
    let qname = parse_name(raw_query, &mut offset)?;
    if raw_query.len() < offset + 4 {
        return Err(anyhow!("dns packet missing qtype/qclass"));
    }
    let qtype = u16::from_be_bytes([raw_query[offset], raw_query[offset + 1]]);
    let qclass = u16::from_be_bytes([raw_query[offset + 2], raw_query[offset + 3]]);
    Ok(ParsedQuestion {
        qname,
        qtype,
        qclass,
        question_end: offset + 4,
    })
}

fn parse_name(packet: &[u8], offset: &mut usize) -> Result<String> {
    let mut labels = Vec::new();
    let mut cursor = *offset;
    let mut jumped = false;
    let mut jump_guard = 0_usize;
    loop {
        if cursor >= packet.len() {
            return Err(anyhow!("dns name out of bounds"));
        }
        let len = packet[cursor];
        if len & DNS_POINTER_MASK == DNS_POINTER_MASK {
            if cursor + 1 >= packet.len() {
                return Err(anyhow!("dns compressed name pointer truncated"));
            }
            let pointer = (((len & !DNS_POINTER_MASK) as usize) << 8) | packet[cursor + 1] as usize;
            if !jumped {
                *offset = cursor + 2;
                jumped = true;
            }
            cursor = pointer;
            jump_guard += 1;
            if jump_guard > packet.len() {
                return Err(anyhow!("dns compressed name loop"));
            }
            continue;
        }
        cursor += 1;
        if len == 0 {
            if !jumped {
                *offset = cursor;
            }
            break;
        }
        let label_len = len as usize;
        if cursor + label_len > packet.len() {
            return Err(anyhow!("dns label out of bounds"));
        }
        let label =
            std::str::from_utf8(&packet[cursor..cursor + label_len]).context("decode dns label")?;
        labels.push(label.to_ascii_lowercase());
        cursor += label_len;
    }
    Ok(labels.join("."))
}

fn dns_type_name(qtype: u16) -> &'static str {
    match qtype {
        DNS_TYPE_A => "A",
        DNS_TYPE_CNAME => "CNAME",
        DNS_TYPE_PTR => "PTR",
        DNS_TYPE_TXT => "TXT",
        DNS_TYPE_AAAA => "AAAA",
        DNS_TYPE_SRV => "SRV",
        _ => "UNKNOWN",
    }
}

fn build_a_response(
    raw_query: &[u8],
    question: &ParsedQuestion,
    ttl: u32,
    ips: &[String],
) -> Result<Vec<u8>> {
    if question.qclass != DNS_CLASS_IN {
        return build_empty_response(raw_query, question);
    }
    let parsed = ips
        .iter()
        .map(|ip| {
            ip.trim()
                .parse::<Ipv4Addr>()
                .with_context(|| format!("parse dns A record ip {ip}"))
        })
        .collect::<Result<Vec<_>>>()?;
    let mut response = build_response_prefix(raw_query, question, parsed.len() as u16, 0)?;
    for ip in parsed {
        push_name_pointer(&mut response);
        response.extend_from_slice(&DNS_TYPE_A.to_be_bytes());
        response.extend_from_slice(&DNS_CLASS_IN.to_be_bytes());
        response.extend_from_slice(&ttl.to_be_bytes());
        response.extend_from_slice(&(4_u16).to_be_bytes());
        response.extend_from_slice(&ip.octets());
    }
    Ok(response)
}

fn build_cname_response(
    raw_query: &[u8],
    question: &ParsedQuestion,
    ttl: u32,
    cname: &str,
) -> Result<Vec<u8>> {
    if question.qclass != DNS_CLASS_IN {
        return build_empty_response(raw_query, question);
    }
    let mut encoded_cname = Vec::new();
    encode_name(cname, &mut encoded_cname)?;
    let mut response = build_response_prefix(raw_query, question, 1, 0)?;
    push_name_pointer(&mut response);
    response.extend_from_slice(&DNS_TYPE_CNAME.to_be_bytes());
    response.extend_from_slice(&DNS_CLASS_IN.to_be_bytes());
    response.extend_from_slice(&ttl.to_be_bytes());
    response.extend_from_slice(&(encoded_cname.len() as u16).to_be_bytes());
    response.extend_from_slice(&encoded_cname);
    Ok(response)
}

fn build_ptr_response(
    raw_query: &[u8],
    question: &ParsedQuestion,
    ttl: u32,
    name: &str,
) -> Result<Vec<u8>> {
    if question.qclass != DNS_CLASS_IN {
        return build_empty_response(raw_query, question);
    }
    let mut encoded_name = Vec::new();
    encode_name(name, &mut encoded_name)?;
    let mut response = build_response_prefix(raw_query, question, 1, 0)?;
    push_name_pointer(&mut response);
    response.extend_from_slice(&DNS_TYPE_PTR.to_be_bytes());
    response.extend_from_slice(&DNS_CLASS_IN.to_be_bytes());
    response.extend_from_slice(&ttl.to_be_bytes());
    response.extend_from_slice(&(encoded_name.len() as u16).to_be_bytes());
    response.extend_from_slice(&encoded_name);
    Ok(response)
}

fn build_txt_response(
    raw_query: &[u8],
    question: &ParsedQuestion,
    ttl: u32,
    texts: &[String],
) -> Result<Vec<u8>> {
    if question.qclass != DNS_CLASS_IN {
        return build_empty_response(raw_query, question);
    }
    let encoded = texts
        .iter()
        .map(|text| {
            let text_bytes = text.as_bytes();
            if text_bytes.len() > 255 {
                return Err(anyhow!("dns TXT record exceeds 255 bytes"));
            }
            Ok(text_bytes.to_vec())
        })
        .collect::<Result<Vec<_>>>()?;
    let mut response = build_response_prefix(raw_query, question, encoded.len() as u16, 0)?;
    for text_bytes in encoded {
        push_name_pointer(&mut response);
        response.extend_from_slice(&DNS_TYPE_TXT.to_be_bytes());
        response.extend_from_slice(&DNS_CLASS_IN.to_be_bytes());
        response.extend_from_slice(&ttl.to_be_bytes());
        response.extend_from_slice(&((text_bytes.len() + 1) as u16).to_be_bytes());
        response.push(text_bytes.len() as u8);
        response.extend_from_slice(&text_bytes);
    }
    Ok(response)
}

fn build_aaaa_response(
    raw_query: &[u8],
    question: &ParsedQuestion,
    ttl: u32,
    ips: &[String],
) -> Result<Vec<u8>> {
    if question.qclass != DNS_CLASS_IN {
        return build_empty_response(raw_query, question);
    }
    let parsed = ips
        .iter()
        .map(|ip| {
            ip.trim()
                .parse::<Ipv6Addr>()
                .with_context(|| format!("parse dns AAAA record ip {ip}"))
        })
        .collect::<Result<Vec<_>>>()?;
    let mut response = build_response_prefix(raw_query, question, parsed.len() as u16, 0)?;
    for ip in parsed {
        push_name_pointer(&mut response);
        response.extend_from_slice(&DNS_TYPE_AAAA.to_be_bytes());
        response.extend_from_slice(&DNS_CLASS_IN.to_be_bytes());
        response.extend_from_slice(&ttl.to_be_bytes());
        response.extend_from_slice(&(16_u16).to_be_bytes());
        response.extend_from_slice(&ip.octets());
    }
    Ok(response)
}

fn build_srv_response(
    raw_query: &[u8],
    question: &ParsedQuestion,
    ttl: u32,
    port: u16,
    target: &str,
) -> Result<Vec<u8>> {
    if question.qclass != DNS_CLASS_IN {
        return build_empty_response(raw_query, question);
    }
    let mut encoded_target = Vec::new();
    encode_name(target, &mut encoded_target)?;
    let mut response = build_response_prefix(raw_query, question, 1, 0)?;
    push_name_pointer(&mut response);
    response.extend_from_slice(&DNS_TYPE_SRV.to_be_bytes());
    response.extend_from_slice(&DNS_CLASS_IN.to_be_bytes());
    response.extend_from_slice(&ttl.to_be_bytes());
    response.extend_from_slice(&((6 + encoded_target.len()) as u16).to_be_bytes());
    response.extend_from_slice(&0_u16.to_be_bytes());
    response.extend_from_slice(&0_u16.to_be_bytes());
    response.extend_from_slice(&port.to_be_bytes());
    response.extend_from_slice(&encoded_target);
    Ok(response)
}

fn build_empty_response(raw_query: &[u8], question: &ParsedQuestion) -> Result<Vec<u8>> {
    build_response_prefix(raw_query, question, 0, 0)
}

fn build_error_response(
    raw_query: &[u8],
    question: &ParsedQuestion,
    rcode: u16,
) -> Result<Vec<u8>> {
    build_response_prefix(raw_query, question, 0, rcode)
}

fn build_response_prefix(
    raw_query: &[u8],
    question: &ParsedQuestion,
    answer_count: u16,
    rcode: u16,
) -> Result<Vec<u8>> {
    if raw_query.len() < question.question_end {
        return Err(anyhow!("dns question out of bounds for response"));
    }
    let request_flags = u16::from_be_bytes([raw_query[2], raw_query[3]]);
    let recursion_desired = request_flags & DNS_FLAG_RECURSION_DESIRED;
    let mut response = Vec::with_capacity(raw_query.len() + 64);
    response.extend_from_slice(&raw_query[0..2]);
    let flags = DNS_FLAG_RESPONSE
        | DNS_FLAG_AUTHORITATIVE
        | DNS_FLAG_RECURSION_AVAILABLE
        | recursion_desired
        | rcode;
    response.extend_from_slice(&flags.to_be_bytes());
    response.extend_from_slice(&(1_u16).to_be_bytes());
    response.extend_from_slice(&answer_count.to_be_bytes());
    response.extend_from_slice(&(0_u16).to_be_bytes());
    response.extend_from_slice(&(0_u16).to_be_bytes());
    response.extend_from_slice(&raw_query[DNS_HEADER_SIZE..question.question_end]);
    Ok(response)
}

fn push_name_pointer(buffer: &mut Vec<u8>) {
    buffer.push(DNS_POINTER_MASK);
    buffer.push(DNS_HEADER_SIZE as u8);
}

fn encode_name(name: &str, output: &mut Vec<u8>) -> Result<()> {
    let trimmed = name.trim().trim_end_matches('.');
    if trimmed.is_empty() {
        output.push(0);
        return Ok(());
    }
    for label in trimmed.split('.') {
        if label.is_empty() || label.len() > 63 {
            return Err(anyhow!("invalid dns label length"));
        }
        output.push(label.len() as u8);
        output.extend_from_slice(label.as_bytes());
    }
    output.push(0);
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::{
        build_cname_response, build_error_response, parse_question, DNS_RCODE_NXDOMAIN, DNS_TYPE_A,
        DNS_TYPE_AAAA, DNS_TYPE_CNAME, DNS_TYPE_PTR, DNS_TYPE_SRV, DNS_TYPE_TXT,
    };

    #[test]
    fn parses_single_question() {
        let packet = build_question_packet("home.slan.test", DNS_TYPE_A);
        let question = parse_question(&packet).expect("parse dns question");
        assert_eq!(question.qname, "home.slan.test");
        assert_eq!(question.qtype, DNS_TYPE_A);
    }

    #[test]
    fn builds_a_like_dns_response() {
        let packet = build_question_packet("home.slan.test", DNS_TYPE_A);
        let question = parse_question(&packet).expect("parse dns question");
        let response = super::build_a_response(&packet, &question, 30, &["10.0.0.2".to_string()])
            .expect("build response");
        assert_eq!(u16::from_be_bytes([response[6], response[7]]), 1);
        assert_eq!(parse_response_answer_ipv4(&response), "10.0.0.2");
    }

    #[test]
    fn builds_cname_response() {
        let packet = build_question_packet("db.slan.test", DNS_TYPE_CNAME);
        let question = parse_question(&packet).expect("parse dns question");
        let response = build_cname_response(&packet, &question, 60, "node.slan.test")
            .expect("build cname response");
        assert_eq!(u16::from_be_bytes([response[6], response[7]]), 1);
        assert_eq!(parse_response_cname(&response), "node.slan.test");
    }

    #[test]
    fn builds_aaaa_response() {
        let packet = build_question_packet("v6.slan.test", DNS_TYPE_AAAA);
        let question = parse_question(&packet).expect("parse dns question");
        let response =
            super::build_aaaa_response(&packet, &question, 60, &["2001:db8::20".to_string()])
                .expect("build aaaa response");
        assert_eq!(u16::from_be_bytes([response[6], response[7]]), 1);
        assert_eq!(parse_response_answer_ipv6(&response), "2001:db8::20");
    }

    #[test]
    fn builds_ptr_response() {
        let packet = build_question_packet("4.3.2.1.in-addr.arpa", DNS_TYPE_PTR);
        let question = parse_question(&packet).expect("parse dns question");
        let response = super::build_ptr_response(&packet, &question, 60, "peer.slan.test")
            .expect("build ptr response");
        assert_eq!(u16::from_be_bytes([response[6], response[7]]), 1);
        assert_eq!(parse_response_cname(&response), "peer.slan.test");
    }

    #[test]
    fn builds_txt_response() {
        let packet = build_question_packet("txt.slan.test", DNS_TYPE_TXT);
        let question = parse_question(&packet).expect("parse dns question");
        let response =
            super::build_txt_response(&packet, &question, 60, &["hello-slan".to_string()])
                .expect("build txt");
        assert_eq!(u16::from_be_bytes([response[6], response[7]]), 1);
        assert_eq!(parse_response_txt(&response), "hello-slan");
    }

    #[test]
    fn builds_srv_response() {
        let packet = build_question_packet("_sip._tcp.slan.test", DNS_TYPE_SRV);
        let question = parse_question(&packet).expect("parse dns question");
        let response = super::build_srv_response(&packet, &question, 60, 5060, "peer.slan.test")
            .expect("build srv response");
        assert_eq!(u16::from_be_bytes([response[6], response[7]]), 1);
        let (port, target) = parse_response_srv(&response);
        assert_eq!(port, 5060);
        assert_eq!(target, "peer.slan.test");
    }

    #[test]
    fn builds_nxdomain_response() {
        let packet = build_question_packet("missing.slan.test", DNS_TYPE_A);
        let question = parse_question(&packet).expect("parse dns question");
        let response = build_error_response(&packet, &question, DNS_RCODE_NXDOMAIN)
            .expect("build nxdomain response");
        assert_eq!(response[3] & 0x0f, DNS_RCODE_NXDOMAIN as u8);
    }

    fn build_question_packet(qname: &str, qtype: u16) -> Vec<u8> {
        let mut packet = vec![
            0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        ];
        super::encode_name(qname, &mut packet).expect("encode qname");
        packet.extend_from_slice(&qtype.to_be_bytes());
        packet.extend_from_slice(&super::DNS_CLASS_IN.to_be_bytes());
        packet
    }

    fn parse_response_answer_ipv4(packet: &[u8]) -> String {
        let rdlength_offset = packet.len() - 6;
        let length = u16::from_be_bytes([packet[rdlength_offset], packet[rdlength_offset + 1]]);
        assert_eq!(length, 4);
        let octets = &packet[packet.len() - 4..];
        format!("{}.{}.{}.{}", octets[0], octets[1], octets[2], octets[3])
    }

    fn parse_response_cname(packet: &[u8]) -> String {
        let question = super::parse_question(packet).expect("parse question from response");
        let mut cursor = question.question_end + 2 + 2 + 2 + 4;
        let rdlength = u16::from_be_bytes([packet[cursor], packet[cursor + 1]]) as usize;
        cursor += 2;
        let end = cursor + rdlength;
        assert!(end <= packet.len());
        super::parse_name(packet, &mut cursor).expect("parse cname")
    }

    fn parse_response_answer_ipv6(packet: &[u8]) -> String {
        let (rdlength_offset, rdata_offset) = answer_offsets(packet);
        let length = u16::from_be_bytes([packet[rdlength_offset], packet[rdlength_offset + 1]]);
        assert_eq!(length, 16);
        let octets: [u8; 16] = packet[rdata_offset..rdata_offset + 16]
            .try_into()
            .expect("ipv6 response octets");
        std::net::Ipv6Addr::from(octets).to_string()
    }

    fn parse_response_txt(packet: &[u8]) -> String {
        let (_, rdata_offset) = answer_offsets(packet);
        let txt_len = usize::from(packet[rdata_offset]);
        let start = rdata_offset + 1;
        let end = start + txt_len;
        String::from_utf8(packet[start..end].to_vec()).expect("decode txt")
    }

    fn parse_response_srv(packet: &[u8]) -> (u16, String) {
        let (_, data_offset) = answer_offsets(packet);
        let port = u16::from_be_bytes([packet[data_offset + 4], packet[data_offset + 5]]);
        let mut cursor = data_offset + 6;
        let target = super::parse_name(packet, &mut cursor).expect("parse srv target");
        (port, target)
    }

    fn answer_offsets(packet: &[u8]) -> (usize, usize) {
        let question = parse_question(packet).expect("parse dns response question");
        let rdlength_offset = question.question_end + 10;
        let rdata_offset = question.question_end + 12;
        (rdlength_offset, rdata_offset)
    }
}
