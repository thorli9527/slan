use relay_daemon::{DaemonConfig, RelayDaemon};

fn main() {
    if let Err(err) = run() {
        eprintln!("{err}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let mut config = parse_args()?;
    config.apply_env_overrides()?;
    let mut daemon = RelayDaemon::bind(config)?;
    eprintln!("server-relay listening on {}", daemon.local_addr()?);
    daemon.serve()
}

fn parse_args() -> Result<DaemonConfig, String> {
    let mut config = DaemonConfig::default();
    let mut args = std::env::args().skip(1);
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--config" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --config".to_string())?;
                config = DaemonConfig::from_file(value)?;
            }
            "--udp-bind" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --udp-bind".to_string())?;
                config.udp_bind = value;
            }
            "--relay-url-prefix" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --relay-url-prefix".to_string())?;
                config.relay_url_prefix = Some(value);
            }
            "--no-relay-url-prefix" => {
                config.relay_url_prefix = None;
            }
            "--ticket-signing-secret" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --ticket-signing-secret".to_string())?;
                config.ticket_signing_secret = Some(value);
            }
            "--mqtt-broker-url" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --mqtt-broker-url".to_string())?;
                config
                    .mqtt
                    .get_or_insert_with(default_mqtt_config)
                    .broker_url = value;
            }
            "--mqtt-topic-prefix" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --mqtt-topic-prefix".to_string())?;
                config
                    .mqtt
                    .get_or_insert_with(default_mqtt_config)
                    .topic_prefix = value;
            }
            "--mqtt-username-prefix" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --mqtt-username-prefix".to_string())?;
                config
                    .mqtt
                    .get_or_insert_with(default_mqtt_config)
                    .username_prefix = value;
            }
            "--mqtt-password-secret" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --mqtt-password-secret".to_string())?;
                config
                    .mqtt
                    .get_or_insert_with(default_mqtt_config)
                    .password_secret = value;
            }
            "--mqtt-enabled" => {
                config.mqtt.get_or_insert_with(default_mqtt_config).enabled = true;
            }
            "--mqtt-disabled" => {
                config.mqtt.get_or_insert_with(default_mqtt_config).enabled = false;
            }
            "--relay-cluster-id" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --relay-cluster-id".to_string())?;
                config
                    .mqtt
                    .get_or_insert_with(default_mqtt_config)
                    .cluster_id = Some(value);
            }
            "--relay-country-code" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --relay-country-code".to_string())?;
                config
                    .mqtt
                    .get_or_insert_with(default_mqtt_config)
                    .country_code = Some(value.to_ascii_uppercase());
            }
            "--relay-city-code" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --relay-city-code".to_string())?;
                config
                    .mqtt
                    .get_or_insert_with(default_mqtt_config)
                    .city_code = Some(value);
            }
            "--relay-transport" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --relay-transport".to_string())?;
                config
                    .mqtt
                    .get_or_insert_with(default_mqtt_config)
                    .transport = Some(value);
            }
            "--relay-address" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --relay-address".to_string())?;
                config.mqtt.get_or_insert_with(default_mqtt_config).address = Some(value);
            }
            "--mqtt-interval-seconds" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --mqtt-interval-seconds".to_string())?;
                config
                    .mqtt
                    .get_or_insert_with(default_mqtt_config)
                    .interval_seconds = Some(parse_u64_flag("--mqtt-interval-seconds", &value)?);
            }
            "--mqtt-credential-ttl-seconds" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --mqtt-credential-ttl-seconds".to_string())?;
                config
                    .mqtt
                    .get_or_insert_with(default_mqtt_config)
                    .credential_ttl_seconds =
                    Some(parse_u64_flag("--mqtt-credential-ttl-seconds", &value)?);
            }
            "--relay-node-id" => {
                let value = args
                    .next()
                    .ok_or_else(|| "missing value for --relay-node-id".to_string())?;
                let mqtt = config.mqtt.get_or_insert_with(default_mqtt_config);
                mqtt.enabled = true;
                mqtt.node_id = value;
            }
            flag => return Err(format!("unsupported flag: {flag}")),
        }
    }
    Ok(config)
}

fn parse_u64_flag(flag: &str, value: &str) -> Result<u64, String> {
    value
        .parse::<u64>()
        .map_err(|err| format!("{flag} must be an unsigned integer: {err}"))
}

fn default_mqtt_config() -> relay_daemon::RelayMqttConfig {
    relay_daemon::RelayMqttConfig::default()
}
