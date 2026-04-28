use slan_app_core::{
    BootstrapConfig, Device, Network, NetworkAssignment, NetworkJoinResult, Node, RelayTicket,
    Session,
};

pub struct RegisterRequest {
    pub email: String,
    pub password: String,
}

pub struct LoginRequest {
    pub email: String,
    pub password: String,
    pub device_id: Option<String>,
}

pub struct RefreshTokenRequest {
    pub refresh_token: String,
    pub device_id: Option<String>,
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
    pub cidr: Option<String>,
    pub description: Option<String>,
    pub allocation_start_ip: Option<String>,
    pub allocation_end_ip: Option<String>,
    pub bind_device_id: Option<String>,
}

pub struct UpdateNetworkDNSRequest {
    pub network_id: String,
    pub servers: Vec<String>,
    pub search_domains: Vec<String>,
    pub wildcards: Vec<String>,
}

pub struct JoinNetworkRequest {
    pub network_id: String,
    pub device_id: String,
}

pub struct SwitchNetworkRequest {
    pub network_id: String,
    pub device_id: String,
}

pub struct JoinNetworkByKeyRequest {
    pub join_key: String,
    pub device_id: String,
}

pub struct DeactivateNetworkRequest {
    pub network_id: String,
    pub device_id: String,
}

pub struct DeviceNetworkStateRequest {
    pub device_id: String,
    pub network_id: String,
    pub control_reachable: bool,
    pub network_online: bool,
    pub tunnel_up: bool,
    pub last_probe_ok: bool,
    pub virtual_ip: Option<String>,
    pub reported_at: Option<i64>,
}

pub struct UpdateAttachmentRemarkRequest {
    pub network_id: String,
    pub attachment_id: String,
    pub remark: Option<String>,
}

pub struct RelayTicketRequest {
    pub network_id: String,
    pub src_node_id: String,
    pub dst_node_id: String,
    pub derp_cluster_id: Option<String>,
    pub preferred_derp_node_ids: Vec<String>,
    pub reason: String,
    pub relay_region_id: Option<String>,
}

pub trait ControllerClient: Send + Sync {
    fn register(&self, req: RegisterRequest) -> Result<Session, String>;
    fn login(&self, req: LoginRequest) -> Result<Session, String>;
    fn refresh(&self, req: RefreshTokenRequest) -> Result<Session, String>;
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
    fn update_network_dns(
        &self,
        access_token: &str,
        req: UpdateNetworkDNSRequest,
    ) -> Result<Network, String>;
    fn join_network(
        &self,
        access_token: &str,
        req: JoinNetworkRequest,
    ) -> Result<NetworkJoinResult, String>;
    fn join_network_by_key(
        &self,
        access_token: &str,
        req: JoinNetworkByKeyRequest,
    ) -> Result<NetworkJoinResult, String>;
    fn update_attachment_remark(
        &self,
        access_token: &str,
        req: UpdateAttachmentRemarkRequest,
    ) -> Result<NetworkAssignment, String>;
    fn activate_network(
        &self,
        access_token: &str,
        req: JoinNetworkRequest,
    ) -> Result<NetworkJoinResult, String>;
    fn switch_network(
        &self,
        access_token: &str,
        req: SwitchNetworkRequest,
    ) -> Result<NetworkJoinResult, String> {
        self.activate_network(
            access_token,
            JoinNetworkRequest {
                network_id: req.network_id,
                device_id: req.device_id,
            },
        )
    }
    fn deactivate_network(
        &self,
        access_token: &str,
        req: DeactivateNetworkRequest,
    ) -> Result<(), String>;
    fn set_device_network_state(
        &self,
        access_token: &str,
        req: DeviceNetworkStateRequest,
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
