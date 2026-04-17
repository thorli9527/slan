use slan_app_core::{
    BootstrapConfig, ConnectionState, Device, Network, Node, RelayTicket, Session,
};

pub trait AppCoreFacade: Send + Sync {
    fn register(&self, email: String, password: String) -> Result<Session, String>;
    fn login(&self, email: String, password: String) -> Result<Session, String>;
    fn register_device(
        &self,
        name: String,
        platform: String,
        machine_id: String,
        public_key: String,
    ) -> Result<Device, String>;
    fn register_node(
        &self,
        device_id: String,
        node_id: String,
        node_public_key: String,
        capabilities: Vec<String>,
    ) -> Result<Node, String>;
    fn list_networks(&self) -> Result<Vec<Network>, String>;
    fn create_network(&self, name: String, cidr: String) -> Result<Network, String>;
    fn bootstrap(&self, node_id: String, network_id: String) -> Result<BootstrapConfig, String>;
    fn issue_relay_ticket(
        &self,
        network_id: String,
        src_node_id: String,
        dst_node_id: String,
        reason: String,
    ) -> Result<RelayTicket, String>;
    fn connect(&self, network_id: String, peer_node_id: String) -> Result<ConnectionState, String>;
    fn disconnect(&self) -> Result<(), String>;
}
