use std::collections::HashMap;

use serde::{Deserialize, Serialize};

/// PathKind 表示客户端数据面可选的链路类型，顺序通常从低成本直连到保底转发。
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PathKind {
    /// 局域网 UDP 直连，优先级最高，依赖同网段可达地址。
    #[serde(rename = "lan_udp")]
    LanUdp,
    /// 公网 IPv6 UDP 直连。
    #[serde(rename = "ipv6_udp")]
    Ipv6Udp,
    /// 通过 punch 或端点上报得到的公网 UDP 直连。
    #[serde(rename = "direct_udp")]
    DirectUdp,
    /// UDP relay 转发。
    #[serde(rename = "relay_udp")]
    RelayUdp,
    /// DERP TCP/TLS 443 保底转发。
    #[serde(rename = "derp_tcp_tls_443")]
    DerpTcpTls443,
}

impl PathKind {
    /// 返回与控制面 JSON 协议一致的路径字符串。
    pub fn as_str(self) -> &'static str {
        match self {
            Self::LanUdp => "lan_udp",
            Self::Ipv6Udp => "ipv6_udp",
            Self::DirectUdp => "direct_udp",
            Self::RelayUdp => "relay_udp",
            Self::DerpTcpTls443 => "derp_tcp_tls_443",
        }
    }

    /// 判断该路径是否属于服务端转发类路径。
    pub fn is_relay(self) -> bool {
        matches!(self, Self::RelayUdp | Self::DerpTcpTls443)
    }
}

/// 将控制面下发的 relay transport 归一化为客户端支持的 transport。
pub fn normalize_relay_transport(value: &str) -> Option<&'static str> {
    match value.trim().to_ascii_lowercase().as_str() {
        "udp" | "relay_udp" | "relay+udp" => Some("udp"),
        "derp" | "derp_tcp_tls_443" | "derp+tcp+tls" => Some("derp_tcp_tls_443"),
        _ => None,
    }
}

/// 将 relay transport 映射为客户端路径类型。
pub fn relay_path_kind_for_transport(value: &str) -> Option<PathKind> {
    match normalize_relay_transport(value)? {
        "udp" => Some(PathKind::RelayUdp),
        "derp_tcp_tls_443" => Some(PathKind::DerpTcpTls443),
        _ => None,
    }
}

/// PathState 表示某条候选路径在客户端当前探测周期中的状态。
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PathState {
    /// 路径被禁用，不参与选择。
    Disabled,
    /// 路径正在探测，尚未确认可用。
    Probing,
    /// 路径已探测成功，可以承载流量。
    Ready,
    /// 路径可作为备用，但当前不主动使用。
    Standby,
    /// 路径降级可用或质量较差。
    Degraded,
    /// 路径失败，需要等待冷却或重新探测。
    Failed,
}

/// PathPolicy 是客户端路径选择和故障切换策略。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PathPolicy {
    /// 路径偏好顺序，默认 LAN/IPv6/Direct/Relay/DERP。
    pub preferred: Vec<PathKind>,
    /// 是否允许故障时自动切换到后备路径。
    #[serde(default = "default_fallback_enabled")]
    pub fallback_enabled: bool,
    /// 主动探测间隔，单位毫秒。
    #[serde(default = "default_probe_interval_ms")]
    pub probe_interval_ms: u64,
    /// 发送失败持续多久后触发 failover，单位毫秒。
    #[serde(default = "default_failover_after_ms")]
    pub failover_after_ms: u64,
    /// 从 relay 升级到更优路径前需要连续成功探测次数。
    #[serde(default = "default_upgrade_successes")]
    pub upgrade_successes: u32,
    /// 失败路径重新参与升级探测前需要等待的探测轮数。
    #[serde(default = "default_failed_path_cooldown_probes")]
    pub failed_path_cooldown_probes: u32,
}

impl Default for PathPolicy {
    fn default() -> Self {
        Self {
            preferred: vec![
                PathKind::LanUdp,
                PathKind::Ipv6Udp,
                PathKind::DirectUdp,
                PathKind::RelayUdp,
                PathKind::DerpTcpTls443,
            ],
            fallback_enabled: true,
            probe_interval_ms: default_probe_interval_ms(),
            failover_after_ms: default_failover_after_ms(),
            upgrade_successes: default_upgrade_successes(),
            failed_path_cooldown_probes: default_failed_path_cooldown_probes(),
        }
    }
}

/// PeerPathConfig 是控制面下发给数据面的单个 peer 路径候选配置。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PeerPathConfig {
    /// 对端节点 ID。
    pub peer_node_id: String,
    /// 对端在虚拟网络中的 IP 列表，用于把 TUN 包路由到 peer。
    #[serde(default)]
    pub peer_virtual_ips: Vec<String>,
    /// 该 peer 的可用路径候选。
    #[serde(default)]
    pub candidates: Vec<PathCandidate>,
}

/// PathCandidate 是单条候选链路及其探测/调度元数据。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PathCandidate {
    /// 路径类型。
    pub kind: PathKind,
    /// 当前路径状态。
    #[serde(default = "default_candidate_state")]
    pub state: PathState,
    /// relay endpoint 或其它控制面端点 ID。
    #[serde(default)]
    pub endpoint_id: Option<String>,
    /// 直连地址或 relay 地址。
    #[serde(default)]
    pub address: Option<String>,
    /// relay/DERP 会话 ID。
    #[serde(default)]
    pub session_id: Option<String>,
    /// 传输协议，例如 udp。
    #[serde(default)]
    pub transport: Option<String>,
    /// 最近探测 RTT，单位毫秒。
    #[serde(default)]
    pub rtt_ms: Option<u32>,
    /// 控制面或客户端计算的路径评分。
    #[serde(default)]
    pub path_score: Option<u32>,
    /// 最近一次成功时间，Unix 毫秒。
    #[serde(default)]
    pub last_ok_at_ms: Option<u64>,
    /// 最近一次错误说明。
    #[serde(default)]
    pub last_error: Option<String>,
}

/// PeerPathRuntime 是平台数据面上报给 UI/核心的 peer 路径运行状态。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PeerPathRuntime {
    pub peer_node_id: String,
    #[serde(default)]
    pub peer_virtual_ips: Vec<String>,
    pub active_path: Option<PathKind>,
    #[serde(default)]
    pub candidates: Vec<PathCandidate>,
}

/// PathTracker 在本地跟踪每个 peer 的活跃路径、发送失败和升级探测状态。
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PathTracker {
    policy: PathPolicy,
    active_paths: HashMap<String, PathKind>,
    send_failures: HashMap<(String, PathKind), u32>,
    send_failure_started_at_ms: HashMap<(String, PathKind), u64>,
    probe_successes: HashMap<(String, PathKind), u32>,
    failed_path_cooldowns: HashMap<(String, PathKind), u32>,
}

impl PathTracker {
    /// 创建路径 tracker，并可传入已有 peer 活跃路径作为初始状态。
    pub fn new(
        policy: PathPolicy,
        active_paths: impl IntoIterator<Item = (String, PathKind)>,
    ) -> Self {
        Self {
            policy,
            active_paths: active_paths.into_iter().collect(),
            send_failures: HashMap::new(),
            send_failure_started_at_ms: HashMap::new(),
            probe_successes: HashMap::new(),
            failed_path_cooldowns: HashMap::new(),
        }
    }

    /// 返回是否允许自动 fallback。
    pub fn fallback_enabled(&self) -> bool {
        self.policy.fallback_enabled
    }

    /// 应用平台层上报的 peer 路径运行状态。
    pub fn apply_runtime_paths(&mut self, peer_paths: &[PeerPathRuntime]) {
        for path in peer_paths {
            if let Some(active_path) = path.active_path {
                self.active_paths
                    .insert(path.peer_node_id.clone(), active_path);
            }
        }
    }

    /// 设置某个 peer 当前活跃路径。
    pub fn set_active_path(&mut self, peer_node_id: impl Into<String>, active_path: PathKind) {
        self.active_paths.insert(peer_node_id.into(), active_path);
    }

    /// 查询某个 peer 当前活跃路径，未知时返回调用方提供的默认路径。
    pub fn active_path_for_node(&self, peer_node_id: &str, default: PathKind) -> PathKind {
        self.active_paths
            .get(peer_node_id)
            .copied()
            .unwrap_or(default)
    }

    /// 记录某个 peer 发送成功，并清理该 peer 的失败计数。
    pub fn record_send_success(&mut self, peer_node_id: &str) {
        self.clear_peer_send_failures(peer_node_id);
    }

    /// 记录一次发送失败，达到阈值时返回 true 表示应触发降级。
    pub fn record_send_failure(
        &mut self,
        peer_node_id: &str,
        path_kind: PathKind,
        failure_threshold: u32,
    ) -> bool {
        self.record_send_failure_at(peer_node_id, path_kind, failure_threshold, None)
    }

    /// 带时间戳的发送失败记录，用于按 failover_after_ms 判断持续失败。
    pub fn record_send_failure_at(
        &mut self,
        peer_node_id: &str,
        path_kind: PathKind,
        failure_threshold: u32,
        now_ms: Option<u64>,
    ) -> bool {
        if !self.policy.fallback_enabled || path_kind == PathKind::RelayUdp {
            return false;
        }
        let key = (peer_node_id.to_string(), path_kind);
        let failures = self.send_failures.entry(key.clone()).or_insert(0);
        *failures = failures.saturating_add(1);
        if let Some(now_ms) = now_ms {
            self.send_failure_started_at_ms
                .entry(key.clone())
                .or_insert(now_ms);
        }
        let failed_long_enough = now_ms
            .zip(self.send_failure_started_at_ms.get(&key).copied())
            .is_some_and(|(now_ms, started_at_ms)| {
                now_ms.saturating_sub(started_at_ms) >= self.policy.failover_after_ms
            });
        if *failures < failure_threshold && !failed_long_enough {
            return false;
        }
        let peer_node_id = peer_node_id.to_string();
        self.failed_path_cooldowns.insert(
            (peer_node_id.clone(), path_kind),
            self.policy.failed_path_cooldown_probes.max(1),
        );
        self.probe_successes
            .remove(&(peer_node_id.clone(), path_kind));
        self.clear_peer_send_failures(&peer_node_id);
        true
    }

    /// 记录一次路径探测成功，达到升级阈值时返回 true。
    pub fn record_probe_success(&mut self, peer_node_id: &str, path_kind: PathKind) -> bool {
        self.record_probe_success_after(peer_node_id, path_kind, self.policy.upgrade_successes)
    }

    /// 按指定连续成功阈值记录探测成功。
    pub fn record_probe_success_after(
        &mut self,
        peer_node_id: &str,
        path_kind: PathKind,
        success_threshold: u32,
    ) -> bool {
        self.clear_peer_send_failures(peer_node_id);
        let current = self.active_path_for_node(peer_node_id, PathKind::RelayUdp);
        if current == path_kind || !path_should_upgrade(&self.policy, current, path_kind) {
            self.probe_successes
                .remove(&(peer_node_id.to_string(), path_kind));
            return false;
        }
        let cooldown_key = (peer_node_id.to_string(), path_kind);
        if let Some(remaining) = self.failed_path_cooldowns.get_mut(&cooldown_key) {
            *remaining = remaining.saturating_sub(1);
            if *remaining == 0 {
                self.failed_path_cooldowns.remove(&cooldown_key);
            }
            return false;
        }
        let successes = self
            .probe_successes
            .entry((peer_node_id.to_string(), path_kind))
            .or_insert(0);
        *successes = successes.saturating_add(1);
        if *successes < success_threshold.max(1) {
            return false;
        }
        self.active_paths
            .insert(peer_node_id.to_string(), path_kind);
        self.probe_successes
            .remove(&(peer_node_id.to_string(), path_kind));
        true
    }

    /// 返回当前所有 peer 活跃路径的摘要，用于诊断展示。
    pub fn active_path_summary(&self) -> String {
        let mut paths = self
            .active_paths
            .values()
            .copied()
            .collect::<Vec<PathKind>>();
        paths.sort_by_key(|path| path.as_str());
        paths.dedup();
        match paths.as_slice() {
            [] => "unknown".to_string(),
            [path] => path.as_str().to_string(),
            _ => "mixed".to_string(),
        }
    }

    /// 返回策略配置中的路径偏好顺序。
    pub fn preferred_paths(&self) -> Vec<PathKind> {
        preferred_path_order(&self.policy)
    }

    fn clear_peer_send_failures(&mut self, peer_node_id: &str) {
        self.send_failures
            .retain(|(failure_peer_node_id, _), _| failure_peer_node_id != peer_node_id);
        self.send_failure_started_at_ms
            .retain(|(failure_peer_node_id, _), _| failure_peer_node_id != peer_node_id);
    }
}

/// 从候选路径里选择当前应使用的活跃路径。
pub fn select_active_path(
    policy: &PathPolicy,
    current: Option<PathKind>,
    candidates: &[PathCandidate],
) -> Option<PathKind> {
    if let Some(current) = current {
        if !policy.fallback_enabled && path_is_ready(candidates, current) {
            return Some(current);
        }
    }
    for preferred in preferred_path_order(policy) {
        if path_is_ready(candidates, preferred) {
            return Some(preferred);
        }
    }
    current.filter(|kind| path_is_ready(candidates, *kind))
}

pub fn path_should_upgrade(policy: &PathPolicy, current: PathKind, candidate: PathKind) -> bool {
    let order = preferred_path_order(policy);
    let current_rank = order
        .iter()
        .position(|path| *path == current)
        .unwrap_or(usize::MAX);
    let candidate_rank = order
        .iter()
        .position(|path| *path == candidate)
        .unwrap_or(usize::MAX);
    candidate_rank < current_rank
}

pub fn preferred_path_order(policy: &PathPolicy) -> Vec<PathKind> {
    if policy.preferred.is_empty() {
        return PathPolicy::default().preferred;
    }
    let mut values = Vec::new();
    for kind in &policy.preferred {
        if !values.contains(kind) {
            values.push(*kind);
        }
    }
    values
}

pub fn update_peer_active_path(
    peer_paths: &mut [PeerPathRuntime],
    peer_node_id: &str,
    active_path: PathKind,
) {
    let Some(path) = peer_paths
        .iter_mut()
        .find(|path| path.peer_node_id == peer_node_id)
    else {
        return;
    };
    path.active_path = Some(active_path);
    for candidate in &mut path.candidates {
        if candidate.kind == active_path {
            candidate.state = PathState::Ready;
            candidate.last_error = None;
        } else if candidate.kind != PathKind::RelayUdp {
            candidate.state = PathState::Degraded;
            candidate.last_error = Some("downgraded after consecutive send failures".to_string());
        }
    }
}

pub fn mark_peer_path_probe_success(
    peer_paths: &mut [PeerPathRuntime],
    peer_node_id: &str,
    active_path: PathKind,
    now_ms: u64,
) {
    let Some(path) = peer_paths
        .iter_mut()
        .find(|path| path.peer_node_id == peer_node_id)
    else {
        return;
    };
    path.active_path = Some(active_path);
    for candidate in &mut path.candidates {
        if candidate.kind == active_path {
            candidate.state = PathState::Ready;
            candidate.last_error = None;
            candidate.last_ok_at_ms = Some(now_ms);
        }
    }
}

pub fn selected_runtime_paths(
    policy: &PathPolicy,
    mut peer_paths: Vec<PeerPathRuntime>,
) -> Vec<PeerPathRuntime> {
    for peer in &mut peer_paths {
        peer.active_path = select_active_path(policy, peer.active_path, &peer.candidates);
    }
    peer_paths
}

pub fn mark_path_ready_for_nodes(
    mut peer_paths: Vec<PeerPathRuntime>,
    node_ids: impl IntoIterator<Item = impl AsRef<str>>,
    kind: PathKind,
) -> Vec<PeerPathRuntime> {
    for node_id in node_ids {
        mark_path_ready_for_node(&mut peer_paths, node_id.as_ref(), kind);
    }
    peer_paths
}

pub fn mark_path_ready_for_node(
    peer_paths: &mut [PeerPathRuntime],
    peer_node_id: &str,
    kind: PathKind,
) {
    let Some(path) = peer_paths
        .iter_mut()
        .find(|path| path.peer_node_id == peer_node_id)
    else {
        return;
    };
    if let Some(candidate) = path
        .candidates
        .iter_mut()
        .find(|candidate| candidate.kind == kind)
    {
        candidate.state = PathState::Ready;
        candidate.last_error = None;
    }
}

fn path_is_ready(candidates: &[PathCandidate], kind: PathKind) -> bool {
    candidates
        .iter()
        .any(|candidate| candidate.kind == kind && candidate.state == PathState::Ready)
}

fn default_fallback_enabled() -> bool {
    true
}

fn default_probe_interval_ms() -> u64 {
    15_000
}

fn default_failover_after_ms() -> u64 {
    30_000
}

fn default_upgrade_successes() -> u32 {
    2
}

fn default_failed_path_cooldown_probes() -> u32 {
    2
}

fn default_candidate_state() -> PathState {
    PathState::Standby
}

#[cfg(test)]
mod tests {
    use super::*;

    fn candidate(kind: PathKind, state: PathState) -> PathCandidate {
        PathCandidate {
            kind,
            state,
            endpoint_id: None,
            address: None,
            session_id: None,
            transport: None,
            rtt_ms: None,
            path_score: None,
            last_ok_at_ms: None,
            last_error: None,
        }
    }

    #[test]
    fn select_active_path_prefers_ready_direct_udp() {
        let policy = PathPolicy::default();
        let candidates = vec![
            candidate(PathKind::RelayUdp, PathState::Ready),
            candidate(PathKind::DirectUdp, PathState::Ready),
        ];

        assert_eq!(
            select_active_path(&policy, Some(PathKind::RelayUdp), &candidates),
            Some(PathKind::DirectUdp)
        );
    }

    #[test]
    fn select_active_path_keeps_current_when_fallback_is_disabled() {
        let policy = PathPolicy {
            fallback_enabled: false,
            ..PathPolicy::default()
        };
        let candidates = vec![
            candidate(PathKind::RelayUdp, PathState::Ready),
            candidate(PathKind::DirectUdp, PathState::Ready),
        ];

        assert_eq!(
            select_active_path(&policy, Some(PathKind::RelayUdp), &candidates),
            Some(PathKind::RelayUdp)
        );
    }

    #[test]
    fn path_should_upgrade_uses_policy_order() {
        let policy = PathPolicy::default();

        assert!(path_should_upgrade(
            &policy,
            PathKind::RelayUdp,
            PathKind::DirectUdp
        ));
        assert!(!path_should_upgrade(
            &policy,
            PathKind::DirectUdp,
            PathKind::RelayUdp
        ));
        assert!(path_should_upgrade(
            &policy,
            PathKind::DerpTcpTls443,
            PathKind::RelayUdp
        ));
        assert!(!path_should_upgrade(
            &policy,
            PathKind::RelayUdp,
            PathKind::DerpTcpTls443
        ));
    }

    #[test]
    fn preferred_path_order_deduplicates_and_defaults_empty_policy() {
        let policy = PathPolicy {
            preferred: vec![
                PathKind::RelayUdp,
                PathKind::RelayUdp,
                PathKind::DerpTcpTls443,
            ],
            ..PathPolicy::default()
        };

        assert_eq!(
            preferred_path_order(&policy),
            vec![PathKind::RelayUdp, PathKind::DerpTcpTls443]
        );
        assert_eq!(
            preferred_path_order(&PathPolicy {
                preferred: Vec::new(),
                ..PathPolicy::default()
            }),
            PathPolicy::default().preferred
        );
    }

    #[test]
    fn derp_tcp_tls_443_is_relay_path_kind() {
        assert_eq!(PathKind::DerpTcpTls443.as_str(), "derp_tcp_tls_443");
        assert!(PathKind::DerpTcpTls443.is_relay());
    }

    #[test]
    fn relay_transport_accepts_udp_only() {
        for value in ["udp", " UDP "] {
            assert_eq!(normalize_relay_transport(value), Some("udp"));
            assert_eq!(
                relay_path_kind_for_transport(value),
                Some(PathKind::RelayUdp)
            );
        }
        for value in ["tcp", "tls", "http3", "h3", "quic"] {
            assert_eq!(normalize_relay_transport(value), None);
            assert_eq!(relay_path_kind_for_transport(value), None);
        }
    }

    #[test]
    fn update_peer_active_path_marks_downgraded_candidates() {
        let mut paths = vec![PeerPathRuntime {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
            active_path: Some(PathKind::DirectUdp),
            candidates: vec![
                candidate(PathKind::DirectUdp, PathState::Ready),
                candidate(PathKind::RelayUdp, PathState::Ready),
            ],
        }];

        update_peer_active_path(&mut paths, "node-a", PathKind::RelayUdp);

        assert_eq!(paths[0].active_path, Some(PathKind::RelayUdp));
        assert!(paths[0].candidates.iter().any(|candidate| {
            candidate.kind == PathKind::DirectUdp
                && candidate.state == PathState::Degraded
                && candidate.last_error.is_some()
        }));
    }

    #[test]
    fn mark_peer_path_probe_success_sets_ready_timestamp() {
        let mut paths = vec![PeerPathRuntime {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
            active_path: Some(PathKind::RelayUdp),
            candidates: vec![
                candidate(PathKind::DirectUdp, PathState::Standby),
                candidate(PathKind::RelayUdp, PathState::Ready),
            ],
        }];

        mark_peer_path_probe_success(&mut paths, "node-a", PathKind::DirectUdp, 1234);

        assert_eq!(paths[0].active_path, Some(PathKind::DirectUdp));
        assert!(paths[0].candidates.iter().any(|candidate| {
            candidate.kind == PathKind::DirectUdp
                && candidate.state == PathState::Ready
                && candidate.last_ok_at_ms == Some(1234)
                && candidate.last_error.is_none()
        }));
    }

    #[test]
    fn selected_runtime_paths_applies_policy_to_all_peers() {
        let peers = vec![
            PeerPathRuntime {
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec![],
                active_path: Some(PathKind::RelayUdp),
                candidates: vec![
                    candidate(PathKind::RelayUdp, PathState::Ready),
                    candidate(PathKind::DirectUdp, PathState::Ready),
                ],
            },
            PeerPathRuntime {
                peer_node_id: "node-b".to_string(),
                peer_virtual_ips: vec![],
                active_path: None,
                candidates: vec![candidate(PathKind::DerpTcpTls443, PathState::Ready)],
            },
        ];

        let selected = selected_runtime_paths(&PathPolicy::default(), peers);

        assert_eq!(selected[0].active_path, Some(PathKind::DirectUdp));
        assert_eq!(selected[1].active_path, Some(PathKind::DerpTcpTls443));
    }

    #[test]
    fn mark_path_ready_for_nodes_updates_matching_candidates() {
        let paths = vec![
            PeerPathRuntime {
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec![],
                active_path: None,
                candidates: vec![candidate(PathKind::DirectUdp, PathState::Standby)],
            },
            PeerPathRuntime {
                peer_node_id: "node-b".to_string(),
                peer_virtual_ips: vec![],
                active_path: None,
                candidates: vec![candidate(PathKind::DirectUdp, PathState::Standby)],
            },
        ];

        let marked = mark_path_ready_for_nodes(paths, ["node-a"], PathKind::DirectUdp);

        assert_eq!(marked[0].candidates[0].state, PathState::Ready);
        assert_eq!(marked[1].candidates[0].state, PathState::Standby);
    }

    #[test]
    fn path_tracker_records_send_failure_threshold_without_inventing_fallback() {
        let mut tracker = PathTracker::new(
            PathPolicy::default(),
            [("node-a".to_string(), PathKind::DirectUdp)],
        );

        assert!(!tracker.record_send_failure("node-a", PathKind::DirectUdp, 3));
        assert!(!tracker.record_send_failure("node-a", PathKind::DirectUdp, 3));
        assert!(tracker.record_send_failure("node-a", PathKind::DirectUdp, 3));
        assert_eq!(
            tracker.active_path_for_node("node-a", PathKind::RelayUdp),
            PathKind::DirectUdp
        );
    }

    #[test]
    fn path_tracker_upgrades_on_better_probe_success() {
        let mut tracker = PathTracker::new(
            PathPolicy::default(),
            [("node-a".to_string(), PathKind::RelayUdp)],
        );

        assert!(!tracker.record_probe_success("node-a", PathKind::DirectUdp));
        assert!(tracker.record_probe_success("node-a", PathKind::DirectUdp));
        assert_eq!(
            tracker.active_path_for_node("node-a", PathKind::RelayUdp),
            PathKind::DirectUdp
        );
    }

    #[test]
    fn path_tracker_uses_policy_upgrade_success_threshold() {
        let mut tracker = PathTracker::new(
            PathPolicy {
                upgrade_successes: 3,
                ..PathPolicy::default()
            },
            [("node-a".to_string(), PathKind::RelayUdp)],
        );

        assert!(!tracker.record_probe_success("node-a", PathKind::DirectUdp));
        assert!(!tracker.record_probe_success("node-a", PathKind::DirectUdp));
        assert!(tracker.record_probe_success("node-a", PathKind::DirectUdp));
    }

    #[test]
    fn path_tracker_uses_policy_failed_path_cooldown() {
        let mut tracker = PathTracker::new(
            PathPolicy {
                failed_path_cooldown_probes: 3,
                ..PathPolicy::default()
            },
            [("node-a".to_string(), PathKind::DirectUdp)],
        );

        assert!(tracker.record_send_failure("node-a", PathKind::DirectUdp, 1));
        tracker.set_active_path("node-a", PathKind::RelayUdp);

        assert!(!tracker.record_probe_success_after("node-a", PathKind::DirectUdp, 1));
        assert!(!tracker.record_probe_success_after("node-a", PathKind::DirectUdp, 1));
        assert!(!tracker.record_probe_success_after("node-a", PathKind::DirectUdp, 1));
        assert!(tracker.record_probe_success_after("node-a", PathKind::DirectUdp, 1));
    }

    #[test]
    fn path_tracker_delays_failed_path_upgrade_until_cooldown_expires() {
        let mut tracker = PathTracker::new(
            PathPolicy::default(),
            [("node-a".to_string(), PathKind::DirectUdp)],
        );

        assert!(tracker.record_send_failure("node-a", PathKind::DirectUdp, 1));
        assert_eq!(
            tracker.active_path_for_node("node-a", PathKind::RelayUdp),
            PathKind::DirectUdp
        );
        tracker.set_active_path("node-a", PathKind::RelayUdp);
        assert!(!tracker.record_probe_success_after("node-a", PathKind::DirectUdp, 1));
        assert!(!tracker.record_probe_success_after("node-a", PathKind::DirectUdp, 1));
        assert!(tracker.record_probe_success_after("node-a", PathKind::DirectUdp, 1));
        assert_eq!(
            tracker.active_path_for_node("node-a", PathKind::RelayUdp),
            PathKind::DirectUdp
        );
    }

    #[test]
    fn path_tracker_flags_failover_window_without_inventing_fallback() {
        let mut tracker = PathTracker::new(
            PathPolicy {
                failover_after_ms: 5_000,
                ..PathPolicy::default()
            },
            [("node-a".to_string(), PathKind::DirectUdp)],
        );

        assert!(!tracker.record_send_failure_at("node-a", PathKind::DirectUdp, 99, Some(10_000)));
        assert!(!tracker.record_send_failure_at("node-a", PathKind::DirectUdp, 99, Some(14_999)));
        assert!(tracker.record_send_failure_at("node-a", PathKind::DirectUdp, 99, Some(15_000)));
        assert_eq!(
            tracker.active_path_for_node("node-a", PathKind::RelayUdp),
            PathKind::DirectUdp
        );
    }

    #[test]
    fn path_tracker_reports_mixed_summary() {
        let tracker = PathTracker::new(
            PathPolicy::default(),
            [
                ("node-a".to_string(), PathKind::DirectUdp),
                ("node-b".to_string(), PathKind::RelayUdp),
            ],
        );

        assert_eq!(tracker.active_path_summary(), "mixed");
    }
}
