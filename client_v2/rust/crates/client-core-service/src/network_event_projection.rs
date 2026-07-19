use anyhow::{ensure, Result};

use crate::{
    control_plane::DeviceNetworkConfig,
    network_event::NetworkEventEnvelope,
    network_event_apply::{apply_network_event, ApplyResult},
    network_module::{
        apply_network_module_event, replace_network_module_configs, sync_resolver_runtime_state,
    },
    network_runtime_state::runtime_network_state_store,
    resolver_apply::apply_resolver_runtime_event,
    session_store::{
        current_session_runtime_epoch, lock_session_runtime_epoch, PersistedSession,
        PreparedSession,
    },
};

pub(crate) fn apply_prepared_session_projection(
    session_epoch: u64,
    prepared: &PreparedSession,
) -> Result<()> {
    let _session_guard = lock_session_runtime_epoch(session_epoch)?;
    ensure_current_session(session_epoch)?;
    if !prepared.relay_candidates.is_empty() {
        crate::relay_candidates::replace_runtime_relay_candidates(
            prepared.relay_candidates.clone(),
        );
    }
    if !prepared.network_configs.is_empty() {
        apply_device_network_configs_unlocked(&prepared.session, prepared.network_configs.clone());
    }
    Ok(())
}

fn apply_device_network_configs_unlocked(
    session: &PersistedSession,
    configs: Vec<DeviceNetworkConfig>,
) {
    replace_network_module_configs(configs.clone());
    sync_resolver_runtime_state(session, &configs);
}

pub(crate) fn apply_network_event_projection(
    session_epoch: u64,
    session: &PersistedSession,
    local_device_id: &str,
    envelope: &NetworkEventEnvelope,
) -> Result<ApplyResult> {
    let _session_guard = lock_session_runtime_epoch(session_epoch)?;
    let apply_result = runtime_network_state_store().try_update(|state| {
        ensure_current_session(session_epoch)?;
        state.bind_persisted_session(session);
        apply_network_event(state, envelope.clone())
    })?;
    if applies_secondary_projections(&apply_result) {
        apply_resolver_runtime_event(envelope)?;
        apply_network_module_event(&envelope.network_id, local_device_id, envelope)?;
    }
    Ok(apply_result)
}

fn applies_secondary_projections(result: &ApplyResult) -> bool {
    matches!(result, ApplyResult::Applied)
}

pub(crate) fn mark_network_projection_syncing(session_epoch: u64) -> Result<()> {
    let _session_guard = lock_session_runtime_epoch(session_epoch)?;
    runtime_network_state_store().try_update(|state| {
        ensure_current_session(session_epoch)?;
        state.mark_syncing_snapshot();
        Ok(())
    })
}

fn ensure_current_session(session_epoch: u64) -> Result<()> {
    ensure!(
        current_session_runtime_epoch() == session_epoch,
        "stale network projection session"
    );
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::applies_secondary_projections;
    use crate::network_event_apply::ApplyResult;

    #[test]
    fn secondary_projections_only_apply_for_committed_event() {
        assert!(applies_secondary_projections(&ApplyResult::Applied));
        assert!(!applies_secondary_projections(
            &ApplyResult::IgnoredDuplicate
        ));
        assert!(!applies_secondary_projections(&ApplyResult::IgnoredStale));
        assert!(!applies_secondary_projections(&ApplyResult::NeedsSnapshot));
    }
}
