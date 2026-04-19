use std::sync::Arc;

use slan_app_core::{
    DerpLinkSnapshot, DerpNodeMeta, DerpPoolState, DerpSwitchEvent, ProbeSample, RelayTicket,
    SwitchReason,
};

/// 单个 DERP 连接能力。
pub trait DerpClient: Send + Sync {
    /// 连接到单个 DERP 节点。
    fn connect_node(&self, meta: &DerpNodeMeta) -> Result<(), String>;
    /// 使用 ticket 完成 attach。
    fn attach_ticket(&self, ticket: &RelayTicket) -> Result<(), String>;
    /// 向当前 DERP 连接发送 transport packet。
    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), String>;
    /// 从当前 DERP 连接轮询接收 transport packet。
    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String>;
    /// 发送一次探测并返回采样结果。
    fn probe(&self) -> Result<ProbeSample, String>;
    /// 返回当前连接快照。
    fn snapshot(&self) -> DerpLinkSnapshot;
    /// 关闭当前连接。
    fn close(&self) -> Result<(), String>;
}

/// DERP 连接池能力。
pub trait DerpPool: Send + Sync {
    /// 安装一个集群配置与本次连接的 ticket。
    fn install_cluster(
        &self,
        cluster_id: &str,
        nodes: Vec<DerpNodeMeta>,
        ticket: RelayTicket,
    ) -> Result<(), String>;
    /// 按 fanout 建立热备连接。
    fn warm_up(&self, fanout: usize) -> Result<(), String>;
    /// 当前 active 链路。
    fn active_link(&self) -> Option<DerpLinkSnapshot>;
    /// 只经由 active 链路发送 transport packet。
    fn send_transport_packet_via_active(&self, packet: &[u8]) -> Result<(), String>;
    /// 非阻塞轮询 active 链路上的 transport packet。
    fn poll_transport_packet_via_active(&self) -> Result<Option<Vec<u8>>, String>;
    /// 执行一次健康检查。
    fn tick_health_check(&self) -> Result<(), String>;
    /// 根据最新评分判断是否切换。
    fn maybe_switch(&self) -> Result<Option<DerpSwitchEvent>, String>;
    /// 返回当前池快照。
    fn state(&self) -> DerpPoolState;
    /// 强制切到指定节点。
    fn force_switch(
        &self,
        target_node_id: &str,
        reason: SwitchReason,
    ) -> Result<DerpSwitchEvent, String>;
    /// 关闭整个连接池。
    fn close(&self) -> Result<(), String>;
}

impl<T> DerpPool for Arc<T>
where
    T: DerpPool,
{
    fn install_cluster(
        &self,
        cluster_id: &str,
        nodes: Vec<DerpNodeMeta>,
        ticket: RelayTicket,
    ) -> Result<(), String> {
        self.as_ref().install_cluster(cluster_id, nodes, ticket)
    }

    fn warm_up(&self, fanout: usize) -> Result<(), String> {
        self.as_ref().warm_up(fanout)
    }

    fn active_link(&self) -> Option<DerpLinkSnapshot> {
        self.as_ref().active_link()
    }

    fn send_transport_packet_via_active(&self, packet: &[u8]) -> Result<(), String> {
        self.as_ref().send_transport_packet_via_active(packet)
    }

    fn poll_transport_packet_via_active(&self) -> Result<Option<Vec<u8>>, String> {
        self.as_ref().poll_transport_packet_via_active()
    }

    fn tick_health_check(&self) -> Result<(), String> {
        self.as_ref().tick_health_check()
    }

    fn maybe_switch(&self) -> Result<Option<DerpSwitchEvent>, String> {
        self.as_ref().maybe_switch()
    }

    fn state(&self) -> DerpPoolState {
        self.as_ref().state()
    }

    fn force_switch(
        &self,
        target_node_id: &str,
        reason: SwitchReason,
    ) -> Result<DerpSwitchEvent, String> {
        self.as_ref().force_switch(target_node_id, reason)
    }

    fn close(&self) -> Result<(), String> {
        self.as_ref().close()
    }
}
