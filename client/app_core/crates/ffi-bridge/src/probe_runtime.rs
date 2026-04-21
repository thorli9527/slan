use std::thread;
use std::time::Duration;

use relay_client::{DerpPool, PathManager, PathManagerError};
use slan_app_core::ActivePath;

use crate::diagnostics::now_ms;

pub fn probe_health_from_active_path<D: DerpPool>(
    derp_pool: &D,
    active_path: &ActivePath,
) -> (
    Option<u32>,
    Option<u32>,
    Option<u32>,
    Option<String>,
    Option<String>,
) {
    match active_path {
        ActivePath::Derp { .. } => derp_pool
            .active_link()
            .map(|link| {
                (
                    Some(link.health.rtt_ms_ewma),
                    Some(link.health.loss_ppm),
                    Some(link.health.score),
                    Some(link.meta.cluster_id),
                    Some(link.meta.node_id),
                )
            })
            .unwrap_or((None, None, None, None, None)),
        _ => (None, None, None, None, None),
    }
}

pub fn poll_reply_until_timeout<P: PathManager>(
    path_manager: &P,
    reply_timeout_ms: u64,
    poll_interval_ms: u64,
) -> Result<Option<(usize, u64)>, PathManagerError> {
    let started_at_ms = now_ms();
    let deadline_ms = started_at_ms.saturating_add(reply_timeout_ms);
    loop {
        if let Some(reply) = path_manager.poll_transport_packet()? {
            return Ok(Some((reply.len(), now_ms())));
        }
        if now_ms() >= deadline_ms {
            return Ok(None);
        }
        thread::sleep(Duration::from_millis(poll_interval_ms));
    }
}
