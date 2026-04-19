use serde::{Deserialize, Serialize};

/// 当前生效的数据路径。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum ActivePath {
    None,
    P2P { peer_node_id: String },
    Relay { peer_node_id: String },
    Derp { cluster_id: String, node_id: String },
}

/// 连接路径。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ConnectionPath {
    P2P,
    Relay,
    Derp,
}

/// 连接状态。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum ConnectionState {
    Disconnected,
    Connecting,
    Connected(ConnectionPath),
    Failed(String),
}
