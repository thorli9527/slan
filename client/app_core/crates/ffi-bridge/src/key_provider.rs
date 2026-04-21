use std::fs;
use std::path::{Path, PathBuf};
use std::sync::Arc;

use serde::{Deserialize, Serialize};
use slan_app_core::{TunnelKeyMaterial, WireGuardKeyPair};

pub trait TunnelKeyProvider: Send + Sync {
    fn generate(
        &self,
        device_id: &str,
        node_id: Option<&str>,
        peer_node_id: &str,
        created_at_ms: u64,
    ) -> Result<TunnelKeyMaterial, String>;
}

impl<T> TunnelKeyProvider for Arc<T>
where
    T: TunnelKeyProvider,
{
    fn generate(
        &self,
        device_id: &str,
        node_id: Option<&str>,
        peer_node_id: &str,
        created_at_ms: u64,
    ) -> Result<TunnelKeyMaterial, String> {
        self.as_ref()
            .generate(device_id, node_id, peer_node_id, created_at_ms)
    }
}

#[derive(Default)]
pub struct InMemoryTunnelKeyProvider;

impl TunnelKeyProvider for InMemoryTunnelKeyProvider {
    fn generate(
        &self,
        device_id: &str,
        node_id: Option<&str>,
        peer_node_id: &str,
        created_at_ms: u64,
    ) -> Result<TunnelKeyMaterial, String> {
        let public_suffix = node_id.unwrap_or(peer_node_id);
        Ok(TunnelKeyMaterial {
            device_id: Some(device_id.to_string()),
            node_id: node_id.map(str::to_string),
            key_pair: WireGuardKeyPair {
                public_key: format!("wg-pub-{device_id}-{public_suffix}-{created_at_ms}"),
                private_key: format!("wg-priv-{device_id}-{public_suffix}-{created_at_ms}"),
            },
            created_at_ms,
        })
    }
}

pub struct FileTunnelKeyProvider {
    path: PathBuf,
    fallback: InMemoryTunnelKeyProvider,
}

#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct TunnelKeyStore {
    entries: Vec<TunnelKeyMaterial>,
}

impl FileTunnelKeyProvider {
    pub fn new(path: impl Into<PathBuf>) -> Self {
        Self {
            path: path.into(),
            fallback: InMemoryTunnelKeyProvider,
        }
    }

    pub fn path(&self) -> &Path {
        &self.path
    }

    fn load_store(&self) -> Result<TunnelKeyStore, String> {
        match fs::read(&self.path) {
            Ok(bytes) => serde_json::from_slice(&bytes)
                .map_err(|err| format!("parse tunnel key store {}: {err}", self.path.display())),
            Err(err) if err.kind() == std::io::ErrorKind::NotFound => Ok(TunnelKeyStore::default()),
            Err(err) => Err(format!(
                "read tunnel key store {}: {err}",
                self.path.display()
            )),
        }
    }

    fn save_store(&self, store: &TunnelKeyStore) -> Result<(), String> {
        if let Some(parent) = self.path.parent() {
            fs::create_dir_all(parent).map_err(|err| {
                format!("create tunnel key store dir {}: {err}", parent.display())
            })?;
        }
        let payload = serde_json::to_vec_pretty(store)
            .map_err(|err| format!("serialize tunnel key store {}: {err}", self.path.display()))?;
        fs::write(&self.path, payload)
            .map_err(|err| format!("write tunnel key store {}: {err}", self.path.display()))
    }
}

impl TunnelKeyProvider for FileTunnelKeyProvider {
    fn generate(
        &self,
        device_id: &str,
        node_id: Option<&str>,
        peer_node_id: &str,
        created_at_ms: u64,
    ) -> Result<TunnelKeyMaterial, String> {
        let mut store = self.load_store()?;
        if let Some(existing) = store.entries.iter().find(|entry| {
            entry.device_id.as_deref() == Some(device_id) && entry.node_id.as_deref() == node_id
        }) {
            return Ok(existing.clone());
        }

        let material = self
            .fallback
            .generate(device_id, node_id, peer_node_id, created_at_ms)?;
        store.entries.push(material.clone());
        self.save_store(&store)?;
        Ok(material)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::{SystemTime, UNIX_EPOCH};

    #[test]
    fn in_memory_provider_uses_node_id_when_available() {
        let provider = InMemoryTunnelKeyProvider;
        let material = provider
            .generate("dev-1", Some("node-1"), "peer-1", 42)
            .unwrap();

        assert_eq!(material.node_id.as_deref(), Some("node-1"));
        assert!(material.key_pair.public_key.contains("node-1"));
    }

    #[test]
    fn in_memory_provider_falls_back_to_peer_node_id() {
        let provider = InMemoryTunnelKeyProvider;
        let material = provider.generate("dev-1", None, "peer-1", 42).unwrap();

        assert_eq!(material.node_id, None);
        assert!(material.key_pair.public_key.contains("peer-1"));
    }

    #[test]
    fn file_provider_reuses_persisted_material() {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_nanos();
        let path = std::env::temp_dir().join(format!("slan-tunnel-key-store-{unique}.json"));
        let provider = FileTunnelKeyProvider::new(&path);

        let first = provider
            .generate("dev-1", Some("node-1"), "peer-1", 42)
            .unwrap();
        let second = provider
            .generate("dev-1", Some("node-1"), "peer-2", 43)
            .unwrap();

        assert_eq!(first, second);
        let _ = fs::remove_file(path);
    }
}
