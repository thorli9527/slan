use crate::RelayError;

/// Relay 会话，描述一条中继通道两端设备的绑定关系。
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RelaySession {
    pub session_id: String,
    pub source_device_id: String,
    pub target_device_id: String,
    pub network_id: String,
}

impl RelaySession {
    /// 返回会话参与双方设备 ID。
    pub fn participants(&self) -> [&str; 2] {
        [&self.source_device_id, &self.target_device_id]
    }

    /// 判断给定设备是否属于当前 relay 会话。
    pub fn involves(&self, device_id: &str) -> bool {
        self.source_device_id == device_id || self.target_device_id == device_id
    }

    /// 给定当前设备 ID，返回会话中的对端设备 ID。
    pub fn peer_of(&self, device_id: &str) -> Result<&str, RelayError> {
        if self.source_device_id == device_id {
            return Ok(&self.target_device_id);
        }
        if self.target_device_id == device_id {
            return Ok(&self.source_device_id);
        }
        Err(RelayError::UnauthorizedPeer)
    }
}
