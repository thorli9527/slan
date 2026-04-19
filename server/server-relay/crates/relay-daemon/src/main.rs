use relay_daemon::{DaemonConfig, RelayDaemon};

fn main() {
    if let Err(err) = run() {
        eprintln!("{err}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let config = parse_args()?;
    let mut daemon = RelayDaemon::bind(config)?;
    eprintln!("server-relay listening on {}", daemon.local_addr()?);
    daemon.serve()
}

fn parse_args() -> Result<DaemonConfig, String> {
    let mut config = DaemonConfig::default();
    let mut args = std::env::args().skip(1);
    while let Some(arg) = args.next() {
        match arg.as_str() {
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
            flag => return Err(format!("unsupported flag: {flag}")),
        }
    }
    Ok(config)
}
