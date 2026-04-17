/// 隧道建立所需配置。
pub struct TunnelConfig {
    pub local_virtual_ip: String,
    pub peer_virtual_ip: String,
    pub peer_public_key: String,
}
