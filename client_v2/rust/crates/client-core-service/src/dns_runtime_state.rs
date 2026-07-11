use std::{
    collections::BTreeMap,
    sync::{Mutex, OnceLock},
};

#[allow(dead_code)]
#[derive(Debug, Clone, Default)]
pub struct DnsZoneView {
    pub zone_id: String,
    pub network_id: String,
    pub zone_name: String,
    pub expose_global: bool,
    pub updated_at: u64,
}

#[allow(dead_code)]
#[derive(Debug, Clone, Default)]
pub struct DnsRecordView {
    pub record_id: String,
    pub zone_id: String,
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
    pub updated_at: u64,
}

#[derive(Debug, Clone, Default)]
pub enum CachedDnsResultKind {
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
pub struct CachedDnsAnswer {
    pub qname: String,
    pub qtype: String,
    pub result_kind: CachedDnsResultKind,
    pub ttl: Option<u32>,
    pub answers: Vec<String>,
    pub expires_at_ms: u64,
}

#[derive(Debug, Clone, Default)]
pub struct RuntimeDnsState {
    pub active_network_id: Option<String>,
    pub zones_by_id: BTreeMap<String, DnsZoneView>,
    pub records_by_id: BTreeMap<String, DnsRecordView>,
    pub record_ids_by_fqdn: BTreeMap<String, Vec<String>>,
    pub cache_by_question: BTreeMap<String, CachedDnsAnswer>,
    pub upstream_servers: Vec<String>,
    pub last_reload_at_ms: Option<u64>,
}

static DNS_RUNTIME_STATE: OnceLock<Mutex<RuntimeDnsState>> = OnceLock::new();

pub(crate) fn dns_runtime_state() -> &'static Mutex<RuntimeDnsState> {
    DNS_RUNTIME_STATE.get_or_init(|| Mutex::new(RuntimeDnsState::default()))
}

impl RuntimeDnsState {
    pub fn bind_network_id(&mut self, network_id: &str) {
        let value = network_id.trim();
        if value.is_empty() {
            return;
        }
        self.active_network_id = Some(value.to_string());
    }

    pub fn replace_zones(&mut self, zones: Vec<DnsZoneView>) {
        self.zones_by_id = zones
            .into_iter()
            .map(|item| (item.zone_id.clone(), item))
            .collect();
    }

    pub fn replace_records(&mut self, records: Vec<DnsRecordView>) {
        self.records_by_id.clear();
        self.record_ids_by_fqdn.clear();
        for record in records {
            let fqdn = normalize_fqdn(&record.fqdn);
            if !fqdn.is_empty() {
                self.record_ids_by_fqdn
                    .entry(fqdn)
                    .or_default()
                    .push(record.record_id.clone());
            }
            self.records_by_id.insert(record.record_id.clone(), record);
        }
    }

    pub fn set_upstream_servers(&mut self, upstream_servers: Vec<String>) {
        self.upstream_servers = upstream_servers
            .into_iter()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .collect();
    }

    pub fn clear_cache(&mut self) {
        self.cache_by_question.clear();
    }

    pub fn cache_key(qname: &str, qtype: &str) -> String {
        format!(
            "{}|{}",
            normalize_fqdn(qname),
            qtype.trim().to_ascii_uppercase()
        )
    }

    pub fn cached_answer(&self, qname: &str, qtype: &str, now_ms: u64) -> Option<&CachedDnsAnswer> {
        let key = Self::cache_key(qname, qtype);
        self.cache_by_question
            .get(&key)
            .filter(|value| value.expires_at_ms > now_ms)
    }

    pub fn put_cached_answer(&mut self, answer: CachedDnsAnswer) {
        let key = Self::cache_key(&answer.qname, &answer.qtype);
        self.cache_by_question.insert(key, answer);
    }
}

pub(crate) fn normalize_fqdn(value: &str) -> String {
    value.trim().trim_end_matches('.').to_ascii_lowercase()
}
