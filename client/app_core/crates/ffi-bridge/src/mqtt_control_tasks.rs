use std::fs;
use std::path::PathBuf;

#[derive(Debug, Clone)]
pub struct MqttControlTask {
    pub id: String,
    pub task_type: String,
    pub network_id: String,
    pub device_id: String,
    pub status: String,
    pub attempts: u32,
    pub created_at_ms: u64,
    pub updated_at_ms: u64,
    pub ui_refresh_required: bool,
    pub ui_refresh_reason: String,
    pub ui_refresh_at_ms: u64,
    pub error: String,
}

impl MqttControlTask {
    pub fn new(task_type: &str, network_id: &str, device_id: &str, timestamp_ms: u64) -> Self {
        Self {
            id: mqtt_control_task_id(task_type, network_id, device_id),
            task_type: task_type.to_string(),
            network_id: network_id.to_string(),
            device_id: device_id.to_string(),
            status: "pending".to_string(),
            attempts: 0,
            created_at_ms: timestamp_ms,
            updated_at_ms: timestamp_ms,
            ui_refresh_required: true,
            ui_refresh_reason: mqtt_control_ui_refresh_reason(task_type).to_string(),
            ui_refresh_at_ms: 0,
            error: String::new(),
        }
    }
}

pub fn mqtt_control_task_id(task_type: &str, network_id: &str, device_id: &str) -> String {
    format!(
        "{}:{}:{}",
        task_type.trim(),
        network_id.trim(),
        device_id.trim()
    )
}

pub fn append_mqtt_control_tasks(new_tasks: &[MqttControlTask]) -> Result<(), String> {
    let mut tasks = read_mqtt_control_tasks()?;
    for new_task in new_tasks {
        if new_task.network_id.trim().is_empty() || new_task.device_id.trim().is_empty() {
            continue;
        }
        if let Some(existing) = tasks.iter_mut().find(|task| task.id == new_task.id) {
            if existing.status != "running" {
                existing.status = "pending".to_string();
                existing.attempts = 0;
            }
            existing.task_type = new_task.task_type.clone();
            existing.network_id = new_task.network_id.clone();
            existing.device_id = new_task.device_id.clone();
            existing.updated_at_ms = new_task.updated_at_ms;
            existing.ui_refresh_required = new_task.ui_refresh_required;
            existing.ui_refresh_reason = new_task.ui_refresh_reason.clone();
            existing.ui_refresh_at_ms = 0;
            existing.error.clear();
        } else {
            tasks.push(new_task.clone());
        }
    }
    write_mqtt_control_tasks(&tasks)
}

pub fn update_mqtt_control_task_status(
    task_id: &str,
    status: &str,
    error: Option<&str>,
) -> Result<(), String> {
    let mut tasks = read_mqtt_control_tasks()?;
    let Some(task) = tasks.iter_mut().find(|task| task.id == task_id) else {
        return Ok(());
    };
    task.status = status.to_string();
    if status == "running" {
        task.attempts = task.attempts.saturating_add(1);
    }
    task.updated_at_ms = crate::diagnostics::now_ms();
    task.error = error.unwrap_or_default().to_string();
    write_mqtt_control_tasks(&tasks)
}

fn read_mqtt_control_tasks() -> Result<Vec<MqttControlTask>, String> {
    let path = mqtt_control_tasks_path();
    let payload = match fs::read_to_string(&path) {
        Ok(payload) => payload,
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => return Ok(Vec::new()),
        Err(err) => return Err(format!("read mqtt control tasks {}: {err}", path.display())),
    };
    let mut tasks = Vec::new();
    for line in payload.lines() {
        let trimmed = line.trim();
        if !trimmed.starts_with("<task ") {
            continue;
        }
        let id = xml_attr(trimmed, "id").unwrap_or_default();
        let task_type = xml_attr(trimmed, "type").unwrap_or_default();
        let network_id = xml_attr(trimmed, "networkId").unwrap_or_default();
        let device_id = xml_attr(trimmed, "deviceId").unwrap_or_default();
        if id.is_empty() || task_type.is_empty() {
            continue;
        }
        tasks.push(MqttControlTask {
            id,
            task_type,
            network_id,
            device_id,
            status: xml_attr(trimmed, "status").unwrap_or_else(|| "pending".to_string()),
            attempts: xml_attr(trimmed, "attempts")
                .and_then(|value| value.parse::<u32>().ok())
                .unwrap_or_default(),
            created_at_ms: xml_attr(trimmed, "createdAtMs")
                .and_then(|value| value.parse::<u64>().ok())
                .unwrap_or_default(),
            updated_at_ms: xml_attr(trimmed, "updatedAtMs")
                .and_then(|value| value.parse::<u64>().ok())
                .unwrap_or_default(),
            ui_refresh_required: xml_attr(trimmed, "uiRefreshRequired")
                .map(|value| value == "true")
                .unwrap_or_default(),
            ui_refresh_reason: xml_attr(trimmed, "uiRefreshReason").unwrap_or_default(),
            ui_refresh_at_ms: xml_attr(trimmed, "uiRefreshAtMs")
                .and_then(|value| value.parse::<u64>().ok())
                .unwrap_or_default(),
            error: xml_attr(trimmed, "error").unwrap_or_default(),
        });
    }
    Ok(tasks)
}

fn write_mqtt_control_tasks(tasks: &[MqttControlTask]) -> Result<(), String> {
    let path = mqtt_control_tasks_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|err| {
            format!(
                "create mqtt control tasks directory {}: {err}",
                parent.display()
            )
        })?;
    }
    let mut payload = String::from("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n");
    payload.push_str("<mqttControlTasks>\n");
    for task in tasks {
        payload.push_str("  <task");
        payload.push_str(&format!(" id=\"{}\"", xml_escape(&task.id)));
        payload.push_str(&format!(" type=\"{}\"", xml_escape(&task.task_type)));
        payload.push_str(&format!(" networkId=\"{}\"", xml_escape(&task.network_id)));
        payload.push_str(&format!(" deviceId=\"{}\"", xml_escape(&task.device_id)));
        payload.push_str(&format!(" status=\"{}\"", xml_escape(&task.status)));
        payload.push_str(&format!(" attempts=\"{}\"", task.attempts));
        payload.push_str(&format!(" createdAtMs=\"{}\"", task.created_at_ms));
        payload.push_str(&format!(" updatedAtMs=\"{}\"", task.updated_at_ms));
        payload.push_str(&format!(
            " uiRefreshRequired=\"{}\"",
            task.ui_refresh_required
        ));
        payload.push_str(&format!(
            " uiRefreshReason=\"{}\"",
            xml_escape(&task.ui_refresh_reason)
        ));
        payload.push_str(&format!(" uiRefreshAtMs=\"{}\"", task.ui_refresh_at_ms));
        payload.push_str(&format!(" error=\"{}\"", xml_escape(&task.error)));
        payload.push_str(" />\n");
    }
    payload.push_str("</mqttControlTasks>\n");
    fs::write(&path, payload)
        .map_err(|err| format!("write mqtt control tasks {}: {err}", path.display()))
}

fn mqtt_control_ui_refresh_reason(task_type: &str) -> &str {
    match task_type {
        "enable_network" => "network_enabled",
        "disable_network" => "network_disabled",
        _ => "network_changed",
    }
}

fn mqtt_control_tasks_path() -> PathBuf {
    if let Ok(configured) = std::env::var("SLAN_MQTT_CONTROL_TASKS_FILE") {
        let trimmed = configured.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed);
        }
    }
    #[cfg(target_os = "windows")]
    if let Ok(program_data) = std::env::var("ProgramData") {
        let trimmed = program_data.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed)
                .join("SLAN")
                .join("mqtt-control-tasks.xml");
        }
    }
    #[cfg(target_os = "windows")]
    return PathBuf::from(r"C:\ProgramData\SLAN\mqtt-control-tasks.xml");
    #[cfg(not(target_os = "windows"))]
    std::env::temp_dir().join("slan-mqtt-control-tasks.xml")
}

fn xml_attr(line: &str, name: &str) -> Option<String> {
    let needle = format!("{name}=\"");
    let start = line.find(&needle)? + needle.len();
    let end = line[start..].find('"')? + start;
    Some(xml_unescape(&line[start..end]))
}

fn xml_escape(value: &str) -> String {
    value
        .replace('&', "&amp;")
        .replace('"', "&quot;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
}

fn xml_unescape(value: &str) -> String {
    value
        .replace("&quot;", "\"")
        .replace("&gt;", ">")
        .replace("&lt;", "<")
        .replace("&amp;", "&")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn task_id_is_stable_and_trimmed() {
        assert_eq!(
            mqtt_control_task_id(" disable_network ", " net-1 ", " dev-1 "),
            "disable_network:net-1:dev-1"
        );
    }

    #[test]
    fn new_task_sets_ui_refresh_reason_from_type() {
        let task = MqttControlTask::new("enable_network", "net-1", "dev-1", 42);

        assert_eq!(task.id, "enable_network:net-1:dev-1");
        assert_eq!(task.status, "pending");
        assert_eq!(task.attempts, 0);
        assert!(task.ui_refresh_required);
        assert_eq!(task.ui_refresh_reason, "network_enabled");
        assert_eq!(task.ui_refresh_at_ms, 0);
        assert_eq!(task.created_at_ms, 42);
        assert_eq!(task.updated_at_ms, 42);
    }
}
