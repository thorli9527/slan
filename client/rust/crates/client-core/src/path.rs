use std::{cmp::Ordering, collections::HashMap};

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

    /// 判断该路径是否使用点对点 UDP 数据面。
    pub fn is_direct_udp(self) -> bool {
        matches!(self, Self::LanUdp | Self::Ipv6Udp | Self::DirectUdp)
    }

    /// 返回固定的数据面选择优先级，数值越小越优先。
    pub fn priority(self) -> u8 {
        match self {
            Self::LanUdp => 0,
            Self::Ipv6Udp => 1,
            Self::DirectUdp => 2,
            Self::RelayUdp => 3,
            Self::DerpTcpTls443 => 4,
        }
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

/// 客户端本地路径探测角色。服务端只提供候选，不参与探测周期或选路。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PathProbeRole {
    Active,
    Standby,
}

/// 单条候选路径的本地健康状态。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PathProbeHealth {
    Healthy,
    Suspect,
    Failed,
    Recovering,
}

/// 客户端固定的自适应探测策略。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ClientPathProbePolicy {
    pub active_interval_ms: u64,
    pub standby_interval_ms: u64,
    pub suspect_interval_ms: u64,
    pub failure_window_ms: u64,
    pub failure_threshold: u32,
    pub recovery_successes: u32,
    pub min_timeout_ms: u64,
    pub max_timeout_ms: u64,
}

impl Default for ClientPathProbePolicy {
    fn default() -> Self {
        Self {
            active_interval_ms: 5_000,
            standby_interval_ms: 10_000,
            suspect_interval_ms: 1_000,
            failure_window_ms: 5_000,
            failure_threshold: 3,
            recovery_successes: 3,
            min_timeout_ms: 1_000,
            max_timeout_ms: 3_000,
        }
    }
}

/// 单条候选路径的客户端探测状态机。
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PathProbeController {
    policy: ClientPathProbePolicy,
    health: PathProbeHealth,
    usable: bool,
    last_probe_at_ms: Option<u64>,
    probe_pending: bool,
    suspect_since_ms: Option<u64>,
    consecutive_failures: u32,
    recovery_successes: u32,
    rtt_ewma_ms: Option<u64>,
    failed_until_ms: Option<u64>,
    backoff_level: u8,
}

impl PathProbeController {
    pub fn new() -> Self {
        Self::with_policy(ClientPathProbePolicy::default())
    }

    pub fn with_policy(policy: ClientPathProbePolicy) -> Self {
        Self {
            policy,
            health: PathProbeHealth::Healthy,
            usable: false,
            last_probe_at_ms: None,
            probe_pending: false,
            suspect_since_ms: None,
            consecutive_failures: 0,
            recovery_successes: 0,
            rtt_ewma_ms: None,
            failed_until_ms: None,
            backoff_level: 0,
        }
    }

    pub fn health(&self) -> PathProbeHealth {
        self.health
    }

    pub fn usable(&self) -> bool {
        self.usable
    }

    pub fn probe_timeout_ms(&self) -> u64 {
        self.rtt_ewma_ms
            .map(|rtt| rtt.saturating_mul(3))
            .unwrap_or(self.policy.min_timeout_ms)
            .clamp(self.policy.min_timeout_ms, self.policy.max_timeout_ms)
    }

    /// 推进状态并判断此刻是否应发送探测包。
    pub fn should_probe(&mut self, now_ms: u64, role: PathProbeRole) -> bool {
        self.expire_pending_probe(now_ms);
        if self.health == PathProbeHealth::Failed {
            if self.failed_until_ms.is_some_and(|until| now_ms < until) {
                return false;
            }
            self.health = PathProbeHealth::Recovering;
            self.recovery_successes = 0;
            self.probe_pending = false;
        }
        if self.health == PathProbeHealth::Suspect
            && (self.consecutive_failures >= self.policy.failure_threshold
                || self.suspect_since_ms.is_some_and(|started| {
                    now_ms.saturating_sub(started) >= self.policy.failure_window_ms
                }))
        {
            self.fail(now_ms);
            return false;
        }
        let interval = match self.health {
            PathProbeHealth::Suspect | PathProbeHealth::Recovering => {
                self.policy.suspect_interval_ms
            }
            PathProbeHealth::Healthy if role == PathProbeRole::Active => {
                self.policy.active_interval_ms
            }
            PathProbeHealth::Healthy => self.policy.standby_interval_ms,
            PathProbeHealth::Failed => return false,
        };
        !self.probe_pending
            && self
                .last_probe_at_ms
                .is_none_or(|last| now_ms.saturating_sub(last) >= interval)
    }

    pub fn on_probe_sent(&mut self, now_ms: u64) {
        self.last_probe_at_ms = Some(now_ms);
        self.probe_pending = true;
    }

    /// 任意已认证的控制包或数据帧都可证明路径双向可达。
    pub fn on_inbound(&mut self, now_ms: u64) {
        if let Some(sent_at) = self.last_probe_at_ms.filter(|_| self.probe_pending) {
            let sample = now_ms.saturating_sub(sent_at).max(1);
            self.rtt_ewma_ms = Some(match self.rtt_ewma_ms {
                Some(previous) => previous.saturating_mul(7).saturating_add(sample) / 8,
                None => sample,
            });
        }
        self.probe_pending = false;
        self.consecutive_failures = 0;
        self.suspect_since_ms = None;
        if !self.usable {
            if self.health == PathProbeHealth::Recovering {
                self.recovery_successes = self.recovery_successes.saturating_add(1);
                if self.recovery_successes < self.policy.recovery_successes.max(1) {
                    return;
                }
            }
            self.usable = true;
        }
        self.health = PathProbeHealth::Healthy;
        self.recovery_successes = 0;
        self.failed_until_ms = None;
        self.backoff_level = 0;
    }

    fn expire_pending_probe(&mut self, now_ms: u64) {
        let Some(sent_at) = self.last_probe_at_ms.filter(|_| self.probe_pending) else {
            return;
        };
        if now_ms.saturating_sub(sent_at) < self.probe_timeout_ms() {
            return;
        }
        self.probe_pending = false;
        self.consecutive_failures = self.consecutive_failures.saturating_add(1);
        if self.health == PathProbeHealth::Healthy {
            self.health = PathProbeHealth::Suspect;
            self.suspect_since_ms = Some(now_ms);
        } else if self.health == PathProbeHealth::Recovering {
            self.recovery_successes = 0;
        }
    }

    fn fail(&mut self, now_ms: u64) {
        const BACKOFF_MS: [u64; 4] = [10_000, 30_000, 60_000, 120_000];
        let index = usize::from(self.backoff_level).min(BACKOFF_MS.len() - 1);
        self.health = PathProbeHealth::Failed;
        self.usable = false;
        self.probe_pending = false;
        self.failed_until_ms = Some(now_ms.saturating_add(BACKOFF_MS[index]));
        self.backoff_level = self.backoff_level.saturating_add(1);
    }
}

impl Default for PathProbeController {
    fn default() -> Self {
        Self::new()
    }
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
    /// 从 relay 升级到更优路径前需要连续成功探测次数。
    #[serde(default = "default_upgrade_successes")]
    pub upgrade_successes: u32,
    /// 失败路径重新参与升级探测前需要等待的探测轮数。
    #[serde(default = "default_failed_path_cooldown_probes")]
    pub failed_path_cooldown_probes: u32,
    /// 当前路径仍健康时，新路径至少需要领先的综合分值。
    #[serde(default = "default_switch_hysteresis_score")]
    pub switch_hysteresis_score: u32,
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
            upgrade_successes: default_upgrade_successes(),
            failed_path_cooldown_probes: default_failed_path_cooldown_probes(),
            switch_hysteresis_score: default_switch_hysteresis_score(),
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
    direct_path_failed_at: HashMap<(String, PathKind), (u64, String)>,
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
            direct_path_failed_at: HashMap::new(),
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

    /// 带时间戳的发送失败记录，持续失败窗口由客户端固定策略决定。
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
                now_ms.saturating_sub(started_at_ms)
                    >= ClientPathProbePolicy::default().failure_window_ms
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

    /// 将指定 peer 的点对点 UDP 路径候选标记为失败，使其不再被 select_active_path 选中。
    /// 用于探测超时后降级直连路径、切换到 relay 的场景。
    pub fn mark_direct_path_failed(
        &mut self,
        peer_node_id: &str,
        path_kind: PathKind,
        now_ms: u64,
        error: String,
    ) {
        if !path_kind.is_direct_udp() {
            return;
        }
        let key = (peer_node_id.to_string(), path_kind);
        self.failed_path_cooldowns
            .insert(key.clone(), self.policy.failed_path_cooldown_probes.max(1));
        self.probe_successes.remove(&key);
        self.direct_path_failed_at.insert(key, (now_ms, error));
    }

    /// 返回指定 peer 的点对点路径是否在冷却中（因探测失败被降级）。
    pub fn is_direct_path_in_cooldown(&self, peer_node_id: &str, path_kind: PathKind) -> bool {
        self.failed_path_cooldowns
            .contains_key(&(peer_node_id.to_string(), path_kind))
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
    let best = candidates
        .iter()
        .filter(|candidate| candidate.state == PathState::Ready)
        .min_by(|left, right| compare_path_candidates(left, right));
    let Some(best) = best else {
        return current.filter(|kind| path_is_ready(candidates, *kind));
    };
    if let Some(current_kind) = current {
        if let Some(current_candidate) = candidates
            .iter()
            .filter(|candidate| {
                candidate.kind == current_kind && candidate.state == PathState::Ready
            })
            .min_by(|left, right| compare_path_candidates(left, right))
        {
            if best.kind != current_kind
                && path_candidate_score(best).saturating_add(policy.switch_hysteresis_score)
                    >= path_candidate_score(current_candidate)
            {
                return Some(current_kind);
            }
        }
    }
    Some(best.kind)
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

/// 按综合链路质量比较候选；Ordering::Less 表示更优。
pub fn compare_path_candidates(left: &PathCandidate, right: &PathCandidate) -> Ordering {
    path_state_rank(left.state)
        .cmp(&path_state_rank(right.state))
        .then_with(|| path_candidate_score(left).cmp(&path_candidate_score(right)))
        .then_with(|| left.kind.priority().cmp(&right.kind.priority()))
        .then_with(|| {
            left.rtt_ms
                .unwrap_or(u32::MAX)
                .cmp(&right.rtt_ms.unwrap_or(u32::MAX))
        })
        .then_with(|| {
            right
                .last_ok_at_ms
                .unwrap_or(0)
                .cmp(&left.last_ok_at_ms.unwrap_or(0))
        })
        .then_with(|| {
            left.address
                .as_deref()
                .unwrap_or_default()
                .cmp(right.address.as_deref().unwrap_or_default())
        })
}

/// 路径基础成本与客户端/服务端观测质量的综合评分，越低越优。
pub fn path_candidate_score(candidate: &PathCandidate) -> u32 {
    let base: u32 = match candidate.kind {
        PathKind::LanUdp => 0,
        PathKind::Ipv6Udp => 20,
        PathKind::DirectUdp => 40,
        PathKind::RelayUdp => 140,
        PathKind::DerpTcpTls443 => 280,
    };
    let quality = candidate.path_score.unwrap_or_else(|| {
        candidate
            .rtt_ms
            .map(|rtt| rtt.saturating_mul(4) / 5)
            .unwrap_or(0)
    });
    base.saturating_add(quality)
}

pub fn sort_path_candidates(candidates: &mut [PathCandidate]) {
    candidates.sort_by(compare_path_candidates);
}

fn path_state_rank(state: PathState) -> u8 {
    match state {
        PathState::Ready => 0,
        PathState::Standby => 1,
        PathState::Probing => 2,
        PathState::Degraded => 3,
        PathState::Failed => 4,
        PathState::Disabled => 5,
    }
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
    let previous_path = path.active_path.replace(active_path);
    for candidate in &mut path.candidates {
        if candidate.kind == active_path {
            candidate.state = PathState::Ready;
            candidate.last_error = None;
        } else if previous_path == Some(candidate.kind) {
            candidate.state = PathState::Degraded;
            candidate.last_error = Some("downgraded after consecutive send failures".to_string());
        } else if candidate.state == PathState::Ready {
            candidate.state = PathState::Standby;
            candidate.last_error = None;
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

fn default_upgrade_successes() -> u32 {
    2
}

fn default_failed_path_cooldown_probes() -> u32 {
    2
}

fn default_switch_hysteresis_score() -> u32 {
    25
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
    fn candidate_sort_combines_transport_cost_and_quality() {
        let mut slow_lan = candidate(PathKind::LanUdp, PathState::Ready);
        slow_lan.rtt_ms = Some(80);
        let mut fast_direct = candidate(PathKind::DirectUdp, PathState::Ready);
        fast_direct.rtt_ms = Some(5);
        let mut slow_relay = candidate(PathKind::RelayUdp, PathState::Standby);
        slow_relay.path_score = Some(200);
        let mut fast_relay = candidate(PathKind::RelayUdp, PathState::Standby);
        fast_relay.path_score = Some(40);
        let mut candidates = vec![slow_relay, fast_direct, fast_relay, slow_lan];

        sort_path_candidates(&mut candidates);

        assert_eq!(candidates[0].kind, PathKind::DirectUdp);
        assert_eq!(candidates[1].kind, PathKind::LanUdp);
        assert_eq!(candidates[2].path_score, Some(40));
        assert_eq!(candidates[3].path_score, Some(200));
    }

    #[test]
    fn selection_keeps_current_path_inside_hysteresis_margin() {
        let mut lan = candidate(PathKind::LanUdp, PathState::Ready);
        lan.rtt_ms = Some(60);
        let mut direct = candidate(PathKind::DirectUdp, PathState::Ready);
        direct.rtt_ms = Some(5);

        assert_eq!(
            select_active_path(
                &PathPolicy::default(),
                Some(PathKind::LanUdp),
                &[lan, direct]
            ),
            Some(PathKind::LanUdp)
        );
    }

    #[test]
    fn selection_allows_relay_to_replace_severely_degraded_direct_path() {
        let mut direct = candidate(PathKind::DirectUdp, PathState::Ready);
        direct.path_score = Some(500);
        let mut relay = candidate(PathKind::RelayUdp, PathState::Ready);
        relay.path_score = Some(20);

        assert_eq!(
            select_active_path(
                &PathPolicy::default(),
                Some(PathKind::DirectUdp),
                &[direct, relay],
            ),
            Some(PathKind::RelayUdp)
        );
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
    fn update_peer_active_path_keeps_other_healthy_candidate_as_standby() {
        let mut paths = vec![PeerPathRuntime {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec![],
            active_path: Some(PathKind::DirectUdp),
            candidates: vec![
                candidate(PathKind::DirectUdp, PathState::Ready),
                candidate(PathKind::RelayUdp, PathState::Ready),
                candidate(PathKind::DerpTcpTls443, PathState::Ready),
            ],
        }];

        update_peer_active_path(&mut paths, "node-a", PathKind::RelayUdp);

        assert_eq!(paths[0].candidates[0].state, PathState::Degraded);
        assert_eq!(paths[0].candidates[1].state, PathState::Ready);
        assert_eq!(paths[0].candidates[2].state, PathState::Standby);
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
            PathPolicy::default(),
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

    #[test]
    fn probe_controller_uses_active_and_standby_cadence() {
        let mut active = PathProbeController::new();
        assert!(active.should_probe(0, PathProbeRole::Active));
        active.on_probe_sent(0);
        active.on_inbound(100);
        assert!(!active.should_probe(4_999, PathProbeRole::Active));
        assert!(active.should_probe(5_000, PathProbeRole::Active));

        let mut standby = PathProbeController::new();
        assert!(standby.should_probe(0, PathProbeRole::Standby));
        standby.on_probe_sent(0);
        standby.on_inbound(100);
        assert!(!standby.should_probe(9_999, PathProbeRole::Standby));
        assert!(standby.should_probe(10_000, PathProbeRole::Standby));
    }

    #[test]
    fn probe_controller_accelerates_then_fails_and_recovers() {
        let mut controller = PathProbeController::new();
        assert!(controller.should_probe(0, PathProbeRole::Active));
        controller.on_probe_sent(0);
        controller.on_inbound(100);

        assert!(controller.should_probe(5_100, PathProbeRole::Active));
        controller.on_probe_sent(5_100);
        assert!(controller.should_probe(6_100, PathProbeRole::Active));
        assert_eq!(controller.health(), PathProbeHealth::Suspect);
        controller.on_probe_sent(6_100);
        assert!(controller.should_probe(7_100, PathProbeRole::Active));
        controller.on_probe_sent(7_100);
        assert!(!controller.should_probe(8_100, PathProbeRole::Active));
        assert_eq!(controller.health(), PathProbeHealth::Failed);
        assert!(!controller.usable());

        assert!(!controller.should_probe(18_099, PathProbeRole::Active));
        assert!(controller.should_probe(18_100, PathProbeRole::Active));
        for now in [18_200, 19_200, 20_200] {
            controller.on_inbound(now);
            if now != 20_200 {
                assert!(!controller.usable());
                assert!(controller.should_probe(now + 1_000, PathProbeRole::Active));
                controller.on_probe_sent(now + 1_000);
            }
        }
        assert!(controller.usable());
        assert_eq!(controller.health(), PathProbeHealth::Healthy);
    }

    #[test]
    fn probe_timeout_tracks_rtt_with_bounds() {
        let mut controller = PathProbeController::new();
        controller.on_probe_sent(10_000);
        controller.on_inbound(10_800);
        assert_eq!(controller.probe_timeout_ms(), 2_400);

        controller.on_probe_sent(20_000);
        controller.on_inbound(25_000);
        assert_eq!(controller.probe_timeout_ms(), 3_000);
    }
}
