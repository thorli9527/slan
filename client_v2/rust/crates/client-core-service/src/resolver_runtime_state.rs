use std::{
    collections::BTreeMap,
    sync::{Mutex, OnceLock},
};

use crate::network_runtime_state::RuntimeNetworkState;

#[derive(Debug, Clone, Default)]
pub struct ResolverZoneView {
    pub zone_id: String,
    pub zone_name: String,
}

#[derive(Debug, Clone, Default)]
pub struct ResolverRecordView {
    pub record_id: String,
    pub network_id: String,
    pub name: String,
    pub fqdn: String,
    pub record_type: String,
    pub value: String,
    pub target_device_id: String,
    pub target_ip: String,
    pub cname: String,
    pub port: i32,
    pub ttl: u32,
    pub enabled: bool,
}

#[derive(Debug, Clone, Default)]
pub enum CachedResolverResultKind {
    #[default]
    NoData,
    AnswerA,
    AnswerAaaa,
    AnswerCname,
    AnswerPtr,
    AnswerTxt,
    AnswerSrv,
    NxDomain,
}

#[derive(Debug, Clone, Default)]
pub struct CachedResolverAnswer {
    pub qname: String,
    pub qtype: String,
    pub result_kind: CachedResolverResultKind,
    pub ttl: Option<u32>,
    pub answers: Vec<String>,
    pub expires_at_ms: u64,
}

#[derive(Debug, Clone, Default)]
pub struct ResolverConfigState {
    pub active_network_id: Option<String>,
    pub search_domains: Vec<String>,
    pub split_domains: Vec<String>,
    pub upstream_resolvers: Vec<String>,
    pub fallback_to_system_resolvers: bool,
}

#[derive(Debug, Clone, Default)]
pub struct ResolverAuthorityState {
    pub zones_by_id: BTreeMap<String, ResolverZoneView>,
    pub records_by_id: BTreeMap<String, ResolverRecordView>,
    pub record_ids_by_fqdn: BTreeMap<String, Vec<String>>,
}

#[derive(Debug, Clone, Default)]
pub struct ResolverCacheState {
    pub cache_by_question: BTreeMap<String, CachedResolverAnswer>,
}

#[derive(Debug, Clone, Default)]
pub struct RuntimeResolverState {
    pub config: ResolverConfigState,
    pub authority: ResolverAuthorityState,
    pub cache: ResolverCacheState,
    pub networks_by_id: BTreeMap<String, RuntimeNetworkState>,
    pub last_reload_at_ms: Option<u64>,
}

static RESOLVER_RUNTIME_STATE: OnceLock<Mutex<RuntimeResolverState>> = OnceLock::new();

pub(crate) fn resolver_runtime_state() -> &'static Mutex<RuntimeResolverState> {
    RESOLVER_RUNTIME_STATE.get_or_init(|| Mutex::new(RuntimeResolverState::default()))
}

pub(crate) fn clear_resolver_runtime_state() {
    let mut state = resolver_runtime_state()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    *state = RuntimeResolverState::default();
}

impl RuntimeResolverState {
    pub fn bind_network_id(&mut self, network_id: &str) {
        let value = network_id.trim();
        if value.is_empty() {
            return;
        }
        self.config.active_network_id = Some(value.to_string());
    }

    pub fn replace_zones(&mut self, zones: Vec<ResolverZoneView>) {
        self.authority.zones_by_id = zones
            .into_iter()
            .map(|item| (item.zone_id.clone(), item))
            .collect();
    }

    pub fn replace_records(&mut self, records: Vec<ResolverRecordView>) {
        self.authority.records_by_id.clear();
        self.authority.record_ids_by_fqdn.clear();
        for record in records {
            let fqdn = normalize_fqdn(&record.fqdn);
            if !fqdn.is_empty() {
                self.authority
                    .record_ids_by_fqdn
                    .entry(fqdn)
                    .or_default()
                    .push(record.record_id.clone());
            }
            self.authority
                .records_by_id
                .insert(record.record_id.clone(), record);
        }
    }

    pub fn set_upstream_servers(&mut self, upstream_servers: Vec<String>) {
        self.config.upstream_resolvers = upstream_servers
            .into_iter()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .collect();
    }

    pub fn set_search_domains(&mut self, search_domains: Vec<String>) {
        self.config.search_domains = search_domains
            .into_iter()
            .map(|value| normalize_fqdn(&value))
            .filter(|value| !value.is_empty())
            .collect();
    }

    pub fn set_split_domains(&mut self, split_domains: Vec<String>) {
        self.config.split_domains = split_domains
            .into_iter()
            .map(|value| normalize_fqdn(&value))
            .filter(|value| !value.is_empty())
            .collect();
    }

    pub fn set_fallback_to_system_resolvers(&mut self, enabled: bool) {
        self.config.fallback_to_system_resolvers = enabled;
    }

    pub fn clear_cache(&mut self) {
        self.cache.cache_by_question.clear();
    }

    pub fn replace_network_states(&mut self, states: Vec<RuntimeNetworkState>) {
        self.networks_by_id = states
            .into_iter()
            .filter_map(|state| {
                let network_id = state.active_network_id.clone()?;
                (!network_id.trim().is_empty()).then_some((network_id, state))
            })
            .collect();
    }

    pub fn network_state(&self, network_id: &str) -> Option<&RuntimeNetworkState> {
        self.networks_by_id.get(network_id.trim())
    }

    pub fn cache_key(qname: &str, qtype: &str) -> String {
        format!(
            "{}|{}",
            normalize_fqdn(qname),
            qtype.trim().to_ascii_uppercase()
        )
    }

    pub fn cached_answer(
        &self,
        qname: &str,
        qtype: &str,
        now_ms: u64,
    ) -> Option<&CachedResolverAnswer> {
        let key = Self::cache_key(qname, qtype);
        self.cache
            .cache_by_question
            .get(&key)
            .filter(|value| value.expires_at_ms > now_ms)
    }

    pub fn put_cached_answer(&mut self, answer: CachedResolverAnswer) {
        let key = Self::cache_key(&answer.qname, &answer.qtype);
        self.cache.cache_by_question.insert(key, answer);
    }

    pub fn active_network_id(&self) -> Option<&str> {
        self.config.active_network_id.as_deref()
    }

    pub fn zone_count(&self) -> usize {
        self.authority.zones_by_id.len()
    }

    pub fn record_count(&self) -> usize {
        self.authority.records_by_id.len()
    }

    pub fn cache_count(&self) -> usize {
        self.cache.cache_by_question.len()
    }

    pub fn effective_upstream_resolvers(&self) -> Vec<String> {
        self.config.upstream_resolvers.clone()
    }

    pub fn has_resolver_data(&self) -> bool {
        !self.authority.records_by_id.is_empty()
            || !self.authority.zones_by_id.is_empty()
            || !self.effective_upstream_resolvers().is_empty()
    }

    pub fn matches_split_domain(&self, fqdn: &str) -> bool {
        let normalized = normalize_fqdn(fqdn);
        !normalized.is_empty()
            && self
                .config
                .split_domains
                .iter()
                .any(|domain| normalized == *domain || normalized.ends_with(&format!(".{domain}")))
    }
}

pub(crate) fn normalize_fqdn(value: &str) -> String {
    value.trim().trim_end_matches('.').to_ascii_lowercase()
}
