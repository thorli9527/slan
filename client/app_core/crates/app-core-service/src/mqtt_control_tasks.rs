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

#[derive(Debug, Clone, Copy)]
pub struct MqttControlTaskPolicy {
    pub max_attempts: u32,
    pub running_stale_ms: u64,
}

impl MqttControlTaskPolicy {
    pub fn next_runnable_index(&self, tasks: &[MqttControlTask], now_ms: u64) -> Option<usize> {
        tasks.iter().position(|task| {
            if task.attempts >= self.max_attempts && task.status == "failed" {
                return false;
            }
            match task.status.as_str() {
                "pending" => true,
                "failed" => task.attempts < self.max_attempts,
                "running" => now_ms.saturating_sub(task.updated_at_ms) > self.running_stale_ms,
                _ => false,
            }
        })
    }
}

pub fn read_mqtt_control_tasks() -> Result<Vec<MqttControlTask>, String> {
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
        if id.is_empty() || task_type.is_empty() {
            continue;
        }
        tasks.push(MqttControlTask {
            id,
            task_type,
            network_id: xml_attr(trimmed, "networkId").unwrap_or_default(),
            device_id: xml_attr(trimmed, "deviceId").unwrap_or_default(),
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

pub fn write_mqtt_control_tasks(tasks: &[MqttControlTask]) -> Result<(), String> {
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

    fn task(status: &str, attempts: u32, updated_at_ms: u64) -> MqttControlTask {
        MqttControlTask {
            id: format!("{status}:{attempts}:{updated_at_ms}"),
            task_type: "disable_network".to_string(),
            network_id: "net-1".to_string(),
            device_id: "dev-1".to_string(),
            status: status.to_string(),
            attempts,
            created_at_ms: 1,
            updated_at_ms,
            ui_refresh_required: true,
            ui_refresh_reason: "network_disabled".to_string(),
            ui_refresh_at_ms: 0,
            error: String::new(),
        }
    }

    #[test]
    fn policy_selects_pending_before_other_tasks() {
        let policy = MqttControlTaskPolicy {
            max_attempts: 3,
            running_stale_ms: 100,
        };
        let tasks = vec![task("succeeded", 0, 1), task("pending", 0, 10)];

        assert_eq!(policy.next_runnable_index(&tasks, 20), Some(1));
    }

    #[test]
    fn policy_retries_failed_until_max_attempts() {
        let policy = MqttControlTaskPolicy {
            max_attempts: 3,
            running_stale_ms: 100,
        };
        let tasks = vec![task("failed", 3, 1), task("failed", 2, 1)];

        assert_eq!(policy.next_runnable_index(&tasks, 20), Some(1));
    }

    #[test]
    fn policy_retries_stale_running_task() {
        let policy = MqttControlTaskPolicy {
            max_attempts: 3,
            running_stale_ms: 100,
        };
        let tasks = vec![task("running", 1, 10), task("running", 1, 150)];

        assert_eq!(policy.next_runnable_index(&tasks, 120), Some(0));
    }
}
