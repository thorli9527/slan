use std::collections::HashMap;
use std::sync::Mutex;

use crate::macos_mapper::MacosWireGuardKitPeerPlan;
use crate::macos_stats::MacosWireGuardKitRuntime;

/// Rust 侧的 macOS WireGuardKit adapter skeleton。
///
/// 当前只负责保存已经映射好的配置计划，后续可以把这里替换成
/// Swift/FFI 层的真实 WireGuardKit 调用。
#[derive(Default)]
pub struct MacosWireGuardKitAdapter {
    interface: Mutex<Option<MacosWireGuardKitRuntime>>,
    peers: Mutex<HashMap<String, MacosWireGuardKitPeerPlan>>,
}

impl MacosWireGuardKitAdapter {
    pub fn apply_interface(
        &self,
        runtime: MacosWireGuardKitRuntime,
    ) -> Result<(), String> {
        let mut interface = self
            .interface
            .lock()
            .map_err(|_| "macos adapter interface state poisoned".to_string())?;
        *interface = Some(runtime);
        Ok(())
    }

    pub fn apply_peer(
        &self,
        peer_virtual_ip: &str,
        peer: MacosWireGuardKitPeerPlan,
    ) -> Result<(), String> {
        let mut peers = self
            .peers
            .lock()
            .map_err(|_| "macos adapter peer state poisoned".to_string())?;
        peers.insert(peer_virtual_ip.to_string(), peer);
        Ok(())
    }

    pub fn remove_peer(&self, peer_virtual_ip: &str) -> Result<(), String> {
        let mut peers = self
            .peers
            .lock()
            .map_err(|_| "macos adapter peer state poisoned".to_string())?;
        peers.remove(peer_virtual_ip);
        Ok(())
    }

    pub fn planned_peer_ips(&self) -> Vec<String> {
        self.peers
            .lock()
            .map(|state| state.keys().cloned().collect())
            .unwrap_or_default()
    }

    pub fn interface_runtime(&self) -> Option<MacosWireGuardKitRuntime> {
        self.interface.lock().ok().and_then(|runtime| runtime.clone())
    }
}
