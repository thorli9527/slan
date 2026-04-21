use std::fs;
use std::path::Path;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DaemonConfig {
    pub udp_bind: String,
    pub relay_url_prefix: Option<String>,
    pub ticket_signing_secret: Option<String>,
}

impl Default for DaemonConfig {
    fn default() -> Self {
        Self {
            udp_bind: "0.0.0.0:9000".into(),
            relay_url_prefix: Some("udp://".into()),
            ticket_signing_secret: None,
        }
    }
}

impl DaemonConfig {
    /// from_file 从 JSON 配置文件加载 daemon 配置。
    pub fn from_file(path: impl AsRef<Path>) -> Result<Self, String> {
        let path = path.as_ref();
        let payload = fs::read_to_string(path)
            .map_err(|err| format!("failed to read config {}: {err}", path.display()))?;
        serde_json::from_str(&payload)
            .map_err(|err| format!("failed to parse config {}: {err}", path.display()))
    }
}
