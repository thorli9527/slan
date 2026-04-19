#[derive(Debug, Clone)]
pub struct DaemonConfig {
    pub udp_bind: String,
    pub relay_url_prefix: Option<String>,
    pub ticket_signing_secret: Option<String>,
}

impl Default for DaemonConfig {
    fn default() -> Self {
        Self {
            udp_bind: "0.0.0.0:9000".into(),
            relay_url_prefix: Some("udp://".into()),
            ticket_signing_secret: None,
        }
    }
}
