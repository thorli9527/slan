use serde::{Deserialize, Serialize};
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
use crate::transport::{HttpMethod, HttpRequest, JsonHttpTransport};

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

    fn post_json<Req, Resp>(
        &self,
        path: &str,
        bearer_token: Option<&str>,
        body: &Req,
    ) -> Result<Resp, String>
    where
        Req: Serialize,
        Resp: for<'de> Deserialize<'de>,
    {
        let request = HttpRequest {
            method: HttpMethod::Post,
            path: self.join_path(path),
            bearer_token: bearer_token.map(ToOwned::to_owned),
            body_json: Some(serde_json::to_vec(body).map_err(|err| err.to_string())?),
        };
        let response = self.transport.send(request)?;
        if !(200..300).contains(&response.status) {
            return Err(format!("unexpected http status: {}", response.status));
        }
        serde_json::from_slice(&response.body_json).map_err(|err| err.to_string())
    }

    fn get_json<Resp>(&self, path: &str, bearer_token: Option<&str>) -> Result<Resp, String>
    where
        Resp: for<'de> Deserialize<'de>,
    {
        let request = HttpRequest {
            method: HttpMethod::Get,
            path: self.join_path(path),
            bearer_token: bearer_token.map(ToOwned::to_owned),
            body_json: None,
        };
        let response = self.transport.send(request)?;
        if !(200..300).contains(&response.status) {
            return Err(format!("unexpected http status: {}", response.status));
        }
        serde_json::from_slice(&response.body_json).map_err(|err| err.to_string())
    }

    fn join_path(&self, path: &str) -> String {
        format!("{}{}", self.base_url.trim_end_matches('/'), path)
    }
}

impl<T> ControllerClient for HttpControllerClient<T>
where
    T: JsonHttpTransport,
{
    fn register(&self, req: RegisterRequest) -> Result<Session, String> {
        let dto: AuthResponseDto =
            self.post_json("/auth/register", None, &RegisterRequestDto::from(req))?;
        dto.try_into()
    }

    fn login(&self, req: LoginRequest) -> Result<Session, String> {
        let dto: AuthResponseDto =
            self.post_json("/auth/login", None, &LoginRequestDto::from(req))?;
        dto.try_into()
    }

    fn register_device(
        &self,
        access_token: &str,
        req: RegisterDeviceRequest,
    ) -> Result<Device, String> {
        let dto: DeviceDto = self.post_json(
            "/devices/register",
            Some(access_token),
            &RegisterDeviceRequestDto::from(req),
        )?;
        Ok(dto.into())
    }

    fn register_node(&self, access_token: &str, req: RegisterNodeRequest) -> Result<Node, String> {
        let dto: NodeDto = self.post_json(
            "/nodes/register",
            Some(access_token),
            &RegisterNodeRequestDto::from(req),
        )?;
        Ok(dto.into())
    }

    fn list_networks(&self, access_token: &str) -> Result<Vec<Network>, String> {
        let dto: ListNetworksResponseDto = self.get_json("/networks", Some(access_token))?;
        Ok(dto.items.into_iter().map(Into::into).collect())
    }

    fn create_network(
        &self,
        access_token: &str,
        req: CreateNetworkRequest,
    ) -> Result<Network, String> {
        let dto: NetworkDto = self.post_json(
            "/networks",
            Some(access_token),
            &CreateNetworkRequestDto::from(req),
        )?;
        Ok(dto.into())
    }

    fn join_network(&self, access_token: &str, req: JoinNetworkRequest) -> Result<(), String> {
        let _: NetworkJoinResultDto = self.post_json(
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
        let dto: BootstrapResponseDto = self.post_json(
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
        let dto: RelayTicketDto = self.post_json(
            "/relay/tickets",
            Some(access_token),
            &RelayTicketRequestDto::from(req),
        )?;
        Ok(dto.into())
    }
}
