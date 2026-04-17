use std::time::{SystemTime, UNIX_EPOCH};

use crate::{parse_timestamp, RelayError};

/// 由控制面签发、供 relay 数据面消费的票据。
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RelayTicket {
    pub ticket_id: String,
    pub network_id: String,
    pub session_id: String,
    pub src_node_id: String,
    pub dst_node_id: String,
    pub derp_cluster_id: Option<String>,
    pub allowed_derp_node_ids: Vec<String>,
    pub relay_url: String,
    pub expires_at: String,
    pub signature: String,
}

impl RelayTicket {
    /// 判断票据在指定 unix 时间戳下是否已过期。
    pub fn is_expired_at(&self, now_epoch_seconds: i64) -> Result<bool, RelayError> {
        let expires_at = parse_timestamp(&self.expires_at)?;
        Ok(now_epoch_seconds >= expires_at)
    }

    /// 按当前系统时间判断票据是否过期。
    pub fn is_expired(&self) -> Result<bool, RelayError> {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_err(|_| RelayError::ClockSkew)?
            .as_secs() as i64;
        self.is_expired_at(now)
    }
}

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
