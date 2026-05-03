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

#[cfg(test)]
mod tests {
    use super::{ipv4_destination, ipv4_source, normalize_virtual_ip, relay_peer_index_for_packet};

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
}
