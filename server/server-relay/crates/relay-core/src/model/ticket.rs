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
    pub country_code: Option<String>,
    pub city_code: Option<String>,
    pub allowed_derp_node_ids: Vec<String>,
    pub relay_url: String,
    pub expires_at: String,
    pub session_key: String,
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

    /// 生成与控制面一致的稳定签名载荷。
    pub fn signing_payload(&self) -> String {
        let mut allowed_derp_node_ids = self.allowed_derp_node_ids.clone();
        allowed_derp_node_ids.sort();
        [
            self.ticket_id.as_str(),
            self.network_id.as_str(),
            self.session_id.as_str(),
            self.src_node_id.as_str(),
            self.dst_node_id.as_str(),
            self.derp_cluster_id.as_deref().unwrap_or(""),
            self.country_code.as_deref().unwrap_or(""),
            self.city_code.as_deref().unwrap_or(""),
            &allowed_derp_node_ids.join(","),
            self.relay_url.as_str(),
            self.expires_at.as_str(),
            self.session_key.as_str(),
        ]
        .join("|")
    }
}
