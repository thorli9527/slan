/// 待转发的 UDP 包抽象。
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RelayPacket {
    pub session_id: String,
    pub from_device_id: String,
    pub payload: Vec<u8>,
}

/// 转发后的 UDP 包抽象。
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ForwardedPacket {
    pub session_id: String,
    pub to_device_id: String,
    pub payload: Vec<u8>,
}
