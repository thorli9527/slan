use std::{
    collections::BTreeMap,
    fs::File,
    io::{Read, Seek, SeekFrom},
    path::{Path, PathBuf},
    sync::OnceLock,
    thread,
    time::Duration,
};

use anyhow::{Context, Result};

use crate::{
    control_plane::ControlPlaneClient,
    log_service_error,
    session_store::{app_data_dir, current_timestamp_ms, load_session, session_device_api_token},
};

const INITIAL_UPLOAD_DELAY: Duration = Duration::from_secs(15);
const UPLOAD_INTERVAL: Duration = Duration::from_secs(5 * 60);
const MAX_FILE_TAIL_BYTES: u64 = 64 * 1024;
const MAX_UPLOAD_CONTENT_BYTES: usize = 512 * 1024;

pub(crate) fn spawn_worker() {
    static WORKER_STARTED: OnceLock<()> = OnceLock::new();
    if WORKER_STARTED.set(()).is_err() {
        return;
    }
    thread::spawn(|| {
        thread::sleep(INITIAL_UPLOAD_DELAY);
        loop {
            if let Err(error) = collect_and_upload() {
                log_service_error(format!(
                    "client-core-service diagnostic log upload skipped: {error:#}"
                ));
            }
            thread::sleep(UPLOAD_INTERVAL);
        }
    });
}

fn collect_and_upload() -> Result<()> {
    let session = load_session().context("load session for diagnostic upload")?;
    let device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .context("device id unavailable")?;
    let access_token = session_device_api_token(&session).trim();
    if access_token.is_empty() {
        anyhow::bail!("device token unavailable");
    }

    let mut files = BTreeMap::new();
    let mut remaining_bytes = MAX_UPLOAD_CONTENT_BYTES;
    for (name, path) in diagnostic_files() {
        if remaining_bytes == 0 {
            break;
        }
        let tail_bytes = MAX_FILE_TAIL_BYTES.min(remaining_bytes as u64);
        if let Some(content) = read_redacted_tail(&path, tail_bytes)? {
            remaining_bytes = remaining_bytes.saturating_sub(content.len());
            files.insert(name, content);
        }
    }
    if files.is_empty() {
        anyhow::bail!("no diagnostic logs available");
    }

    ControlPlaneClient::from_env().upload_device_logs(
        access_token,
        device_id,
        serde_json::json!({
            "deviceId": device_id,
            "platform": std::env::consts::OS,
            "version": env!("CARGO_PKG_VERSION"),
            "capturedAt": current_timestamp_ms(),
            "files": files,
        }),
    )?;
    log_service_error("client-core-service diagnostic log upload ok");
    Ok(())
}

fn diagnostic_files() -> Vec<(String, PathBuf)> {
    let state_dir = app_data_dir().join("SLAN");
    let mut files = vec![
        (
            "client-core-service.log".to_string(),
            state_dir.join("client-core-service.log"),
        ),
        (
            "slan-debug.log".to_string(),
            state_dir.join("slan-debug.log"),
        ),
        (
            "client-platform-error.log".to_string(),
            state_dir.join("client-platform-error.log"),
        ),
        (
            "relay-stats.json".to_string(),
            state_dir.join("client-v2-relay-stats.json"),
        ),
        (
            "network-state.json".to_string(),
            state_dir.join("client-v2-network-state.json"),
        ),
        (
            "direct-udp-endpoint.json".to_string(),
            state_dir.join("client-v2-direct-udp-endpoint.json"),
        ),
    ];
    for index in 1..=3 {
        files.push((
            format!("client-core-service.log.{index}"),
            state_dir.join(format!("client-core-service.log.{index}")),
        ));
    }
    files.push(("client-v2-ui.log".to_string(), ui_log_path()));
    files.push((
        "client-v2-native.log".to_string(),
        std::env::temp_dir()
            .join("slan")
            .join("client-v2-native.log"),
    ));
    #[cfg(target_os = "macos")]
    {
        let log_dir = PathBuf::from("/Library/Logs/SLAN");
        files.push((
            "client-core-service.stderr.log".to_string(),
            log_dir.join("client-core-service.stderr.log"),
        ));
        files.push((
            "client-core-service.stdout.log".to_string(),
            log_dir.join("client-core-service.stdout.log"),
        ));
    }
    files
}

fn ui_log_path() -> PathBuf {
    #[cfg(target_os = "windows")]
    {
        return std::env::var_os("ProgramData")
            .map(PathBuf::from)
            .unwrap_or_else(|| PathBuf::from(r"C:\ProgramData"))
            .join("SLAN")
            .join("client-v2-ui.log");
    }
    #[cfg(not(target_os = "windows"))]
    {
        std::env::temp_dir().join("slan").join("client-v2-ui.log")
    }
}

fn read_redacted_tail(path: &Path, max_tail_bytes: u64) -> Result<Option<String>> {
    let mut file = match File::open(path) {
        Ok(file) => file,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(error).with_context(|| format!("open {}", path.display())),
    };
    let length = file.metadata()?.len();
    if length > max_tail_bytes {
        file.seek(SeekFrom::End(-(max_tail_bytes as i64)))?;
    }
    let mut content = String::new();
    file.read_to_string(&mut content)?;
    if length > max_tail_bytes {
        if let Some(first_newline) = content.find('\n') {
            content.drain(..=first_newline);
        }
    }
    Ok(Some(redact_sensitive_values(&content)))
}

fn redact_sensitive_values(content: &str) -> String {
    let mut result = content.to_string();
    for marker in [
        "Authorization: Bearer ",
        "authorization: bearer ",
        "\"accessToken\":\"",
        "\"refreshToken\":\"",
        "\"deviceToken\":\"",
        "\"password\":\"",
    ] {
        result = redact_after_marker(result, marker);
    }
    result
}

fn redact_after_marker(mut value: String, marker: &str) -> String {
    let mut offset = 0;
    while let Some(relative) = value[offset..].find(marker) {
        let start = offset + relative + marker.len();
        let end = value[start..]
            .find(|char: char| char == '\r' || char == '\n' || char == '"' || char.is_whitespace())
            .map(|relative| start + relative)
            .unwrap_or(value.len());
        value.replace_range(start..end, "[REDACTED]");
        offset = start + "[REDACTED]".len();
    }
    value
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn redacts_tokens_and_passwords() {
        let source = "Authorization: Bearer secret-token\n{\"refreshToken\":\"refresh-secret\",\"password\":\"pass\"}";
        let result = redact_sensitive_values(source);
        assert!(!result.contains("secret-token"));
        assert!(!result.contains("refresh-secret"));
        assert!(!result.contains("\"pass\""));
        assert_eq!(result.matches("[REDACTED]").count(), 3);
    }

    #[test]
    fn diagnostic_files_include_rotated_and_native_logs() {
        let names = diagnostic_files()
            .into_iter()
            .map(|(name, _)| name)
            .collect::<Vec<_>>();
        assert!(names.contains(&"client-core-service.log.1".to_string()));
        assert!(names.contains(&"client-core-service.log.3".to_string()));
        assert!(names.contains(&"client-v2-ui.log".to_string()));
        assert!(names.contains(&"client-v2-native.log".to_string()));
        assert!(names.contains(&"client-platform-error.log".to_string()));
    }
}
