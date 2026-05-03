pub const SLAN_RELAY_MAGIC: &[u8; 4] = b"SLAN";
pub const SLAN_RELAY_VERSION: u8 = 1;
pub const SLAN_RELAY_FRAME_TYPE_DATA: u8 = 1;
pub const SLAN_RELAY_HEADER_LEN: usize = 32;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct RelayDataFrame<'a> {
    pub seq: u64,
    pub config_hash: u64,
    pub payload: &'a [u8],
}

pub fn is_slan_relay_frame(packet: &[u8]) -> bool {
    packet.len() >= 4 && &packet[..4] == SLAN_RELAY_MAGIC
}

pub fn encode_data_frame(seq: u64, config_hash: u64, payload: &[u8]) -> Result<Vec<u8>, String> {
    let payload_len =
        u32::try_from(payload.len()).map_err(|_| "relay payload is too large".to_string())?;
    let mut frame = Vec::with_capacity(SLAN_RELAY_HEADER_LEN + payload.len());
    frame.extend_from_slice(SLAN_RELAY_MAGIC);
    frame.push(SLAN_RELAY_VERSION);
    frame.push(SLAN_RELAY_FRAME_TYPE_DATA);
    frame.extend_from_slice(&(SLAN_RELAY_HEADER_LEN as u16).to_be_bytes());
    frame.extend_from_slice(&seq.to_be_bytes());
    frame.extend_from_slice(&config_hash.to_be_bytes());
    frame.extend_from_slice(&payload_len.to_be_bytes());
    frame.extend_from_slice(&0_u32.to_be_bytes());
    frame.extend_from_slice(payload);
    Ok(frame)
}

pub fn decode_data_frame(frame: &[u8]) -> Result<RelayDataFrame<'_>, String> {
    if frame.len() < SLAN_RELAY_HEADER_LEN {
        return Err("relay frame is shorter than header".to_string());
    }
    if &frame[..4] != SLAN_RELAY_MAGIC {
        return Err("relay frame magic mismatch".to_string());
    }
    if frame[4] != SLAN_RELAY_VERSION {
        return Err(format!("unsupported relay frame version {}", frame[4]));
    }
    if frame[5] != SLAN_RELAY_FRAME_TYPE_DATA {
        return Err(format!("unsupported relay frame type {}", frame[5]));
    }

    let header_len = u16::from_be_bytes([frame[6], frame[7]]) as usize;
    if header_len < SLAN_RELAY_HEADER_LEN || header_len > frame.len() {
        return Err("invalid relay frame header length".to_string());
    }
    let payload_len = u32::from_be_bytes([frame[24], frame[25], frame[26], frame[27]]) as usize;
    let end = header_len
        .checked_add(payload_len)
        .ok_or_else(|| "relay frame payload length overflow".to_string())?;
    if end > frame.len() {
        return Err("relay frame payload is truncated".to_string());
    }

    Ok(RelayDataFrame {
        seq: u64::from_be_bytes([
            frame[8], frame[9], frame[10], frame[11], frame[12], frame[13], frame[14], frame[15],
        ]),
        config_hash: u64::from_be_bytes([
            frame[16], frame[17], frame[18], frame[19], frame[20], frame[21], frame[22], frame[23],
        ]),
        payload: &frame[header_len..end],
    })
}

#[cfg(test)]
mod tests {
    use super::{decode_data_frame, encode_data_frame, is_slan_relay_frame};

    #[test]
    fn binary_frame_round_trips_payload() {
        let payload = b"\x45\x00\x00\x14packet";
        let frame = encode_data_frame(7, 42, payload).unwrap();
        let decoded = decode_data_frame(&frame).unwrap();

        assert!(is_slan_relay_frame(&frame));
        assert_eq!(decoded.seq, 7);
        assert_eq!(decoded.config_hash, 42);
        assert_eq!(decoded.payload, payload);
    }

    #[test]
    fn binary_frame_rejects_truncated_payload() {
        let mut frame = encode_data_frame(1, 2, b"packet").unwrap();
        frame.truncate(frame.len() - 2);

        assert!(decode_data_frame(&frame).is_err());
    }
}
