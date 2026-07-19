use std::net::Ipv4Addr;

use crate::platform::{PlatformAclPeer, PlatformAclPolicy, PlatformAclRule};

pub fn relay_peer_index_for_packet(peer_virtual_ips: &[&[String]], packet: &[u8]) -> Option<usize> {
    let destination = ipv4_destination(packet)?;
    peer_virtual_ips
        .iter()
        .position(|ips| ips.iter().any(|ip| normalize_virtual_ip(ip) == destination))
}

pub fn acl_allows_egress_packet(
    packet: &[u8],
    policies: &[PlatformAclPolicy],
    peer: Option<&PlatformAclPeer>,
) -> bool {
    acl_allows_packet(packet, policies, peer, "egress")
}

pub fn acl_allows_ingress_packet(
    packet: &[u8],
    policies: &[PlatformAclPolicy],
    peer: Option<&PlatformAclPeer>,
) -> bool {
    acl_allows_packet(packet, policies, peer, "ingress")
}

fn acl_allows_packet(
    packet: &[u8],
    policies: &[PlatformAclPolicy],
    peer: Option<&PlatformAclPeer>,
    direction: &str,
) -> bool {
    if ipv4_source(packet).is_none() || ipv4_destination(packet).is_none() {
        return true;
    }
    let mut has_enabled_rule = false;
    for policy in policies {
        for rule in policy.rules.iter().filter(|rule| rule.enabled) {
            has_enabled_rule = true;
            if !acl_rule_direction_matches(rule.direction.as_str(), direction) {
                continue;
            }
            if !acl_rule_protocol_matches(rule.protocol.as_str(), packet) {
                continue;
            }
            if !acl_rule_port_matches(rule, packet) {
                continue;
            }
            if !acl_rule_peer_matches(policy.network_id.as_str(), rule, packet, direction, peer) {
                continue;
            }
            return rule.action.eq_ignore_ascii_case("allow");
        }
    }
    if !has_enabled_rule {
        return true;
    }
    true
}

fn acl_rule_direction_matches(rule_direction: &str, packet_direction: &str) -> bool {
    let value = rule_direction.trim().to_ascii_lowercase();
    value.is_empty()
        || value == "all"
        || value == "any"
        || value == packet_direction
        || (packet_direction == "egress" && matches!(value.as_str(), "out" | "outbound"))
        || (packet_direction == "ingress" && matches!(value.as_str(), "in" | "inbound"))
}

fn acl_rule_protocol_matches(rule_protocol: &str, packet: &[u8]) -> bool {
    let value = rule_protocol.trim().to_ascii_lowercase();
    if value.is_empty() || value == "all" || value == "any" {
        return true;
    }
    let Some(protocol) = ipv4_protocol(packet) else {
        return true;
    };
    matches!(
        (value.as_str(), protocol),
        ("icmp", 1) | ("tcp", 6) | ("udp", 17)
    ) || value == protocol.to_string()
}

fn acl_rule_port_matches(rule: &PlatformAclRule, packet: &[u8]) -> bool {
    let from = rule.port_from.max(0);
    let to = rule.port_to.max(0);
    if from == 0 && to == 0 {
        return true;
    }
    let Some(protocol) = ipv4_protocol(packet) else {
        return false;
    };
    if protocol != 6 && protocol != 17 {
        return false;
    }
    let Some(port) = ipv4_destination_port(packet) else {
        return false;
    };
    let lower = if from == 0 { to } else { from };
    let upper = if to == 0 { from } else { to };
    i64::from(port) >= lower.min(upper) && i64::from(port) <= lower.max(upper)
}

fn acl_rule_peer_matches(
    network_id: &str,
    rule: &PlatformAclRule,
    packet: &[u8],
    packet_direction: &str,
    peer: Option<&PlatformAclPeer>,
) -> bool {
    let peer_type = rule.peer_type.trim().to_ascii_lowercase();
    let peer_value = rule.peer_value.trim();
    if matches!(peer_type.as_str(), "" | "all" | "any") {
        return matches!(peer_value, "")
            || peer_value.eq_ignore_ascii_case("all")
            || peer_value.eq_ignore_ascii_case("any")
            || peer_value == "*";
    }
    if matches!(peer_type.as_str(), "network" | "workspace") {
        return matches!(peer_value, "")
            || peer_value.eq_ignore_ascii_case("self")
            || peer_value.eq_ignore_ascii_case("all")
            || peer_value == network_id;
    }
    if matches!(peer_type.as_str(), "ip" | "cidr" | "subnet") {
        return acl_subject_ips(rule.direction.as_str(), packet_direction, packet)
            .iter()
            .any(|ip| acl_ip_matches(peer_value, ip));
    }
    if peer_type == "device" {
        let subject_ips = acl_subject_ips(rule.direction.as_str(), packet_direction, packet);
        if let Some(peer) = peer {
            let expected_node_id = format!("node-{peer_value}");
            let peer_is_subject = peer.peer_virtual_ips.iter().any(|ip| {
                subject_ips
                    .iter()
                    .any(|subject_ip| normalize_virtual_ip(ip) == *subject_ip)
            });
            if peer_is_subject
                && peer
                    .peer_node_id
                    .as_deref()
                    .is_some_and(|node_id| node_id == peer_value || node_id == expected_node_id)
            {
                return true;
            }
            if peer_is_subject
                && rule
                    .resolved_peer_node_id
                    .as_deref()
                    .is_some_and(|node_id| peer.peer_node_id.as_deref() == Some(node_id))
            {
                return true;
            }
        }
        return subject_ips.iter().any(|subject_ip| {
            rule.resolved_peer_virtual_ips
                .iter()
                .any(|ip| normalize_virtual_ip(ip) == *subject_ip)
        });
    }
    if matches!(peer_type.as_str(), "domain" | "device_group") {
        return acl_subject_ips(rule.direction.as_str(), packet_direction, packet)
            .iter()
            .any(|subject_ip| {
                rule.resolved_peer_virtual_ips
                    .iter()
                    .any(|ip| normalize_virtual_ip(ip) == *subject_ip)
            });
    }
    false
}

fn acl_subject_ips(rule_direction: &str, packet_direction: &str, packet: &[u8]) -> Vec<String> {
    let rule_direction = rule_direction.trim().to_ascii_lowercase();
    let source = ipv4_source(packet);
    let destination = ipv4_destination(packet);
    if packet_direction.eq_ignore_ascii_case("egress")
        && matches!(rule_direction.as_str(), "egress" | "out" | "outbound")
    {
        return destination.into_iter().collect();
    }
    if packet_direction.eq_ignore_ascii_case("ingress")
        && matches!(rule_direction.as_str(), "ingress" | "in" | "inbound")
    {
        return destination.into_iter().collect();
    }
    if matches!(rule_direction.as_str(), "" | "all" | "any") {
        return source.into_iter().chain(destination).collect();
    }
    destination.into_iter().collect()
}

pub fn ipv4_destination(packet: &[u8]) -> Option<String> {
    if packet.len() < 20 || packet[0] >> 4 != 4 {
        return None;
    }
    Some(format!(
        "{}.{}.{}.{}",
        packet[16], packet[17], packet[18], packet[19]
    ))
}

pub fn ipv4_source(packet: &[u8]) -> Option<String> {
    if packet.len() < 20 || packet[0] >> 4 != 4 {
        return None;
    }
    Some(format!(
        "{}.{}.{}.{}",
        packet[12], packet[13], packet[14], packet[15]
    ))
}

pub fn normalize_virtual_ip(value: &str) -> String {
    value
        .trim()
        .split_once('/')
        .map(|(ip, _)| ip)
        .unwrap_or_else(|| value.trim())
        .to_string()
}

pub fn ipv4_protocol(packet: &[u8]) -> Option<u8> {
    if packet.len() < 20 || packet[0] >> 4 != 4 {
        return None;
    }
    Some(packet[9])
}

fn ipv4_destination_port(packet: &[u8]) -> Option<u16> {
    if packet.len() < 20 || packet[0] >> 4 != 4 {
        return None;
    }
    let ihl = usize::from(packet[0] & 0x0f) * 4;
    if ihl < 20 || packet.len() < ihl + 4 {
        return None;
    }
    let flags_fragment = u16::from_be_bytes([packet[6], packet[7]]);
    if flags_fragment & 0x1fff != 0 {
        return None;
    }
    match packet[9] {
        6 | 17 => Some(u16::from_be_bytes([packet[ihl + 2], packet[ihl + 3]])),
        _ => None,
    }
}

fn acl_ip_matches(pattern: &str, ip: &str) -> bool {
    let value = pattern.trim();
    if value.is_empty() || value.eq_ignore_ascii_case("all") || value == "*" {
        return true;
    }
    if let Some((network, prefix)) = value.split_once('/') {
        return ipv4_cidr_contains(network.trim(), prefix.trim(), ip);
    }
    normalize_virtual_ip(value) == normalize_virtual_ip(ip)
}

fn ipv4_cidr_contains(network: &str, prefix: &str, ip: &str) -> bool {
    let Ok(prefix) = prefix.parse::<u32>() else {
        return false;
    };
    if prefix > 32 {
        return false;
    }
    let Ok(network) = network.parse::<Ipv4Addr>() else {
        return false;
    };
    let Ok(ip) = normalize_virtual_ip(ip).parse::<Ipv4Addr>() else {
        return false;
    };
    let mask = if prefix == 0 {
        0
    } else {
        u32::MAX << (32 - prefix)
    };
    (u32::from(network) & mask) == (u32::from(ip) & mask)
}

pub fn icmp_echo_reply_for_request(packet: &[u8], local_virtual_ip: &str) -> Option<Vec<u8>> {
    if packet.len() < 28 || packet[0] >> 4 != 4 {
        return None;
    }
    let ihl = usize::from(packet[0] & 0x0f) * 4;
    if ihl < 20 || packet.len() < ihl + 8 || packet.get(9).copied() != Some(1) {
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
    if ipv4_destination(packet)? != normalize_virtual_ip(local_virtual_ip) {
        return None;
    }
    let icmp_offset = ihl;
    if packet[icmp_offset] != 8 || packet[icmp_offset + 1] != 0 {
        return None;
    }

    let mut reply = packet[..total_len].to_vec();
    reply[12..16].copy_from_slice(&packet[16..20]);
    reply[16..20].copy_from_slice(&packet[12..16]);
    reply[8] = 64;
    reply[10] = 0;
    reply[11] = 0;
    reply[icmp_offset] = 0;
    reply[icmp_offset + 2] = 0;
    reply[icmp_offset + 3] = 0;
    let icmp_sum = internet_checksum(&reply[icmp_offset..total_len]);
    reply[icmp_offset + 2..icmp_offset + 4].copy_from_slice(&icmp_sum.to_be_bytes());
    let ip_sum = internet_checksum(&reply[..ihl]);
    reply[10..12].copy_from_slice(&ip_sum.to_be_bytes());
    Some(reply)
}

pub fn normalize_ipv4_transport_checksums(packet: &[u8]) -> Vec<u8> {
    if packet.len() < 20 || packet[0] >> 4 != 4 {
        return packet.to_vec();
    }
    let ihl = usize::from(packet[0] & 0x0f) * 4;
    if ihl < 20 || packet.len() < ihl {
        return packet.to_vec();
    }
    let total_len = usize::from(u16::from_be_bytes([packet[2], packet[3]]));
    if total_len < ihl || total_len > packet.len() {
        return packet.to_vec();
    }
    let mut normalized = packet[..total_len].to_vec();
    normalized[10] = 0;
    normalized[11] = 0;
    let ip_sum = internet_checksum(&normalized[..ihl]);
    normalized[10..12].copy_from_slice(&ip_sum.to_be_bytes());

    let flags_fragment = u16::from_be_bytes([normalized[6], normalized[7]]);
    if flags_fragment & 0x1fff != 0 {
        return normalized;
    }
    match normalized[9] {
        6 => normalize_tcp_checksum(&mut normalized, ihl, total_len),
        17 => normalize_udp_checksum(&mut normalized, ihl, total_len),
        _ => {}
    }
    normalized
}

pub fn ipv4_transport_checksum_valid(packet: &[u8]) -> Option<bool> {
    if packet.len() < 20 || packet[0] >> 4 != 4 {
        return None;
    }
    let ihl = usize::from(packet[0] & 0x0f) * 4;
    if ihl < 20 || packet.len() < ihl {
        return None;
    }
    let total_len = usize::from(u16::from_be_bytes([packet[2], packet[3]]));
    if total_len < ihl || total_len > packet.len() {
        return None;
    }
    let flags_fragment = u16::from_be_bytes([packet[6], packet[7]]);
    if flags_fragment & 0x1fff != 0 {
        return None;
    }
    match packet[9] {
        6 => {
            let tcp_len = total_len.saturating_sub(ihl);
            if tcp_len < 20 {
                return None;
            }
            Some(transport_checksum(packet, ihl, tcp_len, 6) == 0)
        }
        17 => {
            let udp_len = total_len.saturating_sub(ihl);
            if udp_len < 8 {
                return None;
            }
            let declared_len = usize::from(u16::from_be_bytes([packet[ihl + 4], packet[ihl + 5]]));
            if declared_len < 8 || declared_len > udp_len {
                return None;
            }
            let checksum = u16::from_be_bytes([packet[ihl + 6], packet[ihl + 7]]);
            Some(checksum == 0 || transport_checksum(packet, ihl, declared_len, 17) == 0)
        }
        _ => None,
    }
}

fn normalize_tcp_checksum(packet: &mut [u8], ihl: usize, total_len: usize) {
    let tcp_len = total_len.saturating_sub(ihl);
    if tcp_len < 20 || packet.len() < total_len {
        return;
    }
    packet[ihl + 16] = 0;
    packet[ihl + 17] = 0;
    let sum = transport_checksum(packet, ihl, tcp_len, 6);
    packet[ihl + 16..ihl + 18].copy_from_slice(&sum.to_be_bytes());
}

fn normalize_udp_checksum(packet: &mut [u8], ihl: usize, total_len: usize) {
    let udp_len = total_len.saturating_sub(ihl);
    if udp_len < 8 || packet.len() < total_len {
        return;
    }
    let declared_len = usize::from(u16::from_be_bytes([packet[ihl + 4], packet[ihl + 5]]));
    if declared_len < 8 || declared_len > udp_len {
        return;
    }
    packet[ihl + 6] = 0;
    packet[ihl + 7] = 0;
    let sum = transport_checksum(packet, ihl, declared_len, 17);
    let wire_sum = if sum == 0 { 0xffff } else { sum };
    packet[ihl + 6..ihl + 8].copy_from_slice(&wire_sum.to_be_bytes());
}

fn transport_checksum(
    packet: &[u8],
    transport_offset: usize,
    transport_len: usize,
    proto: u8,
) -> u16 {
    let mut bytes = Vec::with_capacity(12 + transport_len);
    bytes.extend_from_slice(&packet[12..20]);
    bytes.push(0);
    bytes.push(proto);
    bytes.extend_from_slice(&(transport_len as u16).to_be_bytes());
    bytes.extend_from_slice(&packet[transport_offset..transport_offset + transport_len]);
    internet_checksum(&bytes)
}

fn internet_checksum(bytes: &[u8]) -> u16 {
    let mut sum = 0_u32;
    for chunk in bytes.chunks(2) {
        let word = if chunk.len() == 2 {
            u16::from_be_bytes([chunk[0], chunk[1]]) as u32
        } else {
            (u16::from(chunk[0]) << 8) as u32
        };
        sum = sum.wrapping_add(word);
    }
    while (sum >> 16) != 0 {
        sum = (sum & 0xffff) + (sum >> 16);
    }
    !(sum as u16)
}

#[cfg(test)]
mod tests {
    use super::{
        acl_allows_egress_packet, acl_allows_ingress_packet, icmp_echo_reply_for_request,
        ipv4_destination, ipv4_source, normalize_virtual_ip, relay_peer_index_for_packet,
    };
    use crate::{PlatformAclPeer, PlatformAclPolicy, PlatformAclRule};

    #[test]
    fn extracts_ipv4_source_and_destination() {
        let packet = ipv4_packet("10.0.0.2", "10.0.0.9");

        assert_eq!(ipv4_source(&packet).as_deref(), Some("10.0.0.2"));
        assert_eq!(ipv4_destination(&packet).as_deref(), Some("10.0.0.9"));
    }

    #[test]
    fn rejects_non_ipv4_packets() {
        assert_eq!(ipv4_destination(&[]), None);
        assert_eq!(ipv4_destination(&[0x60; 20]), None);
    }

    #[test]
    fn routes_by_destination_virtual_ip() {
        let peers = [
            vec!["10.0.0.2/32".to_string()],
            vec!["10.0.0.9/32".to_string()],
        ];
        let refs = peers.iter().map(Vec::as_slice).collect::<Vec<_>>();

        assert_eq!(
            relay_peer_index_for_packet(&refs, &ipv4_packet("10.0.0.1", "10.0.0.9")),
            Some(1)
        );
        assert_eq!(
            relay_peer_index_for_packet(&refs, &ipv4_packet("10.0.0.1", "10.0.0.99")),
            None
        );
    }

    #[test]
    fn normalizes_cidr_virtual_ip() {
        assert_eq!(normalize_virtual_ip(" 10.0.0.2/32 "), "10.0.0.2");
        assert_eq!(normalize_virtual_ip("10.0.0.2"), "10.0.0.2");
    }

    #[test]
    fn builds_icmp_echo_reply_for_local_virtual_ip() {
        let request = icmp_echo_request("10.0.0.1", "10.0.0.2");
        let reply = icmp_echo_reply_for_request(&request, "10.0.0.2/32").unwrap();

        assert_eq!(ipv4_source(&reply).as_deref(), Some("10.0.0.2"));
        assert_eq!(ipv4_destination(&reply).as_deref(), Some("10.0.0.1"));
        assert_eq!(reply[20], 0);
        assert_eq!(reply[21], 0);
        assert_eq!(&reply[24..], &request[24..]);
        assert!(icmp_echo_reply_for_request(&request, "10.0.0.9").is_none());
    }

    #[test]
    fn acl_allows_when_no_enabled_rules_exist() {
        let packet = tcp_packet("10.0.0.2", "10.0.0.3", 443);

        assert!(acl_allows_egress_packet(&packet, &[], None));
        assert!(acl_allows_egress_packet(
            &packet,
            &[PlatformAclPolicy {
                network_id: "network-1".to_string(),
                rules: vec![PlatformAclRule {
                    enabled: false,
                    action: "deny".to_string(),
                    ..PlatformAclRule::default()
                }],
            }],
            None
        ));
    }

    #[test]
    fn acl_device_group_allow_all_accepts_every_tcp_and_udp_port() {
        let policy = PlatformAclPolicy {
            network_id: "network-1".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "allow-group-all".to_string(),
                direction: "all".to_string(),
                priority: 1,
                action: "allow".to_string(),
                protocol: "all".to_string(),
                port_from: 0,
                port_to: 0,
                peer_type: "device_group".to_string(),
                peer_value: "group-1".to_string(),
                enabled: true,
                resolved_peer_virtual_ips: vec!["10.0.0.2".to_string(), "10.0.0.3".to_string()],
                ..PlatformAclRule::default()
            }],
        };

        for port in [1, 22, 80, 443, 8080, 65_535] {
            assert!(acl_allows_egress_packet(
                &tcp_packet("10.0.0.2", "10.0.0.3", port),
                std::slice::from_ref(&policy),
                None,
            ));
            assert!(acl_allows_ingress_packet(
                &tcp_packet("10.0.0.3", "10.0.0.2", port),
                std::slice::from_ref(&policy),
                None,
            ));
            assert!(acl_allows_egress_packet(
                &udp_packet("10.0.0.2", "10.0.0.3", port),
                std::slice::from_ref(&policy),
                None,
            ));
            assert!(acl_allows_ingress_packet(
                &udp_packet("10.0.0.3", "10.0.0.2", port),
                std::slice::from_ref(&policy),
                None,
            ));
        }
    }

    #[test]
    fn acl_denies_matching_device_and_protocol_port() {
        let packet = tcp_packet("10.0.0.2", "10.0.0.3", 443);
        let policy = PlatformAclPolicy {
            network_id: "network-1".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "rule-1".to_string(),
                direction: "egress".to_string(),
                priority: 10,
                action: "deny".to_string(),
                protocol: "tcp".to_string(),
                port_from: 443,
                port_to: 443,
                peer_type: "device".to_string(),
                peer_value: "peer-device".to_string(),
                enabled: true,
                resolved_peer_node_id: Some("node-peer-device".to_string()),
                resolved_peer_virtual_ips: vec!["10.0.0.3".to_string()],
                ..PlatformAclRule::default()
            }],
        };
        let peer = PlatformAclPeer {
            peer_node_id: Some("node-peer-device".to_string()),
            peer_virtual_ips: vec!["10.0.0.3".to_string()],
        };

        assert!(!acl_allows_egress_packet(&packet, &[policy], Some(&peer)));
    }

    #[test]
    fn acl_device_rule_does_not_match_unrelated_peer_route() {
        let packet = tcp_packet("10.0.0.2", "10.0.0.3", 443);
        let policy = PlatformAclPolicy {
            network_id: "network-1".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "rule-1".to_string(),
                direction: "egress".to_string(),
                priority: 10,
                action: "deny".to_string(),
                protocol: "tcp".to_string(),
                port_from: 443,
                port_to: 443,
                peer_type: "device".to_string(),
                peer_value: "other-device".to_string(),
                enabled: true,
                resolved_peer_node_id: Some("node-other-device".to_string()),
                resolved_peer_virtual_ips: vec!["10.0.0.9".to_string()],
                ..PlatformAclRule::default()
            }],
        };
        let peer = PlatformAclPeer {
            peer_node_id: Some("node-peer-device".to_string()),
            peer_virtual_ips: vec!["10.0.0.3".to_string()],
        };

        assert!(acl_allows_egress_packet(&packet, &[policy], Some(&peer)));
    }

    #[test]
    fn acl_priority_allow_overrides_later_deny() {
        let packet = icmp_echo_request("10.0.0.2", "10.0.0.3");
        let policy = PlatformAclPolicy {
            network_id: "network-1".to_string(),
            rules: vec![
                PlatformAclRule {
                    rule_id: "allow".to_string(),
                    direction: "egress".to_string(),
                    priority: 5,
                    action: "allow".to_string(),
                    protocol: "icmp".to_string(),
                    peer_type: "cidr".to_string(),
                    peer_value: "10.0.0.0/24".to_string(),
                    enabled: true,
                    ..PlatformAclRule::default()
                },
                PlatformAclRule {
                    rule_id: "deny".to_string(),
                    direction: "egress".to_string(),
                    priority: 10,
                    action: "deny".to_string(),
                    protocol: "all".to_string(),
                    peer_type: "all".to_string(),
                    enabled: true,
                    ..PlatformAclRule::default()
                },
            ],
        };

        assert!(acl_allows_egress_packet(&packet, &[policy], None));
    }

    #[test]
    fn acl_policy_order_allows_newer_network_to_override_same_port_rule() {
        let packet = tcp_packet("10.0.0.2", "10.0.0.3", 443);
        let newer = PlatformAclPolicy {
            network_id: "new-network".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "new-deny".to_string(),
                direction: "egress".to_string(),
                priority: 1,
                action: "deny".to_string(),
                protocol: "tcp".to_string(),
                port_from: 443,
                port_to: 443,
                peer_type: "all".to_string(),
                peer_value: "all".to_string(),
                enabled: true,
                ..PlatformAclRule::default()
            }],
        };
        let older = PlatformAclPolicy {
            network_id: "old-network".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "old-allow".to_string(),
                direction: "egress".to_string(),
                priority: 1,
                action: "allow".to_string(),
                protocol: "tcp".to_string(),
                port_from: 443,
                port_to: 443,
                peer_type: "all".to_string(),
                peer_value: "all".to_string(),
                enabled: true,
                ..PlatformAclRule::default()
            }],
        };

        assert!(!acl_allows_egress_packet(&packet, &[newer, older], None));
    }

    #[test]
    fn acl_applies_ingress_direction_to_destination_ip() {
        let packet = tcp_packet("10.0.0.3", "10.0.0.2", 22);
        let policy = PlatformAclPolicy {
            network_id: "network-1".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "deny".to_string(),
                direction: "ingress".to_string(),
                priority: 1,
                action: "deny".to_string(),
                protocol: "tcp".to_string(),
                port_from: 22,
                port_to: 22,
                peer_type: "cidr".to_string(),
                peer_value: "10.0.0.2/32".to_string(),
                enabled: true,
                ..PlatformAclRule::default()
            }],
        };

        assert!(!acl_allows_ingress_packet(&packet, &[policy], None));
    }

    #[test]
    fn acl_ingress_device_rule_does_not_match_remote_peer_node_only() {
        let packet = icmp_echo_request("10.0.0.3", "10.0.0.2");
        let policy = PlatformAclPolicy {
            network_id: "network-1".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "deny-remote-ingress".to_string(),
                direction: "ingress".to_string(),
                priority: 1,
                action: "deny".to_string(),
                protocol: "all".to_string(),
                peer_type: "device".to_string(),
                peer_value: "remote-device".to_string(),
                enabled: true,
                resolved_peer_node_id: Some("node-remote-device".to_string()),
                resolved_peer_virtual_ips: vec!["10.0.0.3/32".to_string()],
                ..PlatformAclRule::default()
            }],
        };
        let peer = PlatformAclPeer {
            peer_node_id: Some("node-remote-device".to_string()),
            peer_virtual_ips: vec!["10.0.0.3/32".to_string()],
        };

        assert!(acl_allows_ingress_packet(&packet, &[policy], Some(&peer)));
    }

    #[test]
    fn acl_ingress_device_rule_matches_local_destination_ip() {
        let packet = icmp_echo_request("10.0.0.3", "10.0.0.2");
        let policy = PlatformAclPolicy {
            network_id: "network-1".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "deny-local-ingress".to_string(),
                direction: "ingress".to_string(),
                priority: 1,
                action: "deny".to_string(),
                protocol: "all".to_string(),
                peer_type: "device".to_string(),
                peer_value: "local-device".to_string(),
                enabled: true,
                resolved_peer_node_id: Some("node-local-device".to_string()),
                resolved_peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                ..PlatformAclRule::default()
            }],
        };

        assert!(!acl_allows_ingress_packet(&packet, &[policy], None));
    }

    #[test]
    fn acl_all_direction_matches_source_or_destination_ip() {
        let packet = icmp_echo_request("10.0.0.2", "10.0.0.3");
        let policy = PlatformAclPolicy {
            network_id: "network-1".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "deny-source-or-destination".to_string(),
                direction: "all".to_string(),
                priority: 1,
                action: "deny".to_string(),
                protocol: "all".to_string(),
                peer_type: "cidr".to_string(),
                peer_value: "10.0.0.2/32".to_string(),
                enabled: true,
                ..PlatformAclRule::default()
            }],
        };

        assert!(!acl_allows_egress_packet(&packet, &[policy], None));
    }

    #[test]
    fn acl_network_peer_value_must_match_current_policy_network() {
        let packet = icmp_echo_request("10.0.0.2", "10.0.0.3");
        let policy = PlatformAclPolicy {
            network_id: "network-a".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "deny-other-network".to_string(),
                direction: "egress".to_string(),
                priority: 1,
                action: "deny".to_string(),
                protocol: "all".to_string(),
                peer_type: "network".to_string(),
                peer_value: "network-b".to_string(),
                enabled: true,
                ..PlatformAclRule::default()
            }],
        };

        assert!(acl_allows_egress_packet(&packet, &[policy], None));
    }

    #[test]
    fn acl_domain_rule_uses_resolved_virtual_ips() {
        let packet = icmp_echo_request("10.0.0.2", "10.0.0.7");
        let policy = PlatformAclPolicy {
            network_id: "network-a".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "deny-domain".to_string(),
                direction: "egress".to_string(),
                priority: 1,
                action: "deny".to_string(),
                protocol: "icmp".to_string(),
                peer_type: "domain".to_string(),
                peer_value: "build.slan".to_string(),
                enabled: true,
                resolved_peer_virtual_ips: vec!["10.0.0.7/32".to_string()],
                ..PlatformAclRule::default()
            }],
        };

        assert!(!acl_allows_egress_packet(&packet, &[policy], None));
    }

    #[test]
    fn acl_unknown_peer_type_does_not_match() {
        let packet = icmp_echo_request("10.0.0.2", "10.0.0.3");
        let policy = PlatformAclPolicy {
            network_id: "network-a".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "deny-unknown".to_string(),
                direction: "egress".to_string(),
                priority: 1,
                action: "deny".to_string(),
                protocol: "all".to_string(),
                peer_type: "unsupported".to_string(),
                peer_value: "10.0.0.3".to_string(),
                enabled: true,
                ..PlatformAclRule::default()
            }],
        };

        assert!(acl_allows_egress_packet(&packet, &[policy], None));
    }

    fn ipv4_packet(src: &str, dst: &str) -> Vec<u8> {
        let mut packet = vec![0_u8; 20];
        packet[0] = 0x45;
        for (index, part) in src.split('.').enumerate().take(4) {
            packet[12 + index] = part.parse::<u8>().unwrap();
        }
        for (index, part) in dst.split('.').enumerate().take(4) {
            packet[16 + index] = part.parse::<u8>().unwrap();
        }
        packet
    }

    fn tcp_packet(src: &str, dst: &str, dst_port: u16) -> Vec<u8> {
        let mut packet = ipv4_packet(src, dst);
        packet.resize(40, 0);
        packet[2..4].copy_from_slice(&(40_u16).to_be_bytes());
        packet[8] = 64;
        packet[9] = 6;
        packet[20..22].copy_from_slice(&12345_u16.to_be_bytes());
        packet[22..24].copy_from_slice(&dst_port.to_be_bytes());
        packet[32] = 0x50;
        let ip_sum = super::internet_checksum(&packet[..20]);
        packet[10..12].copy_from_slice(&ip_sum.to_be_bytes());
        packet
    }

    fn udp_packet(src: &str, dst: &str, dst_port: u16) -> Vec<u8> {
        let mut packet = ipv4_packet(src, dst);
        packet.resize(28, 0);
        packet[2..4].copy_from_slice(&(28_u16).to_be_bytes());
        packet[8] = 64;
        packet[9] = 17;
        packet[20..22].copy_from_slice(&12345_u16.to_be_bytes());
        packet[22..24].copy_from_slice(&dst_port.to_be_bytes());
        packet[24..26].copy_from_slice(&8_u16.to_be_bytes());
        let ip_sum = super::internet_checksum(&packet[..20]);
        packet[10..12].copy_from_slice(&ip_sum.to_be_bytes());
        packet
    }

    fn icmp_echo_request(src: &str, dst: &str) -> Vec<u8> {
        let mut packet = ipv4_packet(src, dst);
        packet.resize(32, 0);
        packet[2..4].copy_from_slice(&(32_u16).to_be_bytes());
        packet[8] = 64;
        packet[9] = 1;
        packet[20] = 8;
        packet[21] = 0;
        packet[24..26].copy_from_slice(&7_u16.to_be_bytes());
        packet[26..28].copy_from_slice(&9_u16.to_be_bytes());
        packet[28..32].copy_from_slice(b"slan");
        let icmp_sum = super::internet_checksum(&packet[20..]);
        packet[22..24].copy_from_slice(&icmp_sum.to_be_bytes());
        let ip_sum = super::internet_checksum(&packet[..20]);
        packet[10..12].copy_from_slice(&ip_sum.to_be_bytes());
        packet
    }
}
