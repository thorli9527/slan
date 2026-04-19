use std::collections::HashMap;
use std::sync::Mutex;

use slan_app_core::{WireGuardInterfaceConfig, WireGuardPeerConfig, WireGuardRuntimeStats};

use crate::TunnelConfig;

/// 平台 tunnel backend 能力。
///
/// 这一层是后续对接 WireGuardKit、Linux kernel WireGuard、
/// Windows embeddable service、Android tunnel SDK 的统一落点。
pub trait TunnelBackend: Send + Sync {
    /// 应用本地 WireGuard 接口配置。
    fn apply_interface_config(
        &self,
        _interface: &WireGuardInterfaceConfig,
    ) -> Result<(), String> {
        Ok(())
    }

    /// 应用指定对端虚拟 IP 对应的 WireGuard peer 配置。
    fn apply_peer_config(
        &self,
        peer_virtual_ip: &str,
        peer: &WireGuardPeerConfig,
    ) -> Result<(), String>;

    /// 移除指定对端虚拟 IP 对应的 WireGuard peer 配置。
    fn remove_peer(&self, peer_virtual_ip: &str) -> Result<(), String>;

    /// 启动 tunnel 接口。
    fn bring_up(&self) -> Result<(), String> {
        Ok(())
    }

    /// 关闭 tunnel 接口。
    fn bring_down(&self) -> Result<(), String> {
        Ok(())
    }

    /// 查询指定 peer 的运行时状态。
    fn runtime_stats(
        &self,
        _peer_virtual_ip: &str,
    ) -> Result<Option<WireGuardRuntimeStats>, String> {
        Ok(None)
    }

    /// 应用一条到对端的隧道配置。
    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        self.apply_interface_config(&config.wireguard_interface)?;
        self.apply_peer_config(&config.peer_virtual_ip, &config.wireguard_peer)?;
        self.bring_up()
    }

    /// 关闭指定对端虚拟 IP 的隧道配置。
    fn close(&self, peer_virtual_ip: &str) -> Result<(), String> {
        self.remove_peer(peer_virtual_ip)
    }
}

/// 用于测试和当前占位运行时的内存 backend。
#[derive(Default)]
pub struct InMemoryTunnelBackend {
    state: Mutex<HashMap<String, WireGuardPeerConfig>>,
}

impl InMemoryTunnelBackend {
    pub fn established_peer_ips(&self) -> Vec<String> {
        self.state
            .lock()
            .map(|state| state.keys().cloned().collect())
            .unwrap_or_default()
    }
}

impl TunnelBackend for InMemoryTunnelBackend {
    fn apply_peer_config(
        &self,
        peer_virtual_ip: &str,
        peer: &WireGuardPeerConfig,
    ) -> Result<(), String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "tunnel backend state poisoned".to_string())?;
        let entry = state
            .entry(peer_virtual_ip.to_string())
            .or_insert_with(|| peer.clone());
        *entry = peer.clone();
        Ok(())
    }

    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "tunnel backend state poisoned".to_string())?;
        state.insert(config.peer_virtual_ip.clone(), config.wireguard_peer.clone());
        Ok(())
    }

    fn remove_peer(&self, peer_virtual_ip: &str) -> Result<(), String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "tunnel backend state poisoned".to_string())?;
        state.remove(peer_virtual_ip);
        Ok(())
    }
}
