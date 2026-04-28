use std::sync::Arc;

use crate::TunnelConfig;

pub trait TunnelManager: Send + Sync {
    fn establish(&self, config: &TunnelConfig) -> Result<(), String>;
    fn close(&self, peer_virtual_ip: &str) -> Result<(), String>;

    fn start_local_dns(&self, _records: Vec<(String, String)>) -> Result<(), String> {
        Ok(())
    }

    fn stop_local_dns(&self) -> Result<(), String> {
        Ok(())
    }
}

impl<T> TunnelManager for Arc<T>
where
    T: TunnelManager + ?Sized,
{
    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        self.as_ref().establish(config)
    }

    fn close(&self, peer_virtual_ip: &str) -> Result<(), String> {
        self.as_ref().close(peer_virtual_ip)
    }

    fn start_local_dns(&self, records: Vec<(String, String)>) -> Result<(), String> {
        self.as_ref().start_local_dns(records)
    }

    fn stop_local_dns(&self) -> Result<(), String> {
        self.as_ref().stop_local_dns()
    }
}
