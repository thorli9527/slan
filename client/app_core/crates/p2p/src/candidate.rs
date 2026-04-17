/// 对端候选信息。
#[derive(Debug, Clone)]
pub struct PeerCandidate {
    pub peer_node_id: String,
    pub endpoint: String,
    pub candidate_type: String,
}
