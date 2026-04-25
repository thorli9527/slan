//! 控制面客户端抽象。
//!
//! 负责定义 app_core 到 server-biz 的请求模型和能力边界。

mod api;
mod client;
mod dto;
mod http_runtime;
mod transport;

pub use api::{
    ControllerClient, CreateNetworkRequest, DeactivateNetworkRequest, JoinNetworkByKeyRequest,
    JoinNetworkByOwnerEmailRequest, JoinNetworkRequest, LoginRequest, RefreshTokenRequest,
    RegisterDeviceRequest, RegisterNodeRequest, RegisterRequest, RelayTicketRequest,
    SwitchNetworkRequest, UpdateAttachmentRemarkRequest, UpdateNetworkDNSRequest,
};
pub use client::HttpControllerClient;
pub use dto::ErrorResponseDto;
pub use transport::{
    HttpMethod, HttpRequest, HttpResponse, JsonHttpTransport, TcpJsonHttpTransport,
};
