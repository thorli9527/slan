use slan_app_core::{BootstrapConfig, Device, Network, Node, RelayTicket, Session};

pub struct RegisterRequest {
    pub email: String,
    pub password: String,
}

pub struct LoginRequest {
    pub email: String,
    pub password: String,
}

pub struct RegisterDeviceRequest {
    pub name: String,
    pub platform: String,
    pub machine_id: String,
    pub public_key: String,
}

pub struct RegisterNodeRequest {
    pub device_id: String,
    pub node_id: String,
    pub node_public_key: String,
    pub capabilities: Vec<String>,
}

pub struct CreateNetworkRequest {
    pub name: String,
    pub cidr: String,
}

pub struct JoinNetworkRequest {
    pub network_id: String,
    pub device_id: String,
}

pub struct DeactivateNetworkRequest {
    pub network_id: String,
    pub device_id: String,
}

pub struct RelayTicketRequest {
    pub network_id: String,
    pub src_node_id: String,
    pub dst_node_id: String,
    pub derp_cluster_id: Option<String>,
    pub preferred_derp_node_ids: Vec<String>,
    pub reason: String,
}

pub trait ControllerClient: Send + Sync {
    fn register(&self, req: RegisterRequest) -> Result<Session, String>;
    fn login(&self, req: LoginRequest) -> Result<Session, String>;
    fn register_device(
        &self,
        access_token: &str,
        req: RegisterDeviceRequest,
    ) -> Result<Device, String>;
    fn register_node(&self, access_token: &str, req: RegisterNodeRequest) -> Result<Node, String>;
    fn list_networks(&self, access_token: &str) -> Result<Vec<Network>, String>;
    fn create_network(
        &self,
        access_token: &str,
        req: CreateNetworkRequest,
    ) -> Result<Network, String>;
    fn join_network(&self, access_token: &str, req: JoinNetworkRequest) -> Result<(), String>;
    fn activate_network(
        &self,
        access_token: &str,
        req: JoinNetworkRequest,
    ) -> Result<(), String>;
    fn deactivate_network(
        &self,
        access_token: &str,
        req: DeactivateNetworkRequest,
    ) -> Result<(), String>;
    fn bootstrap(
        &self,
        access_token: &str,
        node_id: &str,
        network_id: &str,
    ) -> Result<BootstrapConfig, String>;
    fn issue_relay_ticket(
        &self,
        access_token: &str,
        req: RelayTicketRequest,
    ) -> Result<RelayTicket, String>;
}
