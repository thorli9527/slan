use std::{
    fs,
    path::PathBuf,
    time::{SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result};
use serde::Deserialize;

const MAX_COMPLETED_TASK_HISTORY: usize = 200;

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ControlTaskDirection {
    Upstream,
    Downstream,
}

impl ControlTaskDirection {
    pub(crate) fn as_str(&self) -> &'static str {
        match self {
            Self::Upstream => "upstream",
            Self::Downstream => "downstream",
        }
    }

    pub(crate) fn from_str(value: &str) -> Option<Self> {
        match value {
            "upstream" => Some(Self::Upstream),
            "downstream" => Some(Self::Downstream),
            _ => None,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ControlTaskAction {
    EnableNetwork,
    DisableNetwork,
    ReconcileNetworkState,
    DeviceUserLoginSucceeded,
}

impl ControlTaskAction {
    pub(crate) fn as_str(&self) -> &'static str {
        match self {
            Self::EnableNetwork => "enableNetwork",
            Self::DisableNetwork => "disableNetwork",
            Self::ReconcileNetworkState => "reconcileNetworkState",
            Self::DeviceUserLoginSucceeded => "deviceUserLoginSucceeded",
        }
    }

    pub(crate) fn from_str(value: &str) -> Option<Self> {
        match value {
            "enableNetwork" => Some(Self::EnableNetwork),
            "disableNetwork" => Some(Self::DisableNetwork),
            "reconcileNetworkState" => Some(Self::ReconcileNetworkState),
            "deviceUserLoginSucceeded" => Some(Self::DeviceUserLoginSucceeded),
            _ => None,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ControlTaskStatus {
    Pending,
    Running,
    Succeeded,
    Failed,
}

impl ControlTaskStatus {
    pub(crate) fn as_str(&self) -> &'static str {
        match self {
            Self::Pending => "pending",
            Self::Running => "running",
            Self::Succeeded => "succeeded",
            Self::Failed => "failed",
        }
    }

    fn from_str(value: &str) -> Option<Self> {
        match value {
            "pending" => Some(Self::Pending),
            "running" => Some(Self::Running),
            "succeeded" => Some(Self::Succeeded),
            "failed" => Some(Self::Failed),
            _ => None,
        }
    }
}

#[derive(Debug, Clone)]
pub struct ControlTask {
    pub id: String,
    pub delivery_id: Option<String>,
    pub direction: ControlTaskDirection,
    pub action: ControlTaskAction,
    pub status: ControlTaskStatus,
    pub require_ui_refresh: bool,
    pub created_at_ms: u64,
    pub updated_at_ms: u64,
    pub acknowledged_at_ms: Option<u64>,
    pub error: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EnqueueControlTaskRequest {
    pub action: String,
    #[serde(default = "default_task_direction")]
    pub direction: String,
    #[serde(default)]
    pub delivery_id: Option<String>,
    #[serde(default)]
    pub require_ui_refresh: bool,
}

pub struct ControlTaskQueue {
    path: PathBuf,
    tasks: Vec<ControlTask>,
}

impl ControlTaskQueue {
    pub fn load_default() -> Self {
        let path = task_file_path();
        let mut tasks = fs::read_to_string(&path)
            .ok()
            .map(|payload| parse_tasks(&payload))
            .unwrap_or_default();
        prune_completed_task_history(&mut tasks);
        Self { path, tasks }
    }

    pub fn enqueue(&mut self, request: EnqueueControlTaskRequest) -> Result<ControlTask> {
        let action = ControlTaskAction::from_str(request.action.trim()).ok_or_else(|| {
            anyhow::anyhow!("unsupported control task action: {}", request.action)
        })?;
        let direction =
            ControlTaskDirection::from_str(request.direction.trim()).ok_or_else(|| {
                anyhow::anyhow!("unsupported control task direction: {}", request.direction)
            })?;
        let now = current_timestamp_ms();
        let delivery_id = request
            .delivery_id
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty());
        if delivery_id.is_none() {
            if let Some(existing) = self.tasks.iter().rev().find(|task| {
                task.direction == direction
                    && task.action == action
                    && matches!(
                        task.status,
                        ControlTaskStatus::Pending | ControlTaskStatus::Running
                    )
            }) {
                return Ok(existing.clone());
            }
        }
        let task_id = delivery_id
            .as_ref()
            .map(|delivery_id| format!("{}-{}-{delivery_id}", direction.as_str(), action.as_str()))
            .unwrap_or_else(|| format!("{}-task-{now}", direction.as_str()));
        if let Some(index) = self.tasks.iter().position(|task| task.id == task_id) {
            if self.tasks[index].status == ControlTaskStatus::Succeeded {
                return Ok(self.tasks[index].clone());
            }
            self.tasks[index].require_ui_refresh = request.require_ui_refresh;
            self.tasks[index].updated_at_ms = now;
            let task = self.tasks[index].clone();
            self.persist()?;
            return Ok(task);
        }
        let task = ControlTask {
            id: task_id,
            delivery_id,
            direction,
            action,
            status: ControlTaskStatus::Pending,
            require_ui_refresh: request.require_ui_refresh,
            created_at_ms: now,
            updated_at_ms: now,
            acknowledged_at_ms: None,
            error: None,
        };
        self.tasks.push(task.clone());
        prune_completed_task_history(&mut self.tasks);
        self.persist()?;
        Ok(task)
    }

    pub fn enqueue_downstream(
        &mut self,
        action: ControlTaskAction,
        delivery_id: impl Into<String>,
        require_ui_refresh: bool,
    ) -> Result<ControlTask> {
        self.enqueue(EnqueueControlTaskRequest {
            action: action.as_str().to_string(),
            direction: ControlTaskDirection::Downstream.as_str().to_string(),
            delivery_id: Some(delivery_id.into()),
            require_ui_refresh,
        })
    }

    pub fn enqueue_downstream_unacked(
        &mut self,
        action: ControlTaskAction,
        require_ui_refresh: bool,
    ) -> Result<ControlTask> {
        self.enqueue(EnqueueControlTaskRequest {
            action: action.as_str().to_string(),
            direction: ControlTaskDirection::Downstream.as_str().to_string(),
            delivery_id: None,
            require_ui_refresh,
        })
    }

    pub fn take_next_pending(&mut self) -> Result<Option<ControlTask>> {
        let Some(index) = self.next_pending_index() else {
            return Ok(None);
        };
        let task = &mut self.tasks[index];
        task.status = ControlTaskStatus::Running;
        task.updated_at_ms = current_timestamp_ms();
        let task = task.clone();
        self.persist()?;
        Ok(Some(task))
    }

    fn next_pending_index(&self) -> Option<usize> {
        self.tasks
            .iter()
            .position(|task| {
                task.direction == ControlTaskDirection::Downstream
                    && task.status == ControlTaskStatus::Pending
            })
            .or_else(|| {
                self.tasks
                    .iter()
                    .position(|task| task.status == ControlTaskStatus::Pending)
            })
    }

    pub fn mark_succeeded(&mut self, task_id: &str) -> Result<()> {
        self.update_task(task_id, ControlTaskStatus::Succeeded, None)
    }

    pub fn mark_failed(&mut self, task_id: &str, error: String) -> Result<()> {
        self.update_task(task_id, ControlTaskStatus::Failed, Some(error))
    }

    pub fn pending_downstream_acks(&self) -> Vec<ControlTask> {
        self.tasks
            .iter()
            .filter(|task| {
                task.direction == ControlTaskDirection::Downstream
                    && task.delivery_id.is_some()
                    && task.acknowledged_at_ms.is_none()
                    && matches!(
                        task.status,
                        ControlTaskStatus::Succeeded | ControlTaskStatus::Failed
                    )
            })
            .cloned()
            .collect()
    }

    pub fn mark_acknowledged(&mut self, task_id: &str) -> Result<()> {
        if let Some(task) = self.tasks.iter_mut().find(|task| task.id == task_id) {
            task.acknowledged_at_ms = Some(current_timestamp_ms());
            task.updated_at_ms = current_timestamp_ms();
        }
        prune_completed_task_history(&mut self.tasks);
        self.persist()
    }

    fn update_task(
        &mut self,
        task_id: &str,
        status: ControlTaskStatus,
        error: Option<String>,
    ) -> Result<()> {
        if let Some(task) = self.tasks.iter_mut().find(|task| task.id == task_id) {
            task.status = status;
            task.error = error;
            task.updated_at_ms = current_timestamp_ms();
        }
        prune_completed_task_history(&mut self.tasks);
        self.persist()
    }

    fn persist(&self) -> Result<()> {
        if let Some(parent) = self.path.parent() {
            fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
        }
        let temporary_path = self
            .path
            .with_extension(format!("xml.tmp-{}", std::process::id()));
        fs::write(&temporary_path, render_tasks(&self.tasks))
            .with_context(|| format!("write {}", temporary_path.display()))?;
        #[cfg(target_os = "windows")]
        if self.path.exists() {
            fs::remove_file(&self.path)
                .with_context(|| format!("replace {}", self.path.display()))?;
        }
        fs::rename(&temporary_path, &self.path)
            .with_context(|| format!("replace {}", self.path.display()))
    }
}

fn prune_completed_task_history(tasks: &mut Vec<ControlTask>) {
    let removable = tasks
        .iter()
        .filter(|task| {
            matches!(
                task.status,
                ControlTaskStatus::Succeeded | ControlTaskStatus::Failed
            ) && (task.delivery_id.is_none() || task.acknowledged_at_ms.is_some())
        })
        .count()
        .saturating_sub(MAX_COMPLETED_TASK_HISTORY);
    let mut removed = 0;
    tasks.retain(|task| {
        let can_remove = matches!(
            task.status,
            ControlTaskStatus::Succeeded | ControlTaskStatus::Failed
        ) && (task.delivery_id.is_none() || task.acknowledged_at_ms.is_some());
        if can_remove && removed < removable {
            removed += 1;
            false
        } else {
            true
        }
    });
}

fn task_file_path() -> PathBuf {
    let base = app_data_dir();
    base.join("SLAN").join("client-v2-control-tasks.xml")
}

fn default_task_direction() -> String {
    ControlTaskDirection::Upstream.as_str().to_string()
}

fn app_data_dir() -> PathBuf {
    if let Some(dir) = std::env::var_os("SLAN_STATE_DIR") {
        return PathBuf::from(dir);
    }
    if cfg!(target_os = "windows") {
        return std::env::var_os("ProgramData")
            .map(PathBuf::from)
            .unwrap_or_else(|| PathBuf::from(r"C:\ProgramData"));
    }
    if cfg!(target_os = "macos") {
        return PathBuf::from("/Library/Application Support");
    }
    if cfg!(target_os = "ios") {
        if let Some(home) = std::env::var_os("HOME") {
            return PathBuf::from(home)
                .join("Library")
                .join("Application Support");
        }
        return std::env::temp_dir();
    }
    if cfg!(target_os = "android") {
        return std::env::temp_dir();
    }
    PathBuf::from("/var/lib")
}

fn render_tasks(tasks: &[ControlTask]) -> String {
    let mut xml = String::from("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<controlTasks>\n");
    render_task_group(
        &mut xml,
        "upstreamTasks",
        tasks
            .iter()
            .filter(|task| task.direction == ControlTaskDirection::Upstream),
    );
    render_task_group(
        &mut xml,
        "downstreamTasks",
        tasks
            .iter()
            .filter(|task| task.direction == ControlTaskDirection::Downstream),
    );
    xml.push_str("</controlTasks>\n");
    xml
}

fn render_task_group<'a>(
    xml: &mut String,
    group_name: &str,
    tasks: impl Iterator<Item = &'a ControlTask>,
) {
    xml.push_str(&format!("  <{group_name}>\n"));
    for task in tasks {
        xml.push_str(&format!(
            "    <task id=\"{}\" deliveryId=\"{}\" direction=\"{}\" action=\"{}\" status=\"{}\" requireUiRefresh=\"{}\" createdAtMs=\"{}\" updatedAtMs=\"{}\" acknowledgedAtMs=\"{}\">",
            escape_xml(&task.id),
            escape_xml(task.delivery_id.as_deref().unwrap_or_default()),
            task.direction.as_str(),
            task.action.as_str(),
            task.status.as_str(),
            task.require_ui_refresh,
            task.created_at_ms,
            task.updated_at_ms,
            task.acknowledged_at_ms
                .map(|value| value.to_string())
                .unwrap_or_default(),
        ));
        if let Some(error) = &task.error {
            xml.push_str(&format!("<error>{}</error>", escape_xml(error)));
        }
        xml.push_str("</task>\n");
    }
    xml.push_str(&format!("  </{group_name}>\n"));
}

fn parse_tasks(payload: &str) -> Vec<ControlTask> {
    payload
        .lines()
        .filter_map(|line| {
            let line = line.trim();
            if !line.starts_with("<task ") {
                return None;
            }
            let action = ControlTaskAction::from_str(&attr(line, "action")?)?;
            let status = ControlTaskStatus::from_str(&attr(line, "status")?)?;
            Some(ControlTask {
                id: attr(line, "id")?,
                delivery_id: attr(line, "deliveryId").filter(|value| !value.trim().is_empty()),
                direction: attr(line, "direction")
                    .as_deref()
                    .and_then(ControlTaskDirection::from_str)
                    .unwrap_or(ControlTaskDirection::Upstream),
                action,
                status,
                require_ui_refresh: attr(line, "requireUiRefresh")
                    .map(|value| value == "true")
                    .unwrap_or(false),
                created_at_ms: attr(line, "createdAtMs")
                    .and_then(|value| value.parse().ok())
                    .unwrap_or_default(),
                updated_at_ms: attr(line, "updatedAtMs")
                    .and_then(|value| value.parse().ok())
                    .unwrap_or_default(),
                acknowledged_at_ms: attr(line, "acknowledgedAtMs")
                    .and_then(|value| value.parse().ok()),
                error: text_between(line, "<error>", "</error>"),
            })
        })
        .collect()
}

fn attr(line: &str, name: &str) -> Option<String> {
    let needle = format!("{name}=\"");
    let start = line.find(&needle)? + needle.len();
    let end = line[start..].find('"')? + start;
    Some(unescape_xml(&line[start..end]))
}

fn text_between(line: &str, start: &str, end: &str) -> Option<String> {
    let start_index = line.find(start)? + start.len();
    let end_index = line[start_index..].find(end)? + start_index;
    Some(unescape_xml(&line[start_index..end_index]))
}

fn escape_xml(value: &str) -> String {
    value
        .replace('&', "&amp;")
        .replace('"', "&quot;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
}

fn unescape_xml(value: &str) -> String {
    value
        .replace("&quot;", "\"")
        .replace("&gt;", ">")
        .replace("&lt;", "<")
        .replace("&amp;", "&")
}

fn current_timestamp_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn queue_path(name: &str) -> PathBuf {
        let mut path = std::env::temp_dir();
        path.push(format!(
            "slan-control-tasks-{name}-{}-{}.xml",
            std::process::id(),
            current_timestamp_ms()
        ));
        let _ = fs::remove_file(&path);
        path
    }

    fn queue(name: &str) -> ControlTaskQueue {
        ControlTaskQueue {
            path: queue_path(name),
            tasks: Vec::new(),
        }
    }

    #[test]
    fn downstream_delivery_id_is_idempotent() {
        let mut queue = queue("idempotent");

        let first = queue
            .enqueue_downstream(ControlTaskAction::EnableNetwork, "delivery-1", true)
            .expect("enqueue first downstream task");
        let second = queue
            .enqueue_downstream(ControlTaskAction::EnableNetwork, "delivery-1", false)
            .expect("enqueue duplicate downstream task");

        assert_eq!(first.id, second.id);
        assert_eq!(queue.tasks.len(), 1);
        assert_eq!(queue.tasks[0].delivery_id.as_deref(), Some("delivery-1"));
        assert!(!queue.tasks[0].require_ui_refresh);
    }

    #[test]
    fn reconcile_network_state_action_round_trips() {
        assert_eq!(
            ControlTaskAction::from_str("reconcileNetworkState"),
            Some(ControlTaskAction::ReconcileNetworkState)
        );
        assert_eq!(
            ControlTaskAction::ReconcileNetworkState.as_str(),
            "reconcileNetworkState"
        );
    }

    #[test]
    fn device_user_login_succeeded_action_round_trips() {
        assert_eq!(
            ControlTaskAction::from_str("deviceUserLoginSucceeded"),
            Some(ControlTaskAction::DeviceUserLoginSucceeded)
        );
        assert_eq!(
            ControlTaskAction::DeviceUserLoginSucceeded.as_str(),
            "deviceUserLoginSucceeded"
        );
    }

    #[test]
    fn downstream_ack_lifecycle_only_reports_unacknowledged_terminal_tasks() {
        let mut queue = queue("ack-lifecycle");
        let succeeded = queue
            .enqueue_downstream(ControlTaskAction::EnableNetwork, "delivery-ok", false)
            .expect("enqueue succeeded task");
        let failed = queue
            .enqueue_downstream(ControlTaskAction::DisableNetwork, "delivery-failed", false)
            .expect("enqueue failed task");

        assert!(queue.pending_downstream_acks().is_empty());

        queue
            .mark_succeeded(&succeeded.id)
            .expect("mark task succeeded");
        queue
            .mark_failed(&failed.id, "configure failed".to_string())
            .expect("mark task failed");

        let acks = queue.pending_downstream_acks();
        assert_eq!(acks.len(), 2);
        assert!(acks.iter().any(|task| {
            task.delivery_id.as_deref() == Some("delivery-ok")
                && task.status == ControlTaskStatus::Succeeded
        }));
        assert!(acks.iter().any(|task| {
            task.delivery_id.as_deref() == Some("delivery-failed")
                && task.status == ControlTaskStatus::Failed
                && task.error.as_deref() == Some("configure failed")
        }));

        queue
            .mark_acknowledged(&succeeded.id)
            .expect("ack succeeded task");
        let acks = queue.pending_downstream_acks();
        assert_eq!(acks.len(), 1);
        assert_eq!(acks[0].delivery_id.as_deref(), Some("delivery-failed"));
    }

    #[test]
    fn downstream_tasks_restore_pending_ack_after_restart() {
        let path = queue_path("restore");
        let mut before_restart = ControlTaskQueue {
            path: path.clone(),
            tasks: Vec::new(),
        };
        let task = before_restart
            .enqueue_downstream(ControlTaskAction::EnableNetwork, "delivery-restore", false)
            .expect("enqueue downstream task");
        before_restart
            .mark_succeeded(&task.id)
            .expect("mark task succeeded");

        let after_restart = ControlTaskQueue {
            path,
            tasks: parse_tasks(&fs::read_to_string(&before_restart.path).expect("read task file")),
        };

        let acks = after_restart.pending_downstream_acks();
        assert_eq!(acks.len(), 1);
        assert_eq!(acks[0].id, task.id);
        assert_eq!(acks[0].delivery_id.as_deref(), Some("delivery-restore"));
        assert_eq!(acks[0].status, ControlTaskStatus::Succeeded);
    }

    #[test]
    fn pruning_keeps_recent_history_and_unacknowledged_tasks() {
        let mut tasks = (0..(MAX_COMPLETED_TASK_HISTORY + 25))
            .map(|index| ControlTask {
                id: format!("completed-{index}"),
                delivery_id: None,
                direction: ControlTaskDirection::Downstream,
                action: ControlTaskAction::ReconcileNetworkState,
                status: ControlTaskStatus::Succeeded,
                require_ui_refresh: false,
                created_at_ms: index as u64,
                updated_at_ms: index as u64,
                acknowledged_at_ms: None,
                error: None,
            })
            .collect::<Vec<_>>();
        tasks.push(ControlTask {
            id: "waiting-ack".to_string(),
            delivery_id: Some("delivery-1".to_string()),
            direction: ControlTaskDirection::Downstream,
            action: ControlTaskAction::EnableNetwork,
            status: ControlTaskStatus::Succeeded,
            require_ui_refresh: false,
            created_at_ms: 1,
            updated_at_ms: 1,
            acknowledged_at_ms: None,
            error: None,
        });

        prune_completed_task_history(&mut tasks);

        assert_eq!(tasks.len(), MAX_COMPLETED_TASK_HISTORY + 1);
        assert!(tasks.iter().any(|task| task.id == "waiting-ack"));
        assert!(!tasks.iter().any(|task| task.id == "completed-0"));
        assert!(tasks
            .iter()
            .any(|task| task.id == format!("completed-{}", MAX_COMPLETED_TASK_HISTORY + 24)));
    }
}
