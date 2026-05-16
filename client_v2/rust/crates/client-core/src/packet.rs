pub fn relay_peer_index_for_packet(peer_virtual_ips: &[&[String]], packet: &[u8]) -> Option<usize> {
    let destination = ipv4_destination(packet)?;
    peer_virtual_ips
        .iter()
        .position(|ips| ips.iter().any(|ip| normalize_virtual_ip(ip) == destination))
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
        icmp_echo_reply_for_request, ipv4_destination, ipv4_source, normalize_virtual_ip,
        relay_peer_index_for_packet,
    };

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
