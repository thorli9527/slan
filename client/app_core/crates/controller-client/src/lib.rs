//! 控制面客户端抽象。
//!
//! 负责定义 app_core 到 server-biz 的请求模型和能力边界。

mod api;
mod client;
mod dto;
mod transport;

pub use api::{
    ControllerClient, CreateNetworkRequest, JoinNetworkRequest, LoginRequest,
    RegisterDeviceRequest, RegisterNodeRequest, RegisterRequest, RelayTicketRequest,
};
pub use client::HttpControllerClient;
pub use transport::{
    HttpMethod, HttpRequest, HttpResponse, JsonHttpTransport, TcpJsonHttpTransport,
};
