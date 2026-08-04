use std::{
    collections::BTreeMap,
    fs,
    io::Write,
    path::{Path, PathBuf},
    sync::{Mutex, OnceLock},
};

#[cfg(unix)]
use std::os::unix::fs::{OpenOptionsExt, PermissionsExt};

use aes_gcm::{
    aead::{Aead, KeyInit, Payload},
    Aes256Gcm, Nonce,
};
use anyhow::{Context, Result};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use sha2::{Digest, Sha256};

const CONFIG_VERSION: u32 = 1;
const ENCRYPTION_ALGORITHM: &str = "AES-256-GCM";
pub(crate) const KEY_SESSION: &str = "session";
pub(crate) const KEY_SERVER_API_BASE_URL: &str = "serverApiBaseUrl";
pub(crate) const KEY_DEVICE_PROVISIONED: &str = "deviceProvisioned";
pub(crate) const KEY_DEVICE_AUTHORIZATION_KEY: &str = "deviceAuthorizationKey";

#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ClientConfigFile {
    version: u32,
    device_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    encrypted: Option<EncryptedConfig>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct EncryptedConfig {
    algorithm: String,
    nonce: String,
    ciphertext: String,
}

#[derive(Debug, Default, Serialize, Deserialize)]
struct PrivateConfig {
    #[serde(flatten)]
    values: BTreeMap<String, serde_json::Value>,
}

fn config_lock() -> &'static Mutex<()> {
    static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
    LOCK.get_or_init(|| Mutex::new(()))
}

pub(crate) fn client_state_dir() -> PathBuf {
    crate::session_store::app_data_dir().join("SLAN")
}

pub(crate) fn config_file_path() -> PathBuf {
    client_state_dir().join("config.json")
}

pub(crate) fn load_device_id() -> Result<Option<String>> {
    let _guard = config_lock()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    let Some(config) = read_config_unlocked()? else {
        return Ok(None);
    };
    let device_id = config.device_id.trim();
    Ok((!device_id.is_empty()).then(|| device_id.to_string()))
}

pub(crate) fn store_device_id(device_id: &str) -> Result<()> {
    let device_id = device_id.trim();
    anyhow::ensure!(
        device_id.len() >= 20,
        "device id must contain at least 20 bytes"
    );
    let _guard = config_lock()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    let mut config = read_config_unlocked()?.unwrap_or_default();
    if config.device_id != device_id {
        config.encrypted = None;
    }
    config.version = CONFIG_VERSION;
    config.device_id = device_id.to_string();
    write_config_unlocked(&config)
}

pub(crate) fn load_secret<T>(device_id: &str, key: &str) -> Result<Option<T>>
where
    T: DeserializeOwned,
{
    let _guard = config_lock()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    let Some(config) = read_config_unlocked()? else {
        return Ok(None);
    };
    anyhow::ensure!(
        config.device_id == device_id.trim(),
        "config device id mismatch"
    );
    let private = decrypt_private_config(&config)?;
    private
        .values
        .get(key)
        .cloned()
        .map(serde_json::from_value)
        .transpose()
        .with_context(|| format!("decode encrypted config field {key}"))
}

pub(crate) fn store_secret<T>(device_id: &str, key: &str, value: &T) -> Result<()>
where
    T: Serialize,
{
    let _guard = config_lock()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    let mut config = read_config_unlocked()?.unwrap_or_default();
    if config.device_id.trim().is_empty() {
        config.version = CONFIG_VERSION;
        config.device_id = device_id.trim().to_string();
    }
    anyhow::ensure!(
        config.device_id == device_id.trim(),
        "config device id mismatch"
    );
    let mut private = decrypt_private_config(&config)?;
    private.values.insert(
        key.to_string(),
        serde_json::to_value(value).with_context(|| format!("encode config field {key}"))?,
    );
    config.encrypted = Some(encrypt_private_config(&config.device_id, &private)?);
    write_config_unlocked(&config)
}

pub(crate) fn remove_secret(device_id: &str, key: &str) -> Result<()> {
    let _guard = config_lock()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    let Some(mut config) = read_config_unlocked()? else {
        return Ok(());
    };
    anyhow::ensure!(
        config.device_id == device_id.trim(),
        "config device id mismatch"
    );
    let mut private = decrypt_private_config(&config)?;
    private.values.remove(key);
    config.encrypted = if private.values.is_empty() {
        None
    } else {
        Some(encrypt_private_config(&config.device_id, &private)?)
    };
    write_config_unlocked(&config)
}

fn read_config_unlocked() -> Result<Option<ClientConfigFile>> {
    let path = config_file_path();
    let payload = match fs::read(&path) {
        Ok(payload) => payload,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(error).with_context(|| format!("read {}", path.display())),
    };
    let config: ClientConfigFile =
        serde_json::from_slice(&payload).with_context(|| format!("decode {}", path.display()))?;
    anyhow::ensure!(
        config.version == CONFIG_VERSION,
        "unsupported client config version"
    );
    Ok(Some(config))
}

fn decrypt_private_config(config: &ClientConfigFile) -> Result<PrivateConfig> {
    let Some(encrypted) = config.encrypted.as_ref() else {
        return Ok(PrivateConfig::default());
    };
    anyhow::ensure!(
        encrypted.algorithm == ENCRYPTION_ALGORITHM,
        "unsupported client config encryption"
    );
    let nonce = BASE64
        .decode(&encrypted.nonce)
        .context("decode client config nonce")?;
    anyhow::ensure!(nonce.len() == 12, "invalid client config nonce length");
    let ciphertext = BASE64
        .decode(&encrypted.ciphertext)
        .context("decode client config ciphertext")?;
    let key = derive_key(&config.device_id)?;
    let cipher = Aes256Gcm::new_from_slice(&key).context("initialize client config cipher")?;
    let plaintext = cipher
        .decrypt(
            Nonce::from_slice(&nonce),
            Payload {
                msg: &ciphertext,
                aad: config.device_id.as_bytes(),
            },
        )
        .map_err(|_| anyhow::anyhow!("decrypt client config"))?;
    serde_json::from_slice(&plaintext).context("decode decrypted client config")
}

fn encrypt_private_config(device_id: &str, private: &PrivateConfig) -> Result<EncryptedConfig> {
    let key = derive_key(device_id)?;
    let cipher = Aes256Gcm::new_from_slice(&key).context("initialize client config cipher")?;
    let mut nonce = [0u8; 12];
    getrandom::getrandom(&mut nonce)
        .map_err(|error| anyhow::anyhow!("generate client config nonce: {error}"))?;
    let plaintext = serde_json::to_vec(private).context("encode private client config")?;
    let ciphertext = cipher
        .encrypt(
            Nonce::from_slice(&nonce),
            Payload {
                msg: &plaintext,
                aad: device_id.as_bytes(),
            },
        )
        .map_err(|_| anyhow::anyhow!("encrypt client config"))?;
    Ok(EncryptedConfig {
        algorithm: ENCRYPTION_ALGORITHM.to_string(),
        nonce: BASE64.encode(nonce),
        ciphertext: BASE64.encode(ciphertext),
    })
}

fn derive_key(device_id: &str) -> Result<[u8; 32]> {
    let device_id = device_id.trim().as_bytes();
    anyhow::ensure!(
        device_id.len() >= 20,
        "device id must contain at least 20 bytes"
    );
    Ok(Sha256::digest(&device_id[..20]).into())
}

fn write_config_unlocked(config: &ClientConfigFile) -> Result<()> {
    let path = config_file_path();
    let parent = path.parent().context("client config path has no parent")?;
    fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    #[cfg(unix)]
    fs::set_permissions(parent, fs::Permissions::from_mode(0o700))
        .with_context(|| format!("secure {}", parent.display()))?;
    let payload = serde_json::to_vec_pretty(config).context("encode client config")?;
    write_atomically(&path, &payload)
}

fn write_atomically(path: &Path, payload: &[u8]) -> Result<()> {
    let parent = path.parent().context("client config path has no parent")?;
    let temp_path = parent.join(format!(
        ".config.json.tmp-{}-{}",
        std::process::id(),
        crate::session_store::current_timestamp_ms()
    ));
    let mut options = fs::OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    options.mode(0o600);
    let mut file = options
        .open(&temp_path)
        .with_context(|| format!("create {}", temp_path.display()))?;
    file.write_all(payload)
        .with_context(|| format!("write {}", temp_path.display()))?;
    file.sync_all()
        .with_context(|| format!("sync {}", temp_path.display()))?;
    drop(file);
    #[cfg(windows)]
    if path.exists() {
        fs::remove_file(path).with_context(|| format!("remove {}", path.display()))?;
    }
    fs::rename(&temp_path, path)
        .with_context(|| format!("rename {} -> {}", temp_path.display(), path.display()))?;
    #[cfg(unix)]
    fs::set_permissions(path, fs::Permissions::from_mode(0o600))
        .with_context(|| format!("secure {}", path.display()))?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn encrypted_config_exposes_only_device_id() {
        let device_id = "0123456789abcdef0123456789abcdef";
        let mut private = PrivateConfig::default();
        private.values.insert(
            KEY_SESSION.to_string(),
            serde_json::json!({"deviceToken": "secret-device-token"}),
        );
        private
            .values
            .insert(KEY_DEVICE_PROVISIONED.to_string(), serde_json::json!(true));
        private.values.insert(
            KEY_DEVICE_AUTHORIZATION_KEY.to_string(),
            serde_json::json!("secret-authorization-key"),
        );

        let encrypted = encrypt_private_config(device_id, &private).expect("encrypt config");
        let config = ClientConfigFile {
            version: CONFIG_VERSION,
            device_id: device_id.to_string(),
            encrypted: Some(encrypted),
        };
        let encoded = serde_json::to_string(&config).expect("encode config");
        let decoded = decrypt_private_config(&config).expect("decrypt config");

        assert!(encoded.contains(device_id));
        assert!(!encoded.contains("secret-device-token"));
        assert!(!encoded.contains("secret-authorization-key"));
        assert!(!encoded.contains(KEY_DEVICE_PROVISIONED));
        assert_eq!(
            decoded.values[KEY_SESSION]["deviceToken"],
            "secret-device-token"
        );
        assert_eq!(decoded.values[KEY_DEVICE_PROVISIONED], true);
        assert_eq!(
            decoded.values[KEY_DEVICE_AUTHORIZATION_KEY],
            "secret-authorization-key"
        );
    }

    #[test]
    fn device_id_change_invalidates_encrypted_payload() {
        let old_id = "0123456789abcdef0123456789abcdef";
        let new_id = "fedcba9876543210fedcba9876543210";
        let encrypted =
            encrypt_private_config(old_id, &PrivateConfig::default()).expect("encrypt old config");
        let mut config = ClientConfigFile {
            version: CONFIG_VERSION,
            device_id: old_id.to_string(),
            encrypted: Some(encrypted),
        };

        config.device_id = new_id.to_string();

        assert!(decrypt_private_config(&config).is_err());
    }
}
