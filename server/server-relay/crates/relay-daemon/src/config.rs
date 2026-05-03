use std::fs;
use std::path::Path;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DaemonConfig {
    pub udp_bind: String,
    pub tcp_bind: Option<String>,
    pub relay_url_prefix: Option<String>,
    pub ticket_signing_secret: Option<String>,
    #[serde(default)]
    pub allow_unsigned_tickets: bool,
    pub mqtt: Option<RelayMqttConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RelayMqttConfig {
    pub enabled: bool,
    pub broker_url: String,
    pub topic_prefix: String,
    pub username_prefix: String,
    pub password_secret: String,
    pub node_id: String,
    pub cluster_id: Option<String>,
    pub country_code: Option<String>,
    pub city_code: Option<String>,
    pub transport: Option<String>,
    pub address: Option<String>,
    #[serde(default)]
    pub nodes: Vec<RelayMqttNodeConfig>,
    pub interval_seconds: Option<u64>,
    pub credential_ttl_seconds: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RelayMqttNodeConfig {
    pub node_id: String,
    pub cluster_id: Option<String>,
    pub country_code: Option<String>,
    pub city_code: Option<String>,
    pub transport: String,
    pub address: String,
}

impl Default for RelayMqttConfig {
    fn default() -> Self {
        Self {
            enabled: true,
            broker_url: "mqtt://127.0.0.1:1883".to_string(),
            topic_prefix: "slan/devices".to_string(),
            username_prefix: "slan".to_string(),
            password_secret: "dev-mqtt-secret".to_string(),
            node_id: String::new(),
            cluster_id: None,
            country_code: None,
            city_code: None,
            transport: Some("udp".to_string()),
            address: None,
            nodes: Vec::new(),
            interval_seconds: Some(30),
            credential_ttl_seconds: Some(86_400),
        }
    }
}

impl Default for DaemonConfig {
    fn default() -> Self {
        Self {
            udp_bind: "0.0.0.0:9000".into(),
            tcp_bind: None,
            relay_url_prefix: Some("udp://".into()),
            ticket_signing_secret: None,
            allow_unsigned_tickets: false,
            mqtt: None,
        }
    }
}

impl DaemonConfig {
    /// from_file 从 JSON 配置文件加载 daemon 配置。
    pub fn from_file(path: impl AsRef<Path>) -> Result<Self, String> {
        let path = path.as_ref();
        let payload = fs::read_to_string(path)
            .map_err(|err| format!("failed to read config {}: {err}", path.display()))?;
        serde_json::from_str(&payload)
            .map_err(|err| format!("failed to parse config {}: {err}", path.display()))
    }

    pub fn apply_env_overrides(&mut self) -> Result<(), String> {
        self.apply_env_pairs(std::env::vars())?;
        self.validate()
    }

    pub fn validate(&self) -> Result<(), String> {
        let Some(mqtt) = self.mqtt.as_ref().filter(|value| value.enabled) else {
            return Ok(());
        };
        let nodes = mqtt.effective_nodes();
        if nodes.is_empty() {
            return Err("relay mqtt node_id or nodes are required".to_string());
        }
        for node in nodes {
            validate_relay_transport_listener(self, node.transport.as_str())?;
        }
        Ok(())
    }

    fn apply_env_pairs<I, K, V>(&mut self, vars: I) -> Result<(), String>
    where
        I: IntoIterator<Item = (K, V)>,
        K: AsRef<str>,
        V: AsRef<str>,
    {
        for (key, value) in vars {
            let key = key.as_ref();
            let value = value.as_ref().trim();
            if value.is_empty() {
                continue;
            }
            match key {
                "SLAN_RELAY_UDP_BIND" => self.udp_bind = value.to_string(),
                "SLAN_RELAY_TCP_BIND" => self.tcp_bind = Some(value.to_string()),
                "SLAN_RELAY_URL_PREFIX" => self.relay_url_prefix = Some(value.to_string()),
                "SLAN_RELAY_NO_URL_PREFIX" => {
                    if parse_env_bool(key, value)? {
                        self.relay_url_prefix = None;
                    }
                }
                "SLAN_RELAY_TICKET_SIGNING_SECRET" => {
                    self.ticket_signing_secret = Some(value.to_string())
                }
                "SLAN_RELAY_ALLOW_UNSIGNED_TICKETS" => {
                    self.allow_unsigned_tickets = parse_env_bool(key, value)?;
                }
                "SLAN_RELAY_MQTT_ENABLED" => {
                    self.mqtt_mut().enabled = parse_env_bool(key, value)?;
                }
                "SLAN_RELAY_MQTT_BROKER_URL" => {
                    self.mqtt_mut().broker_url = value.to_string();
                }
                "SLAN_RELAY_MQTT_TOPIC_PREFIX" => {
                    self.mqtt_mut().topic_prefix = value.to_string();
                }
                "SLAN_RELAY_MQTT_USERNAME_PREFIX" => {
                    self.mqtt_mut().username_prefix = value.to_string();
                }
                "SLAN_RELAY_MQTT_PASSWORD_SECRET" => {
                    self.mqtt_mut().password_secret = value.to_string();
                }
                "SLAN_RELAY_NODE_ID" => self.mqtt_mut().node_id = value.to_string(),
                "SLAN_RELAY_CLUSTER_ID" => self.mqtt_mut().cluster_id = Some(value.to_string()),
                "SLAN_RELAY_COUNTRY_CODE" => {
                    self.mqtt_mut().country_code = Some(value.to_ascii_uppercase())
                }
                "SLAN_RELAY_CITY_CODE" => self.mqtt_mut().city_code = Some(value.to_string()),
                "SLAN_RELAY_TRANSPORT" => self.mqtt_mut().transport = Some(value.to_string()),
                "SLAN_RELAY_ADDRESS" => self.mqtt_mut().address = Some(value.to_string()),
                "SLAN_RELAY_MQTT_INTERVAL_SECONDS" => {
                    self.mqtt_mut().interval_seconds = Some(parse_env_u64(key, value)?);
                }
                "SLAN_RELAY_MQTT_CREDENTIAL_TTL_SECONDS" => {
                    self.mqtt_mut().credential_ttl_seconds = Some(parse_env_u64(key, value)?);
                }
                _ => {}
            }
        }
        Ok(())
    }

    fn mqtt_mut(&mut self) -> &mut RelayMqttConfig {
        self.mqtt.get_or_insert_with(RelayMqttConfig::default)
    }
}

impl RelayMqttConfig {
    pub fn effective_nodes(&self) -> Vec<RelayMqttNodeConfig> {
        if !self.nodes.is_empty() {
            return self
                .nodes
                .iter()
                .filter(|node| !node.node_id.trim().is_empty())
                .map(|node| RelayMqttNodeConfig {
                    node_id: node.node_id.trim().to_string(),
                    cluster_id: node.cluster_id.clone().or_else(|| self.cluster_id.clone()),
                    country_code: node
                        .country_code
                        .clone()
                        .or_else(|| self.country_code.clone())
                        .map(|value| value.to_ascii_uppercase()),
                    city_code: node.city_code.clone().or_else(|| self.city_code.clone()),
                    transport: normalize_relay_transport(&node.transport)
                        .unwrap_or_else(|_| node.transport.trim().to_ascii_lowercase()),
                    address: node.address.trim().to_string(),
                })
                .collect();
        }
        let node_id = self.node_id.trim();
        if node_id.is_empty() {
            return Vec::new();
        }
        vec![RelayMqttNodeConfig {
            node_id: node_id.to_string(),
            cluster_id: self.cluster_id.clone(),
            country_code: self
                .country_code
                .clone()
                .map(|value| value.to_ascii_uppercase()),
            city_code: self.city_code.clone(),
            transport: normalize_relay_transport(self.transport.as_deref().unwrap_or("udp"))
                .unwrap_or_else(|_| {
                    self.transport
                        .as_deref()
                        .unwrap_or("udp")
                        .to_ascii_lowercase()
                }),
            address: self.address.clone().unwrap_or_default(),
        }]
    }
}

fn validate_relay_transport_listener(config: &DaemonConfig, transport: &str) -> Result<(), String> {
    let transport = normalize_relay_transport(transport)?;
    match transport.as_str() {
        "udp" => {
            if config.udp_bind.trim().is_empty() {
                return Err("relay udp transport requires udp_bind".to_string());
            }
        }
        "tcp" => {
            if config
                .tcp_bind
                .as_deref()
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .is_none()
            {
                return Err("relay tcp transport requires tcp_bind".to_string());
            }
        }
        "tls" | "http3" => {
            return Err(format!(
                "relay {transport} transport is declared in control-plane but this relay-daemon build does not implement a {transport} listener yet"
            ));
        }
        _ => unreachable!(),
    }
    Ok(())
}

fn normalize_relay_transport(value: &str) -> Result<String, String> {
    match value.trim().to_ascii_lowercase().as_str() {
        "udp" => Ok("udp".to_string()),
        "tcp" => Ok("tcp".to_string()),
        "tls" => Ok("tls".to_string()),
        "http3" => Ok("http3".to_string()),
        other => Err(format!("unsupported relay transport: {other}")),
    }
}

fn parse_env_bool(key: &str, value: &str) -> Result<bool, String> {
    match value.to_ascii_lowercase().as_str() {
        "1" | "true" | "yes" | "on" => Ok(true),
        "0" | "false" | "no" | "off" => Ok(false),
        _ => Err(format!("{key} must be a boolean value")),
    }
}

fn parse_env_u64(key: &str, value: &str) -> Result<u64, String> {
    value
        .parse::<u64>()
        .map_err(|err| format!("{key} must be an unsigned integer: {err}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn apply_env_overrides_relay_mqtt_config() {
        let mut config = DaemonConfig::default();

        config
            .apply_env_pairs([
                ("SLAN_RELAY_UDP_BIND", "0.0.0.0:9100"),
                ("SLAN_RELAY_TCP_BIND", "0.0.0.0:9101"),
                ("SLAN_RELAY_TICKET_SIGNING_SECRET", "ticket-secret"),
                ("SLAN_RELAY_MQTT_ENABLED", "true"),
                ("SLAN_RELAY_MQTT_BROKER_URL", "mqtt://bifromq:1883"),
                ("SLAN_RELAY_MQTT_TOPIC_PREFIX", "slan/devices"),
                ("SLAN_RELAY_MQTT_USERNAME_PREFIX", "slan"),
                ("SLAN_RELAY_MQTT_PASSWORD_SECRET", "mqtt-secret"),
                ("SLAN_RELAY_NODE_ID", "relay-cn-local-udp"),
                ("SLAN_RELAY_CLUSTER_ID", "cn-local-a"),
                ("SLAN_RELAY_COUNTRY_CODE", "cn"),
                ("SLAN_RELAY_CITY_CODE", "local"),
                ("SLAN_RELAY_TRANSPORT", "udp"),
                ("SLAN_RELAY_ADDRESS", "127.0.0.1:19000"),
                ("SLAN_RELAY_MQTT_INTERVAL_SECONDS", "10"),
                ("SLAN_RELAY_MQTT_CREDENTIAL_TTL_SECONDS", "600"),
            ])
            .unwrap();

        let mqtt = config.mqtt.unwrap();
        assert_eq!(config.udp_bind, "0.0.0.0:9100");
        assert_eq!(config.tcp_bind.as_deref(), Some("0.0.0.0:9101"));
        assert_eq!(
            config.ticket_signing_secret.as_deref(),
            Some("ticket-secret")
        );
        assert!(!config.allow_unsigned_tickets);
        assert!(mqtt.enabled);
        assert_eq!(mqtt.broker_url, "mqtt://bifromq:1883");
        assert_eq!(mqtt.node_id, "relay-cn-local-udp");
        assert_eq!(mqtt.cluster_id.as_deref(), Some("cn-local-a"));
        assert_eq!(mqtt.country_code.as_deref(), Some("CN"));
        assert_eq!(mqtt.city_code.as_deref(), Some("local"));
        assert_eq!(mqtt.transport.as_deref(), Some("udp"));
        assert_eq!(mqtt.address.as_deref(), Some("127.0.0.1:19000"));
        assert_eq!(mqtt.interval_seconds, Some(10));
        assert_eq!(mqtt.credential_ttl_seconds, Some(600));
    }

    #[test]
    fn apply_env_overrides_can_disable_mqtt() {
        let mut config = DaemonConfig {
            mqtt: Some(RelayMqttConfig::default()),
            ..DaemonConfig::default()
        };

        config
            .apply_env_pairs([("SLAN_RELAY_MQTT_ENABLED", "false")])
            .unwrap();

        assert!(!config.mqtt.unwrap().enabled);
    }

    #[test]
    fn apply_env_overrides_can_allow_unsigned_tickets_for_development() {
        let mut config = DaemonConfig::default();

        config
            .apply_env_pairs([("SLAN_RELAY_ALLOW_UNSIGNED_TICKETS", "true")])
            .unwrap();

        assert!(config.allow_unsigned_tickets);
    }

    #[test]
    fn validate_rejects_declared_tls_or_http3_until_listener_exists() {
        for transport in ["tls", "http3"] {
            let config = DaemonConfig {
                mqtt: Some(RelayMqttConfig {
                    node_id: format!("relay-{transport}"),
                    transport: Some(transport.to_string()),
                    ..RelayMqttConfig::default()
                }),
                ..DaemonConfig::default()
            };

            assert!(config.validate().is_err());
        }
    }

    #[test]
    fn validate_requires_tcp_listener_for_tcp_transport() {
        let config = DaemonConfig {
            mqtt: Some(RelayMqttConfig {
                node_id: "relay-cn-local-tcp".to_string(),
                transport: Some("tcp".to_string()),
                ..RelayMqttConfig::default()
            }),
            tcp_bind: None,
            ..DaemonConfig::default()
        };

        assert!(config.validate().is_err());
    }

    #[test]
    fn validate_accepts_multiple_udp_tcp_heartbeat_nodes() {
        let config = DaemonConfig {
            tcp_bind: Some("0.0.0.0:9001".to_string()),
            mqtt: Some(RelayMqttConfig {
                nodes: vec![
                    RelayMqttNodeConfig {
                        node_id: "relay-cn-local-udp".to_string(),
                        cluster_id: None,
                        country_code: None,
                        city_code: None,
                        transport: "udp".to_string(),
                        address: "127.0.0.1:9000".to_string(),
                    },
                    RelayMqttNodeConfig {
                        node_id: "relay-cn-local-tcp".to_string(),
                        cluster_id: None,
                        country_code: None,
                        city_code: None,
                        transport: "tcp".to_string(),
                        address: "127.0.0.1:9001".to_string(),
                    },
                ],
                ..RelayMqttConfig::default()
            }),
            ..DaemonConfig::default()
        };

        config.validate().unwrap();
    }

    #[test]
    fn effective_nodes_inherit_parent_region() {
        let mqtt = RelayMqttConfig {
            cluster_id: Some("cn-local-a".to_string()),
            country_code: Some("cn".to_string()),
            city_code: Some("local".to_string()),
            nodes: vec![RelayMqttNodeConfig {
                node_id: "relay-cn-local-udp".to_string(),
                cluster_id: None,
                country_code: None,
                city_code: None,
                transport: "UDP".to_string(),
                address: "127.0.0.1:9000".to_string(),
            }],
            ..RelayMqttConfig::default()
        };

        let nodes = mqtt.effective_nodes();
        assert_eq!(nodes.len(), 1);
        assert_eq!(nodes[0].cluster_id.as_deref(), Some("cn-local-a"));
        assert_eq!(nodes[0].country_code.as_deref(), Some("CN"));
        assert_eq!(nodes[0].city_code.as_deref(), Some("local"));
        assert_eq!(nodes[0].transport, "udp");
    }
}
