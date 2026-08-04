pub const SLAN_RELAY_MAGIC: &[u8; 4] = b"SLAN";
pub const SLAN_RELAY_VERSION: u8 = 1;
pub const SLAN_RELAY_FRAME_TYPE_DATA: u8 = 1;
pub const SLAN_RELAY_HEADER_LEN: usize = 32;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct DecodedRelayDataFrame<'a> {
    pub seq: u64,
    pub config_hash: u64,
    pub payload: &'a [u8],
}

pub fn encode_slan_relay_data_frame(seq: u64, config_hash: u64, payload: &[u8]) -> Option<Vec<u8>> {
    let payload_len = u32::try_from(payload.len()).ok()?;
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
    Some(frame)
}

pub fn decode_slan_relay_data_frame(frame: &[u8]) -> Option<&[u8]> {
    decode_slan_relay_data_frame_full(frame).map(|decoded| decoded.payload)
}

pub fn decode_slan_relay_data_frame_full(frame: &[u8]) -> Option<DecodedRelayDataFrame<'_>> {
    if frame.len() < SLAN_RELAY_HEADER_LEN {
        return None;
    }
    if &frame[..4] != SLAN_RELAY_MAGIC {
        return None;
    }
    if frame[4] != SLAN_RELAY_VERSION || frame[5] != SLAN_RELAY_FRAME_TYPE_DATA {
        return None;
    }
    let header_len = u16::from_be_bytes([frame[6], frame[7]]) as usize;
    if header_len < SLAN_RELAY_HEADER_LEN || header_len > frame.len() {
        return None;
    }
    let seq = u64::from_be_bytes([
        frame[8], frame[9], frame[10], frame[11], frame[12], frame[13], frame[14], frame[15],
    ]);
    let config_hash = u64::from_be_bytes([
        frame[16], frame[17], frame[18], frame[19], frame[20], frame[21], frame[22], frame[23],
    ]);
    let payload_len = u32::from_be_bytes([frame[24], frame[25], frame[26], frame[27]]) as usize;
    let end = header_len.checked_add(payload_len)?;
    if end > frame.len() {
        return None;
    }
    Some(DecodedRelayDataFrame {
        seq,
        config_hash,
        payload: &frame[header_len..end],
    })
}

pub fn stable_hash64(value: &str) -> u64 {
    let mut hash = 0xcbf29ce484222325_u64;
    for byte in value.as_bytes() {
        hash ^= u64::from(*byte);
        hash = hash.wrapping_mul(0x100000001b3);
    }
    hash
}

pub fn base64_encode(input: &[u8]) -> String {
    const TABLE: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::with_capacity(input.len().div_ceil(3) * 4);
    for chunk in input.chunks(3) {
        let b0 = chunk[0];
        let b1 = *chunk.get(1).unwrap_or(&0);
        let b2 = *chunk.get(2).unwrap_or(&0);
        out.push(TABLE[(b0 >> 2) as usize] as char);
        out.push(TABLE[(((b0 & 0x03) << 4) | (b1 >> 4)) as usize] as char);
        if chunk.len() > 1 {
            out.push(TABLE[(((b1 & 0x0f) << 2) | (b2 >> 6)) as usize] as char);
        } else {
            out.push('=');
        }
        if chunk.len() > 2 {
            out.push(TABLE[(b2 & 0x3f) as usize] as char);
        } else {
            out.push('=');
        }
    }
    out
}

pub fn base64_decode(input: &str) -> Option<Vec<u8>> {
    let bytes = input.trim().as_bytes();
    if bytes.is_empty() {
        return Some(Vec::new());
    }
    if !bytes.len().is_multiple_of(4) {
        return None;
    }
    let mut out = Vec::with_capacity(bytes.len() / 4 * 3);
    for chunk in bytes.chunks(4) {
        let v0 = base64_value(chunk[0])?;
        let v1 = base64_value(chunk[1])?;
        let pad2 = chunk[2] == b'=';
        let pad3 = chunk[3] == b'=';
        let v2 = if pad2 { 0 } else { base64_value(chunk[2])? };
        let v3 = if pad3 { 0 } else { base64_value(chunk[3])? };
        out.push((v0 << 2) | (v1 >> 4));
        if !pad2 {
            out.push(((v1 & 0x0f) << 4) | (v2 >> 2));
        }
        if !pad3 {
            out.push(((v2 & 0x03) << 6) | v3);
        }
    }
    Some(out)
}

fn base64_value(byte: u8) -> Option<u8> {
    match byte {
        b'A'..=b'Z' => Some(byte - b'A'),
        b'a'..=b'z' => Some(byte - b'a' + 26),
        b'0'..=b'9' => Some(byte - b'0' + 52),
        b'+' => Some(62),
        b'/' => Some(63),
        _ => None,
    }
}

pub fn relay_frame_is_replayed(last_rx_seq: u64, seq: u64) -> bool {
    last_rx_seq > 0 && seq <= last_rx_seq
}

#[cfg(test)]
mod tests {
    use super::{
        base64_decode, base64_encode, decode_slan_relay_data_frame,
        decode_slan_relay_data_frame_full, encode_slan_relay_data_frame, relay_frame_is_replayed,
        stable_hash64, SLAN_RELAY_HEADER_LEN,
    };

    #[test]
    fn relay_data_frame_round_trips_payload() {
        let payload = b"\x45\x00\x00\x14hello";
        let frame = encode_slan_relay_data_frame(42, stable_hash64("config"), payload).unwrap();

        assert_eq!(&frame[..4], b"SLAN");
        assert_eq!(frame.len(), SLAN_RELAY_HEADER_LEN + payload.len());
        assert_eq!(
            decode_slan_relay_data_frame(&frame),
            Some(payload.as_slice())
        );
        let decoded = decode_slan_relay_data_frame_full(&frame).unwrap();
        assert_eq!(decoded.seq, 42);
        assert_eq!(decoded.config_hash, stable_hash64("config"));
        assert_eq!(decoded.payload, payload);
    }

    #[test]
    fn relay_data_frame_rejects_invalid_magic() {
        let mut frame = encode_slan_relay_data_frame(1, 2, b"packet").unwrap();
        frame[0] = b'X';

        assert!(decode_slan_relay_data_frame(&frame).is_none());
    }

    #[test]
    fn relay_replay_guard_rejects_duplicate_or_older_sequence() {
        assert!(!relay_frame_is_replayed(0, 1));
        assert!(!relay_frame_is_replayed(10, 11));
        assert!(relay_frame_is_replayed(10, 10));
        assert!(relay_frame_is_replayed(10, 9));
    }

    #[test]
    fn base64_round_trips_payload() {
        let payload = b"SLAN\x00\xffpayload";
        let encoded = base64_encode(payload);
        assert_eq!(base64_decode(&encoded).as_deref(), Some(payload.as_slice()));
    }
}
