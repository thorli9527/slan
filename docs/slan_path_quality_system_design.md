# SLAN 信号质量 / 链路质量评估系统设计文档（Path Quality System v1.0）

> 目标：单独设计一套用于评估 LAN、IPv6、UDP P2P、HTTP/3 Relay、TCP/TLS Relay 链路质量的系统，为 PathManager 智能选路、路径切换、Relay 降本、P2P 升级提供可靠依据。

---

## 1. 系统定位

### 1.1 这套系统解决什么问题

打洞模块解决的是：

```text
能不能连
```

Relay 模块解决的是：

```text
连不上时怎么兜底
```

Path Quality System 解决的是：

```text
哪条链路更好
什么时候切换
什么时候降级
什么时候继续保持当前路径
```

---

## 2. 核心原则

```text
1. 不只看 RTT
2. 不只看一次探测结果
3. 不频繁切换
4. Direct 优先，但不是盲目优先
5. Relay 可用，但要尽量降低使用成本
6. 所有路径统一质量模型
```

---

## 3. 系统架构

```text
Path Quality System
├── Quality Probe Engine       探测引擎
├── Metric Collector           指标采集
├── Sliding Window Store       滑动窗口
├── Quality Scorer             质量评分
├── Path Comparator            路径比较器
├── Switch Decision Engine     切换决策
├── Degradation Detector       退化检测
└── Diagnostics Exporter       诊断导出
```

与其他模块关系：

```text
HolePuncher / RelayTransport / DirectTransport
        ↓ 上报 rtt/loss/error
Path Quality System
        ↓ 输出 path_score / decision
PathManager
        ↓ 执行路径切换
Transport Layer
```

---

## 4. 支持的路径类型

```rust
pub enum PathKind {
    LanDirect,
    Ipv6Direct,
    UdpP2p,
    RelayHttp3,
    RelayTls443,
}
```

建议拆分底层传输：

```rust
pub enum TransportKind {
    LanUdp,
    Ipv6Udp,
    UdpP2p,
    QuicHttp3,
    TlsTcp443,
}
```

---

## 5. 质量指标定义

### 5.1 基础指标

```text
RTT             延迟
Loss Rate       丢包率
Jitter          抖动
Success Rate    探测成功率
Throughput      吞吐能力
Stability       稳定性
Cost Level      成本等级
Freshness       数据新鲜度
```

---

### 5.2 PathQuality 数据结构

```rust
use std::time::{Duration, Instant};

#[derive(Debug, Clone)]
pub struct PathQuality {
    pub path_kind: PathKind,
    pub transport_kind: TransportKind,

    pub rtt_ms: Option<u32>,
    pub loss_rate: f32,
    pub jitter_ms: Option<u32>,
    pub success_rate: f32,
    pub throughput_kbps: Option<u32>,

    pub stability_score: f32,
    pub cost_score: f32,
    pub freshness_score: f32,

    pub total_score: f32,
    pub sample_count: u32,
    pub last_updated: Instant,
}
```

字段解释：

```text
rtt_ms              最近窗口内平均 RTT
loss_rate           最近窗口内丢包率，0.0~1.0
jitter_ms           RTT 波动平均值
success_rate        探测成功率
throughput_kbps     可选，吞吐能力
stability_score     稳定性评分
cost_score          成本评分
freshness_score     数据新鲜度评分
total_score         综合评分
sample_count        样本数量
last_updated        最近更新时间
```

---

## 6. 采样数据模型

### 6.1 ProbeSample

每一次探测产生一条样本：

```rust
#[derive(Debug, Clone)]
pub struct ProbeSample {
    pub peer_id: PeerId,
    pub path_kind: PathKind,
    pub transport_kind: TransportKind,

    pub success: bool,
    pub rtt_ms: Option<u32>,
    pub bytes_sent: u64,
    pub bytes_received: u64,
    pub error: Option<ProbeError>,

    pub timestamp: Instant,
}
```

### 6.2 ProbeError

```rust
#[derive(Debug, Clone)]
pub enum ProbeError {
    Timeout,
    NetworkUnreachable,
    PermissionDenied,
    RelayUnavailable,
    P2pMappingExpired,
    PacketLossHigh,
    Unknown(String),
}
```

---

## 7. 滑动窗口设计

### 7.1 为什么需要滑动窗口

不能根据单次 RTT 或一次失败就切换路径，否则会频繁抖动。

推荐使用最近 N 秒或最近 N 条样本：

```text
短窗口：10 秒，用于快速发现故障
长窗口：60 秒，用于稳定评分
```

---

### 7.2 数据结构

```rust
use std::collections::VecDeque;

pub struct QualityWindow {
    short_window: VecDeque<ProbeSample>,
    long_window: VecDeque<ProbeSample>,
    short_duration: Duration,
    long_duration: Duration,
}
```

### 7.3 方法

```rust
impl QualityWindow {
    pub fn push(&mut self, sample: ProbeSample) {
        self.short_window.push_back(sample.clone());
        self.long_window.push_back(sample);
        self.evict_expired();
    }

    pub fn short_stats(&self) -> WindowStats {
        WindowStats::from_samples(&self.short_window)
    }

    pub fn long_stats(&self) -> WindowStats {
        WindowStats::from_samples(&self.long_window)
    }

    fn evict_expired(&mut self) {
        let now = Instant::now();
        while let Some(front) = self.short_window.front() {
            if now.duration_since(front.timestamp) > self.short_duration {
                self.short_window.pop_front();
            } else {
                break;
            }
        }

        while let Some(front) = self.long_window.front() {
            if now.duration_since(front.timestamp) > self.long_duration {
                self.long_window.pop_front();
            } else {
                break;
            }
        }
    }
}
```

---

## 8. WindowStats 统计模型

```rust
#[derive(Debug, Clone)]
pub struct WindowStats {
    pub sample_count: u32,
    pub success_count: u32,
    pub failure_count: u32,
    pub avg_rtt_ms: Option<u32>,
    pub min_rtt_ms: Option<u32>,
    pub max_rtt_ms: Option<u32>,
    pub jitter_ms: Option<u32>,
    pub loss_rate: f32,
    pub success_rate: f32,
}
```

计算规则：

```text
loss_rate = failure_count / sample_count
success_rate = success_count / sample_count
jitter = 平均 |rtt[i] - rtt[i-1]|
```

---

## 9. 质量评分模型

### 9.1 评分目标

输出一个 `0~100` 的分数：

```text
100 = 最优
0   = 不可用
```

---

### 9.2 CostLevel

```rust
#[derive(Debug, Clone, Copy)]
pub enum CostLevel {
    Free,
    Low,
    Medium,
    High,
}
```

映射：

```text
LAN Direct       Free
IPv6 Direct      Free
UDP P2P          Free
HTTP/3 Relay     Medium
TCP/TLS Relay    High
```

---

### 9.3 基础评分函数

```rust
pub struct QualityScorer;

impl QualityScorer {
    pub fn score(stats: &WindowStats, path_kind: PathKind, age: Duration) -> f32 {
        if stats.sample_count == 0 {
            return 0.0;
        }

        let mut score = 100.0;

        if let Some(rtt) = stats.avg_rtt_ms {
            score -= (rtt as f32 * 0.15).min(30.0);
        } else {
            score -= 30.0;
        }

        score -= (stats.loss_rate * 100.0).min(40.0);

        if let Some(jitter) = stats.jitter_ms {
            score -= (jitter as f32 * 0.10).min(15.0);
        }

        score += stats.success_rate * 10.0;

        score += Self::path_bonus(path_kind);
        score -= Self::cost_penalty(path_kind);
        score -= Self::freshness_penalty(age);

        score.clamp(0.0, 100.0)
    }

    fn path_bonus(path_kind: PathKind) -> f32 {
        match path_kind {
            PathKind::LanDirect => 15.0,
            PathKind::Ipv6Direct => 10.0,
            PathKind::UdpP2p => 8.0,
            PathKind::RelayHttp3 => 0.0,
            PathKind::RelayTls443 => -5.0,
        }
    }

    fn cost_penalty(path_kind: PathKind) -> f32 {
        match path_kind {
            PathKind::LanDirect => 0.0,
            PathKind::Ipv6Direct => 0.0,
            PathKind::UdpP2p => 0.0,
            PathKind::RelayHttp3 => 10.0,
            PathKind::RelayTls443 => 18.0,
        }
    }

    fn freshness_penalty(age: Duration) -> f32 {
        let secs = age.as_secs();
        if secs <= 10 {
            0.0
        } else if secs <= 30 {
            5.0
        } else if secs <= 60 {
            15.0
        } else {
            30.0
        }
    }
}
```

---

## 10. 探测策略

### 10.1 探测类型

```text
LAN Direct Probe       UDP ping/pong
IPv6 Direct Probe      IPv6 UDP ping/pong
UDP P2P Probe          P2P punch packet + ping/pong
HTTP/3 Relay Probe     OverlayFrame Ping over QUIC
TLS443 Relay Probe     OverlayFrame Ping over TLS
```

---

### 10.2 探测频率

| 路径状态 | 频率 |
|---|---:|
| 当前主路径 | 2~5 秒一次 |
| Relay 备用路径 | 10~30 秒一次 |
| P2P 后台升级探测 | 30~120 秒一次 |
| 已失败路径 | 指数退避 |
| 网络变化后 | 立即探测 |

---

### 10.3 指数退避

```text
失败 1 次：10 秒后再试
失败 2 次：30 秒后再试
失败 3 次：60 秒后再试
失败 4 次：120 秒后再试
最大：300 秒
```

---

## 11. Quality Probe Engine

### 11.1 职责

```text
- 定期对路径发起 ping/pong
- 记录 RTT / 成功 / 失败
- 生成 ProbeSample
- 写入 QualityStore
- 通知 PathManager 质量变化
```

---

### 11.2 结构设计

```rust
pub struct QualityProbeEngine {
    sample_tx: tokio::sync::mpsc::Sender<ProbeSample>,
    transports: TransportRegistry,
    config: QualityProbeConfig,
}

pub struct QualityProbeConfig {
    pub active_interval: Duration,
    pub standby_interval: Duration,
    pub p2p_upgrade_interval: Duration,
    pub probe_timeout: Duration,
}
```

---

### 11.3 核心方法

```rust
impl QualityProbeEngine {
    pub async fn probe_path(
        &self,
        peer_id: PeerId,
        path_kind: PathKind,
    ) -> ProbeSample {
        let started = Instant::now();

        let result = self
            .transports
            .send_ping(peer_id.clone(), path_kind)
            .await;

        match result {
            Ok(()) => ProbeSample {
                peer_id,
                path_kind,
                transport_kind: path_kind.into(),
                success: true,
                rtt_ms: Some(started.elapsed().as_millis() as u32),
                bytes_sent: 0,
                bytes_received: 0,
                error: None,
                timestamp: Instant::now(),
            },
            Err(err) => ProbeSample {
                peer_id,
                path_kind,
                transport_kind: path_kind.into(),
                success: false,
                rtt_ms: None,
                bytes_sent: 0,
                bytes_received: 0,
                error: Some(err.into()),
                timestamp: Instant::now(),
            },
        }
    }
}
```

---

## 12. Quality Store

### 12.1 存储结构

```rust
use std::collections::HashMap;

pub struct QualityStore {
    windows: HashMap<(PeerId, PathKind), QualityWindow>,
    latest: HashMap<(PeerId, PathKind), PathQuality>,
}
```

### 12.2 方法

```rust
impl QualityStore {
    pub fn record_sample(&mut self, sample: ProbeSample) {
        let key = (sample.peer_id.clone(), sample.path_kind);
        let window = self.windows.entry(key.clone()).or_insert_with(QualityWindow::default);
        window.push(sample);

        let stats = window.long_stats();
        let age = Duration::from_secs(0);
        let score = QualityScorer::score(&stats, key.1, age);

        self.latest.insert(key.clone(), PathQuality::from_stats(key.1, stats, score));
    }

    pub fn get_quality(&self, peer_id: &PeerId, path_kind: PathKind) -> Option<&PathQuality> {
        self.latest.get(&(peer_id.clone(), path_kind))
    }

    pub fn list_peer_qualities(&self, peer_id: &PeerId) -> Vec<PathQuality> {
        self.latest
            .iter()
            .filter(|((p, _), _)| p == peer_id)
            .map(|(_, q)| q.clone())
            .collect()
    }
}
```

---

## 13. 切换决策系统

### 13.1 为什么不能只选最高分

如果只要新路径分数高一点就切换，会导致路径频繁抖动。

必须加入：

```text
- 分数差阈值
- 连续稳定次数
- 切换冷却时间
- 当前路径保护
- Relay 成本惩罚
```

---

### 13.2 Decision 结果

```rust
pub enum PathDecision {
    KeepCurrent,
    SwitchTo {
        path_kind: PathKind,
        reason: SwitchReason,
    },
    DegradeToRelay {
        reason: SwitchReason,
    },
    MarkOffline,
}

pub enum SwitchReason {
    BetterQuality,
    CurrentPathDegraded,
    CurrentPathFailed,
    RelayCostOptimization,
    NetworkChanged,
}
```

---

### 13.3 SwitchDecisionEngine

```rust
pub struct SwitchDecisionEngine {
    pub min_score_delta: f32,
    pub min_stable_samples: u32,
    pub switch_cooldown: Duration,
}
```

### 13.4 核心逻辑

```rust
impl SwitchDecisionEngine {
    pub fn decide(
        &self,
        current: Option<&PathQuality>,
        candidates: &[PathQuality],
        last_switch_at: Option<Instant>,
    ) -> PathDecision {
        if self.in_cooldown(last_switch_at) {
            return PathDecision::KeepCurrent;
        }

        let best = candidates
            .iter()
            .filter(|q| q.success_rate >= 0.7)
            .filter(|q| q.sample_count >= self.min_stable_samples)
            .max_by(|a, b| a.total_score.partial_cmp(&b.total_score).unwrap())
            .cloned();

        let Some(best) = best else {
            return PathDecision::KeepCurrent;
        };

        match current {
            None => PathDecision::SwitchTo {
                path_kind: best.path_kind,
                reason: SwitchReason::BetterQuality,
            },
            Some(cur) => {
                if cur.success_rate < 0.3 {
                    return PathDecision::DegradeToRelay {
                        reason: SwitchReason::CurrentPathFailed,
                    };
                }

                if best.total_score > cur.total_score + self.min_score_delta {
                    PathDecision::SwitchTo {
                        path_kind: best.path_kind,
                        reason: SwitchReason::BetterQuality,
                    }
                } else {
                    PathDecision::KeepCurrent
                }
            }
        }
    }

    fn in_cooldown(&self, last_switch_at: Option<Instant>) -> bool {
        match last_switch_at {
            Some(t) => t.elapsed() < self.switch_cooldown,
            None => false,
        }
    }
}
```

---

## 14. 路径退化检测

### 14.1 退化条件

```text
连续 3 次探测失败
最近 10 秒 loss_rate > 30%
RTT 突增 3 倍
jitter 持续高于 100ms
当前路径无样本超过 30 秒
```

---

### 14.2 DegradationDetector

```rust
pub struct DegradationDetector;

impl DegradationDetector {
    pub fn is_degraded(short: &WindowStats, long: &WindowStats) -> bool {
        if short.sample_count >= 3 && short.success_rate < 0.3 {
            return true;
        }

        if short.loss_rate > 0.3 {
            return true;
        }

        if let (Some(short_rtt), Some(long_rtt)) = (short.avg_rtt_ms, long.avg_rtt_ms) {
            if short_rtt > long_rtt * 3 {
                return true;
            }
        }

        if let Some(jitter) = short.jitter_ms {
            if jitter > 100 {
                return true;
            }
        }

        false
    }
}
```

---

## 15. PathManager 集成方式

### 15.1 事件流

```text
Transport ping/pong
        ↓
ProbeSample
        ↓
QualityStore
        ↓
PathQuality updated
        ↓
SwitchDecisionEngine
        ↓
PathManager.switch_path()
```

---

### 15.2 PathManager 接入伪代码

```rust
impl PathManager {
    pub async fn on_quality_updated(&mut self, peer_id: PeerId) {
        let current = self.current_path_quality(&peer_id);
        let candidates = self.quality_store.list_peer_qualities(&peer_id);

        let decision = self.switch_engine.decide(
            current.as_ref(),
            &candidates,
            self.last_switch_at(&peer_id),
        );

        match decision {
            PathDecision::KeepCurrent => {}
            PathDecision::SwitchTo { path_kind, reason } => {
                self.switch_path(peer_id, path_kind, reason).await;
            }
            PathDecision::DegradeToRelay { reason } => {
                self.switch_to_best_relay(peer_id, reason).await;
            }
            PathDecision::MarkOffline => {
                self.mark_peer_offline(peer_id).await;
            }
        }
    }
}
```

---

## 16. 当前路径保护策略

如果当前路径还能用，不要轻易切换。

推荐规则：

```text
新路径分数至少高 15 分才切
新路径连续 3 次探测成功才切
切换后 30 秒内不再切换
Relay -> P2P 可以更积极
P2P -> Relay 必须快速
```

区别对待：

```text
P2P -> Relay：故障降级，优先保证可用
Relay -> P2P：成本优化，要求更稳定
HTTP3 Relay -> TLS Relay：UDP 失效时立即降级
TLS Relay -> HTTP3 Relay：谨慎升级
```

---

## 17. 不同路径的策略差异

### 17.1 LAN Direct

```text
优点：最快，零成本
策略：一旦可用，强优先
问题：网络变化时容易消失
```

### 17.2 IPv6 Direct

```text
优点：直连，零成本
策略：优先级高于 UDP P2P
问题：某些 IPv6 网络质量不稳定
```

### 17.3 UDP P2P

```text
优点：零成本，通常低延迟
策略：Relay 使用期间持续探测，成功后切换
问题：NAT 映射可能失效，需要 keepalive
```

### 17.4 HTTP/3 Relay

```text
优点：比 TCP Relay 低延迟
策略：UDP 可用但 P2P 失败时优先
问题：UDP 被封时不可用
```

### 17.5 TCP/TLS Relay

```text
优点：兼容性最好
策略：最终兜底
问题：成本高、延迟高
```

---

## 18. 诊断接口设计

### 18.1 本地接口

```http
GET /local/path-quality
GET /local/path-quality/{peer_id}
GET /local/path-decisions
GET /local/probe-samples/{peer_id}
```

### 18.2 返回示例

```json
{
  "peer_id": "peer_b",
  "current_path": "udp_p2p",
  "paths": [
    {
      "path_kind": "udp_p2p",
      "score": 88.5,
      "rtt_ms": 32,
      "loss_rate": 0.01,
      "jitter_ms": 4,
      "success_rate": 0.98,
      "sample_count": 25,
      "last_updated_ms_ago": 1200
    },
    {
      "path_kind": "relay_http3",
      "score": 72.0,
      "rtt_ms": 58,
      "loss_rate": 0.0,
      "jitter_ms": 8,
      "success_rate": 1.0,
      "sample_count": 20,
      "last_updated_ms_ago": 1800
    }
  ],
  "last_decision": {
    "action": "keep_current",
    "reason": "current_path_good_enough"
  }
}
```

---

## 19. Metrics 设计

必须记录：

```text
path_quality_score
path_rtt_ms
path_loss_rate
path_jitter_ms
path_success_rate
path_switch_total
path_switch_reason_total
path_degraded_total
path_probe_timeout_total
path_current_kind
relay_to_p2p_upgrade_total
p2p_to_relay_degrade_total
```

按标签维度：

```text
peer_id
path_kind
transport_kind
relay_region
nat_level
```

---

## 20. 推荐默认参数

```text
active_path_probe_interval      3s
standby_path_probe_interval     30s
p2p_upgrade_probe_interval      60s
probe_timeout                   800ms
short_window                    10s
long_window                     60s
min_score_delta                 15
min_stable_samples              3
switch_cooldown                 30s
high_loss_threshold             30%
high_jitter_threshold           100ms
```

---

## 21. MVP 落地顺序

第一版不要做太复杂，建议顺序：

```text
1. ProbeSample
2. QualityWindow
3. WindowStats
4. QualityScorer
5. QualityStore
6. QualityProbeEngine
7. SwitchDecisionEngine
8. PathManager 接入
9. 本地诊断接口
```

第一版只做：

```text
RTT
loss_rate
success_rate
score
switch cooldown
```

第二版再做：

```text
jitter
throughput
stability_score
cost_score
freshness_score
```

---

## 22. 最终总结

```text
Path Quality System 是 Smart Path 的“大脑评分系统”。

HolePuncher 负责发现 P2P 能不能通；
Relay 负责兜底；
Path Quality System 负责判断哪条路径更值得用；
PathManager 负责最终切换。
```

推荐落地原则：

```text
先采样
再评分
再决策
最后切换
```


---

## 44. 多 ICE Server 管理方案（不自动注册版本）

本节按当前产品策略调整：

```text
服务端具备多 ICE Server 管理能力
但 ICE Server 不需要自动注册
由管理员在后台或配置文件中维护
客户端拉取 ICE Server 列表
客户端周期 Probe，并在 RelayConnected 状态下常试打洞
```

---

## 44.1 总体架构

```text
Admin / Config
    ↓
Control Plane ICE Server Registry
    ↓
Client GET /ice-servers
    ↓
Client 多 ICE Probe
    ↓
Client 上报 Candidates / NAT Profile
    ↓
Control Plane 生成 PunchPlan
    ↓
Client 常试打洞
    ↓
P2P 成功则切换，失败继续 Relay
```

---

## 44.2 服务端职责边界

### 服务端负责

```text
1. 管理 ICE Server 列表
2. 标记 ICE Server 状态：enabled / disabled / degraded / maintenance
3. 按区域、优先级、权重下发 ICE Server
4. 接收客户端 candidate 上报
5. 生成 PunchPlan
6. 接收打洞结果上报
7. 统计 ICE Server 成功率和质量
```

### 服务端不负责

```text
1. 不要求 ICE Server 自动注册
2. 不替客户端打洞
3. 不保存业务流量
4. 不参与 P2P 数据面
```

---

## 44.3 服务端 ICE Server 表设计

```sql
CREATE TABLE ice_servers (
    id BIGSERIAL PRIMARY KEY,
    server_id VARCHAR(128) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,

    provider VARCHAR(64) NOT NULL,
    region VARCHAR(64) NOT NULL,
    country VARCHAR(64) DEFAULT 'CN',

    public_ip VARCHAR(64) NOT NULL,
    udp_addr VARCHAR(128) NOT NULL,
    stun_port INT DEFAULT 3478,

    priority INT DEFAULT 100,
    weight INT DEFAULT 100,

    status VARCHAR(32) DEFAULT 'enabled',
    features JSONB DEFAULT '[]',

    remark TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

状态含义：

```text
enabled      正常下发
disabled     不下发
degraded     降权下发
maintenance  维护中，不下发
```

features 示例：

```json
[
  "udp_probe",
  "stun",
  "nat_detect"
]
```

---

## 44.4 管理端接口

新增 ICE Server：

```http
POST /api/v1/admin/ice-servers
```

```json
{
  "server_id": "ice-aliyun-hz-01",
  "name": "阿里云杭州 ICE 01",
  "provider": "aliyun",
  "region": "cn-east",
  "country": "CN",
  "public_ip": "1.2.3.4",
  "udp_addr": "1.2.3.4:3478",
  "priority": 10,
  "weight": 100,
  "features": ["udp_probe", "stun", "nat_detect"]
}
```

修改 ICE Server：

```http
PUT /api/v1/admin/ice-servers/{server_id}
```

修改状态：

```http
PATCH /api/v1/admin/ice-servers/{server_id}/status
```

```json
{
  "status": "disabled"
}
```

查询列表：

```http
GET /api/v1/admin/ice-servers
```

---

## 44.5 客户端拉取 ICE Server 列表

客户端启动、网络变化、定时刷新时调用：

```http
GET /api/v1/client/ice-servers?region=cn-east&limit=5
```

返回：

```json
{
  "ice_servers": [
    {
      "server_id": "ice-aliyun-hz-01",
      "provider": "aliyun",
      "region": "cn-east",
      "udp_addr": "1.2.3.4:3478",
      "priority": 10,
      "weight": 100,
      "features": ["udp_probe", "stun"]
    },
    {
      "server_id": "ice-tencent-gz-01",
      "provider": "tencent",
      "region": "cn-south",
      "udp_addr": "5.6.7.8:3478",
      "priority": 20,
      "weight": 80,
      "features": ["udp_probe", "stun"]
    }
  ],
  "refresh_after_sec": 300,
  "probe_interval_sec": 60
}
```

客户端缓存策略：

```text
正常缓存 5 分钟
网络变化立即刷新
Control Plane 不可用时使用旧列表
连续失败时指数退避
```

---

## 44.6 ICE Server 下发策略

服务端下发时按以下规则：

```text
1. status = enabled 的节点优先
2. 同 region 优先
3. priority 越小越优先
4. 同优先级按 weight 分配
5. degraded 节点可以少量下发
6. disabled / maintenance 不下发
7. 每次返回 3~7 个节点
8. 尽量跨厂商：阿里云 + 腾讯云 + 华为云
```

推荐下发组合：

```text
本区域 2 个
邻近区域 2 个
跨厂商 1~2 个
香港/海外备用 1 个
```

---

## 44.7 客户端周期 Probe

客户端不是只拉一次，而是周期执行：

```text
客户端启动：立即 Probe
网络变化：立即 Probe
每 30~60 秒：轻量 Probe
连接 peer 前：立即 Probe
RelayConnected 状态：后台低频 Probe + 打洞
```

推荐频率：

```text
正常在线：60 秒
网络变化：立即
准备连接 peer：立即
RelayConnected 后台 P2P 重试：30~120 秒
移动端省电模式：120~300 秒
```

---

## 44.8 客户端 Probe 流程

```text
1. 拉取 ICE Server 列表
2. 创建 2~4 个 UDP socket
3. 对每个 ICE Server 并发发送 Probe
4. 收集 observed_addr
5. 生成 srflx candidate
6. 判断 NAT 类型
7. 上报 candidates 到 Control Plane
```

自研 Probe 请求：

```json
{
  "magic": "SLAN_ICE_PROBE",
  "version": 1,
  "peer_id": "peer-a",
  "nonce": 123456,
  "timestamp_ms": 1710000000000
}
```

自研 Probe 响应：

```json
{
  "magic": "SLAN_ICE_PROBE_RESP",
  "version": 1,
  "server_id": "ice-aliyun-hz-01",
  "observed_addr": "8.8.8.8:62001",
  "nonce": 123456,
  "timestamp_ms": 1710000000030
}
```

---

## 44.9 客户端 Candidate 上报

```http
POST /api/v1/peers/{peer_id}/candidates
```

```json
{
  "peer_id": "peer-a",
  "udp_available": true,
  "nat_level": "hard",
  "mapping_stable": false,
  "candidates": [
    {
      "candidate_id": "cand-host-01",
      "candidate_type": "host",
      "addr": "192.168.1.10:51000",
      "priority": 10
    },
    {
      "candidate_id": "cand-srflx-01",
      "candidate_type": "srflx",
      "addr": "8.8.8.8:62001",
      "source_server_id": "ice-aliyun-hz-01",
      "priority": 30
    },
    {
      "candidate_id": "cand-srflx-02",
      "candidate_type": "srflx",
      "addr": "8.8.8.8:62005",
      "source_server_id": "ice-tencent-gz-01",
      "priority": 35
    }
  ]
}
```

---

## 44.10 服务端 Candidate 存储

```sql
CREATE TABLE peer_candidates (
    id BIGSERIAL PRIMARY KEY,
    peer_id VARCHAR(128) NOT NULL,
    candidate_id VARCHAR(128) NOT NULL,

    candidate_type VARCHAR(32) NOT NULL,
    addr VARCHAR(128) NOT NULL,
    source_server_id VARCHAR(128),

    priority INT DEFAULT 100,
    udp_available BOOLEAN DEFAULT TRUE,
    nat_level VARCHAR(32),
    mapping_stable BOOLEAN,

    last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE(peer_id, candidate_id)
);
```

建议 candidate TTL：

```text
30~120 秒
```

过期 candidate 不参与 PunchPlan。

---

## 44.11 PunchPlan 生成

当 A 要连接 B：

```http
POST /api/v1/punch-plans
```

请求：

```json
{
  "src_peer_id": "peer-a",
  "dst_peer_id": "peer-b"
}
```

返回：

```json
{
  "session_id": "p2p-session-xxx",
  "peer_a": "peer-a",
  "peer_b": "peer-b",
  "a_candidates": [],
  "b_candidates": [],
  "start_after_ms": 300,
  "timeout_ms": 3000,
  "retry_policy": {
    "max_rounds": 3,
    "burst_ms": [0, 20, 40, 100, 120, 300]
  },
  "fallback_relay": {
    "relay_id": "relay-hz-01",
    "http3_addr": "relay-hz.slan.com:443",
    "tls_addr": "relay-hz.slan.com:443"
  }
}
```

---

## 44.12 客户端常试打洞策略

触发条件：

```text
1. peer 首次连接
2. 当前路径是 RelayConnected
3. Relay 质量下降
4. 网络变化
5. 新 candidate 产生
6. 定时后台重试
```

状态策略：

```text
RelayConnected:
    每 30~120 秒尝试一次 P2P

UdpP2pConnected:
    每 15~30 秒 keepalive
    质量下降时准备 Relay fallback

Offline / Failed:
    指数退避重试
```

伪代码：

```rust
async fn background_p2p_upgrade_loop(peer_id: PeerId) {
    loop {
        sleep(select_retry_interval(peer_id)).await;

        if !path_manager.is_relay_connected(peer_id) {
            continue;
        }

        let plan = control_client.create_punch_plan(peer_id).await?;
        let result = hole_puncher.start_punch(plan).await;

        if let Ok(path) = result {
            path_manager.verify_and_switch_to_p2p(path).await;
        }
    }
}
```

---

## 44.13 打洞结果上报

成功/失败都要上报：

```http
POST /api/v1/punch-sessions/{session_id}/result
```

成功：

```json
{
  "session_id": "p2p-session-xxx",
  "reporter_peer_id": "peer-a",
  "remote_peer_id": "peer-b",
  "success": true,
  "path_kind": "udp_p2p",
  "selected_pair": {
    "local_candidate_id": "cand-a-01",
    "remote_candidate_id": "cand-b-02",
    "local_srflx_addr": "8.8.8.8:62001",
    "remote_addr_seen": "9.9.9.9:53002"
  },
  "quality": {
    "rtt_ms": 38,
    "loss_rate": 0.01,
    "jitter_ms": 4
  }
}
```

失败：

```json
{
  "session_id": "p2p-session-xxx",
  "reporter_peer_id": "peer-a",
  "remote_peer_id": "peer-b",
  "success": false,
  "failure": {
    "reason": "timeout",
    "tried_pairs": 16,
    "timeout_ms": 3000
  },
  "fallback": {
    "current_path": "relay_tls443",
    "relay_id": "relay-hz-01"
  }
}
```

---

## 44.14 统计与优化

服务端基于结果统计：

```text
1. 每个 ICE Server 产生 candidate 的成功率
2. 每个 region 的 P2P 成功率
3. 每种 NAT level 的成功率
4. candidate pair 命中率
5. Relay -> P2P 升级成功率
6. 哪些 ICE Server 下发后效果差
```

```sql
CREATE TABLE ice_server_stats (
    id BIGSERIAL PRIMARY KEY,
    server_id VARCHAR(128) NOT NULL,
    region VARCHAR(64),

    probe_count BIGINT DEFAULT 0,
    candidate_count BIGINT DEFAULT 0,
    p2p_success_count BIGINT DEFAULT 0,
    p2p_failure_count BIGINT DEFAULT 0,

    avg_probe_rtt_ms INT,
    success_rate DOUBLE PRECISION,

    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

---

## 44.15 最小 MVP 落地顺序

```text
1. 管理端维护 ice_servers 表
2. 客户端 GET /client/ice-servers
3. 客户端多 ICE Server Probe
4. 客户端上报 candidates
5. 服务端保存 peer_candidates
6. 服务端生成 PunchPlan
7. 客户端按 PunchPlan 尝试打洞
8. RelayConnected 状态下定时常试 P2P
9. 上报打洞结果
10. 服务端统计 ICE Server 效果
```

---

## 45. 最终一句话

```text
多 ICE Server 不需要自动注册也可以做得很强：
服务端管列表，客户端拉列表，客户端持续 Probe 和常试打洞，服务端根据结果持续优化下发策略。
```
