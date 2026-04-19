use slan_app_core::{BootstrapConfig, Device, Network, Node, RelayTicket, Session};

use crate::api::{
    ControllerClient, CreateNetworkRequest, JoinNetworkRequest, LoginRequest,
    RegisterDeviceRequest, RegisterNodeRequest, RegisterRequest, RelayTicketRequest,
};
use crate::dto::{
    AuthResponseDto, BootstrapRequestDto, BootstrapResponseDto, CreateNetworkRequestDto, DeviceDto,
    JoinNetworkRequestDto, ListNetworksResponseDto, LoginRequestDto, NetworkDto,
    NetworkJoinResultDto, NodeDto, RegisterDeviceRequestDto, RegisterNodeRequestDto,
    RegisterRequestDto, RelayTicketDto, RelayTicketRequestDto,
};
use crate::http_runtime::{get_json, post_json};
use crate::transport::JsonHttpTransport;

pub struct HttpControllerClient<T>
where
    T: JsonHttpTransport,
{
    pub base_url: String,
    pub transport: T,
}

impl<T> HttpControllerClient<T>
where
    T: JsonHttpTransport,
{
    pub fn new(base_url: impl Into<String>, transport: T) -> Self {
        Self {
            base_url: base_url.into(),
            transport,
        }
    }

    fn register(&self, req: RegisterRequest) -> Result<Session, String> {
        let dto: AuthResponseDto =
            post_json(&self.transport, &self.base_url, "/auth/register", None, &RegisterRequestDto::from(req))?;
        dto.try_into()
    }

    fn login(&self, req: LoginRequest) -> Result<Session, String> {
        let dto: AuthResponseDto =
            post_json(&self.transport, &self.base_url, "/auth/login", None, &LoginRequestDto::from(req))?;
        dto.try_into()
    }

    fn register_device(
        &self,
        access_token: &str,
        req: RegisterDeviceRequest,
    ) -> Result<Device, String> {
        let dto: DeviceDto = post_json(
            &self.transport,
            &self.base_url,
            "/devices/register",
            Some(access_token),
            &RegisterDeviceRequestDto::from(req),
        )?;
        Ok(dto.into())
    }

    fn register_node(&self, access_token: &str, req: RegisterNodeRequest) -> Result<Node, String> {
        let dto: NodeDto = post_json(
            &self.transport,
            &self.base_url,
            "/nodes/register",
            Some(access_token),
            &RegisterNodeRequestDto::from(req),
        )?;
        Ok(dto.into())
    }

    fn list_networks(&self, access_token: &str) -> Result<Vec<Network>, String> {
        let dto: ListNetworksResponseDto =
            get_json(&self.transport, &self.base_url, "/networks", Some(access_token))?;
        Ok(dto.items.into_iter().map(Into::into).collect())
    }

    fn create_network(
        &self,
        access_token: &str,
        req: CreateNetworkRequest,
    ) -> Result<Network, String> {
        let dto: NetworkDto = post_json(
            &self.transport,
            &self.base_url,
            "/networks",
            Some(access_token),
            &CreateNetworkRequestDto::from(req),
        )?;
        Ok(dto.into())
    }

    fn join_network(&self, access_token: &str, req: JoinNetworkRequest) -> Result<(), String> {
        let _: NetworkJoinResultDto = post_json(
            &self.transport,
            &self.base_url,
            &format!("/networks/{}/join", req.network_id),
            Some(access_token),
            &JoinNetworkRequestDto {
                device_id: req.device_id,
            },
        )?;
        Ok(())
    }

    fn bootstrap(
        &self,
        access_token: &str,
        node_id: &str,
        network_id: &str,
    ) -> Result<BootstrapConfig, String> {
        let dto: BootstrapResponseDto = post_json(
            &self.transport,
            &self.base_url,
            "/bootstrap",
            Some(access_token),
            &BootstrapRequestDto {
                node_id: node_id.to_string(),
                network_id: network_id.to_string(),
            },
        )?;
        dto.try_into()
    }

    fn issue_relay_ticket(
        &self,
        access_token: &str,
        req: RelayTicketRequest,
    ) -> Result<RelayTicket, String> {
        let dto: RelayTicketDto = post_json(
            &self.transport,
            &self.base_url,
            "/relay/tickets",
            Some(access_token),
            &RelayTicketRequestDto::from(req),
        )?;
        Ok(dto.into())
    }
}

impl<T> ControllerClient for HttpControllerClient<T>
where
    T: JsonHttpTransport,
{
    fn register(&self, req: RegisterRequest) -> Result<Session, String> {
        HttpControllerClient::register(self, req)
    }

    fn login(&self, req: LoginRequest) -> Result<Session, String> {
        HttpControllerClient::login(self, req)
    }

    fn register_device(
        &self,
        access_token: &str,
        req: RegisterDeviceRequest,
    ) -> Result<Device, String> {
        HttpControllerClient::register_device(self, access_token, req)
    }

    fn register_node(&self, access_token: &str, req: RegisterNodeRequest) -> Result<Node, String> {
        HttpControllerClient::register_node(self, access_token, req)
    }

    fn list_networks(&self, access_token: &str) -> Result<Vec<Network>, String> {
        HttpControllerClient::list_networks(self, access_token)
    }

    fn create_network(
        &self,
        access_token: &str,
        req: CreateNetworkRequest,
    ) -> Result<Network, String> {
        HttpControllerClient::create_network(self, access_token, req)
    }

    fn join_network(&self, access_token: &str, req: JoinNetworkRequest) -> Result<(), String> {
        HttpControllerClient::join_network(self, access_token, req)
    }

    fn bootstrap(
        &self,
        access_token: &str,
        node_id: &str,
        network_id: &str,
    ) -> Result<BootstrapConfig, String> {
        HttpControllerClient::bootstrap(self, access_token, node_id, network_id)
    }

    fn issue_relay_ticket(
        &self,
        access_token: &str,
        req: RelayTicketRequest,
    ) -> Result<RelayTicket, String> {
        HttpControllerClient::issue_relay_ticket(self, access_token, req)
    }
}
