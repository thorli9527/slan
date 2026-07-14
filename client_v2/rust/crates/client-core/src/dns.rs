use std::net::Ipv4Addr;

use crate::platform::PlatformResolverRecord;

const DNS_PORT: u16 = 53;
const DNS_TYPE_A: u16 = 1;
const DNS_TYPE_CNAME: u16 = 5;
const DNS_CLASS_IN: u16 = 1;

pub fn resolver_response_for_query(
    packet: &[u8],
    local_resolver_ip: &str,
    resolver_records: &[PlatformResolverRecord],
) -> Option<Vec<u8>> {
    let query = ResolverIpv4Query::parse(packet)?;
    let reply_ip = crate::normalize_virtual_ip(local_resolver_ip);
    if query.destination_ip != reply_ip {
        return None;
    }
    if query.destination_port != DNS_PORT {
        return None;
    }
    let resolved = resolve_resolver_answers(&query.qname, query.qtype, resolver_records)?;
    build_resolver_ipv4_response(packet, &query, &resolved, &reply_ip)
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct ResolverIpv4Query {
    destination_ip: String,
    destination_port: u16,
    source_port: u16,
    ip_header_len: usize,
    udp_offset: usize,
    dns_offset: usize,
    dns_len: usize,
    dns_id: [u8; 2],
    qname: String,
    qtype: u16,
    question_end: usize,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct ResolverAnswer {
    rr_type: u16,
    ttl: u32,
    rdata: Vec<u8>,
}

impl ResolverIpv4Query {
    fn parse(packet: &[u8]) -> Option<Self> {
        if packet.len() < 20 || packet[0] >> 4 != 4 {
            return None;
        }
        if packet.get(9).copied()? != 17 {
            return None;
        }
        let ihl = usize::from(packet[0] & 0x0f) * 4;
        if ihl < 20 || packet.len() < ihl + 8 {
            return None;
        }
        let total_len = usize::from(u16::from_be_bytes([packet[2], packet[3]]));
        if total_len < ihl + 8 || total_len > packet.len() {
            return None;
        }
        let flags_fragment = u16::from_be_bytes([packet[6], packet[7]]);
        if flags_fragment & 0x1fff != 0 {
            return None;
        }
        let udp_offset = ihl;
        let udp_len = usize::from(u16::from_be_bytes([
            packet[udp_offset + 4],
            packet[udp_offset + 5],
        ]));
        if udp_len < 8 || ihl + udp_len > total_len {
            return None;
        }
        let dns_offset = udp_offset + 8;
        let dns_len = udp_len - 8;
        if dns_len < 12 {
            return None;
        }
        let dns = &packet[dns_offset..dns_offset + dns_len];
        let qdcount = u16::from_be_bytes([dns[4], dns[5]]);
        if qdcount != 1 {
            return None;
        }
        if dns[2] & 0x80 != 0 {
            return None;
        }
        let (qname, question_end) = parse_qname(dns, 12)?;
        if question_end + 4 > dns.len() {
            return None;
        }
        let qtype = u16::from_be_bytes([dns[question_end], dns[question_end + 1]]);
        let qclass = u16::from_be_bytes([dns[question_end + 2], dns[question_end + 3]]);
        if qclass != DNS_CLASS_IN {
            return None;
        }
        Some(Self {
            destination_ip: crate::ipv4_destination(packet)?,
            destination_port: u16::from_be_bytes([packet[udp_offset + 2], packet[udp_offset + 3]]),
            source_port: u16::from_be_bytes([packet[udp_offset], packet[udp_offset + 1]]),
            ip_header_len: ihl,
            udp_offset,
            dns_offset,
            dns_len,
            dns_id: [dns[0], dns[1]],
            qname,
            qtype,
            question_end: question_end + 4,
        })
    }
}

fn resolve_resolver_answers(
    qname: &str,
    qtype: u16,
    resolver_records: &[PlatformResolverRecord],
) -> Option<Vec<ResolverAnswer>> {
    let normalized = normalize_name(qname)?;
    let mut answers = Vec::new();
    for record in resolver_records {
        let record_name = normalize_name(record.fqdn.as_deref().unwrap_or(record.name.as_str()))?;
        if record_name != normalized {
            continue;
        }
        let record_type = record.record_type.trim().to_ascii_uppercase();
        let ttl = record.ttl.unwrap_or(60).clamp(0, i64::from(u32::MAX)) as u32;
        match (qtype, record_type.as_str()) {
            (DNS_TYPE_A, "A") => {
                let ip = record
                    .target_ip
                    .as_deref()?
                    .trim()
                    .parse::<Ipv4Addr>()
                    .ok()?;
                answers.push(ResolverAnswer {
                    rr_type: DNS_TYPE_A,
                    ttl,
                    rdata: ip.octets().to_vec(),
                });
            }
            (DNS_TYPE_CNAME, "CNAME") => {
                let cname = normalize_name(record.cname.as_deref()?)?;
                answers.push(ResolverAnswer {
                    rr_type: DNS_TYPE_CNAME,
                    ttl,
                    rdata: encode_qname(&cname)?,
                });
            }
            _ => {}
        }
    }
    (!answers.is_empty()).then_some(answers)
}

fn build_resolver_ipv4_response(
    packet: &[u8],
    query: &ResolverIpv4Query,
    answers: &[ResolverAnswer],
    reply_ip: &str,
) -> Option<Vec<u8>> {
    let dns_query = &packet[query.dns_offset..query.dns_offset + query.dns_len];
    let mut dns = Vec::with_capacity(query.dns_len + answers.len() * 32);
    dns.extend_from_slice(&query.dns_id);
    dns.extend_from_slice(&0x8180u16.to_be_bytes());
    dns.extend_from_slice(&1u16.to_be_bytes());
    dns.extend_from_slice(&(answers.len() as u16).to_be_bytes());
    dns.extend_from_slice(&0u16.to_be_bytes());
    dns.extend_from_slice(&0u16.to_be_bytes());
    dns.extend_from_slice(&dns_query[12..query.question_end]);
    for answer in answers {
        dns.extend_from_slice(&0xc00cu16.to_be_bytes());
        dns.extend_from_slice(&answer.rr_type.to_be_bytes());
        dns.extend_from_slice(&DNS_CLASS_IN.to_be_bytes());
        dns.extend_from_slice(&answer.ttl.to_be_bytes());
        dns.extend_from_slice(&(answer.rdata.len() as u16).to_be_bytes());
        dns.extend_from_slice(&answer.rdata);
    }

    let udp_len = 8 + dns.len();
    let total_len = query.ip_header_len + udp_len;
    let mut reply = vec![0_u8; total_len];
    reply[..query.ip_header_len].copy_from_slice(&packet[..query.ip_header_len]);
    let reply_ip = reply_ip.parse::<Ipv4Addr>().ok()?;
    reply[12..16].copy_from_slice(&reply_ip.octets());
    reply[16..20].copy_from_slice(&packet[12..16]);
    reply[2..4].copy_from_slice(&(total_len as u16).to_be_bytes());
    reply[8] = 64;
    reply[10] = 0;
    reply[11] = 0;

    let udp = query.udp_offset;
    reply[udp..udp + 2].copy_from_slice(&DNS_PORT.to_be_bytes());
    reply[udp + 2..udp + 4].copy_from_slice(&query.source_port.to_be_bytes());
    reply[udp + 4..udp + 6].copy_from_slice(&(udp_len as u16).to_be_bytes());
    reply[udp + 6] = 0;
    reply[udp + 7] = 0;
    reply[query.dns_offset..query.dns_offset + dns.len()].copy_from_slice(&dns);

    Some(crate::normalize_ipv4_transport_checksums(&reply))
}

fn parse_qname(dns: &[u8], mut offset: usize) -> Option<(String, usize)> {
    let mut labels = Vec::new();
    loop {
        let len = usize::from(*dns.get(offset)?);
        offset += 1;
        if len == 0 {
            break;
        }
        if (len & 0xc0) != 0 || offset + len > dns.len() {
            return None;
        }
        let label = std::str::from_utf8(&dns[offset..offset + len]).ok()?.trim();
        if label.is_empty() {
            return None;
        }
        labels.push(label.to_ascii_lowercase());
        offset += len;
    }
    Some((labels.join("."), offset))
}

fn encode_qname(name: &str) -> Option<Vec<u8>> {
    let normalized = normalize_name(name)?;
    let mut out = Vec::new();
    for label in normalized.split('.') {
        if label.is_empty() || label.len() > 63 {
            return None;
        }
        out.push(label.len() as u8);
        out.extend_from_slice(label.as_bytes());
    }
    out.push(0);
    Some(out)
}

fn normalize_name(value: &str) -> Option<String> {
    let trimmed = value.trim().trim_end_matches('.').to_ascii_lowercase();
    (!trimmed.is_empty()).then_some(trimmed)
}

#[cfg(test)]
mod tests {
    use std::net::Ipv4Addr;

    use super::resolver_response_for_query;
    use crate::{ipv4_destination, ipv4_source, PlatformResolverRecord};

    #[test]
    fn responds_to_a_record_query_for_local_dns_ip() {
        let query = resolver_query_packet("10.0.0.1", "10.0.0.53", 53000, "mac.test.lan", 1);
        let response = resolver_response_for_query(
            &query,
            "10.0.0.53/32",
            &[PlatformResolverRecord {
                record_id: "record-1".to_string(),
                zone_id: "zone-1".to_string(),
                network_id: "network-1".to_string(),
                name: "mac".to_string(),
                fqdn: Some("mac.test.lan".to_string()),
                record_type: "A".to_string(),
                target_ip: Some("10.0.0.9".to_string()),
                ttl: Some(120),
                ..PlatformResolverRecord::default()
            }],
        )
        .unwrap();

        assert_eq!(ipv4_source(&response).as_deref(), Some("10.0.0.53"));
        assert_eq!(ipv4_destination(&response).as_deref(), Some("10.0.0.1"));
        assert_eq!(&response[20..22], &53u16.to_be_bytes());
        assert_eq!(&response[22..24], &53000u16.to_be_bytes());
        assert_eq!(&response[32..34], &1u16.to_be_bytes());
        assert_eq!(&response[34..36], &1u16.to_be_bytes());
    }

    #[test]
    fn ignores_non_matching_destination_or_unknown_record() {
        let query = resolver_query_packet("10.0.0.1", "10.0.0.53", 53000, "mac.test.lan", 1);
        assert!(resolver_response_for_query(&query, "10.0.0.2/32", &[]).is_none());
        assert!(resolver_response_for_query(
            &query,
            "10.0.0.53/32",
            &[PlatformResolverRecord {
                fqdn: Some("other.test.lan".to_string()),
                record_type: "A".to_string(),
                target_ip: Some("10.0.0.9".to_string()),
                ..PlatformResolverRecord::default()
            }]
        )
        .is_none());
    }

    fn resolver_query_packet(
        src_ip: &str,
        dst_ip: &str,
        src_port: u16,
        name: &str,
        qtype: u16,
    ) -> Vec<u8> {
        let mut dns = Vec::new();
        dns.extend_from_slice(&0x1234u16.to_be_bytes());
        dns.extend_from_slice(&0x0100u16.to_be_bytes());
        dns.extend_from_slice(&1u16.to_be_bytes());
        dns.extend_from_slice(&0u16.to_be_bytes());
        dns.extend_from_slice(&0u16.to_be_bytes());
        dns.extend_from_slice(&0u16.to_be_bytes());
        for label in name.split('.') {
            dns.push(label.len() as u8);
            dns.extend_from_slice(label.as_bytes());
        }
        dns.push(0);
        dns.extend_from_slice(&qtype.to_be_bytes());
        dns.extend_from_slice(&1u16.to_be_bytes());

        let udp_len = 8 + dns.len();
        let total_len = 20 + udp_len;
        let mut packet = vec![0_u8; total_len];
        packet[0] = 0x45;
        packet[2..4].copy_from_slice(&(total_len as u16).to_be_bytes());
        packet[8] = 64;
        packet[9] = 17;
        packet[12..16].copy_from_slice(&src_ip.parse::<Ipv4Addr>().unwrap().octets());
        packet[16..20].copy_from_slice(&dst_ip.parse::<Ipv4Addr>().unwrap().octets());
        packet[20..22].copy_from_slice(&src_port.to_be_bytes());
        packet[22..24].copy_from_slice(&53u16.to_be_bytes());
        packet[24..26].copy_from_slice(&(udp_len as u16).to_be_bytes());
        packet[28..28 + dns.len()].copy_from_slice(&dns);
        crate::normalize_ipv4_transport_checksums(&packet)
    }
}
