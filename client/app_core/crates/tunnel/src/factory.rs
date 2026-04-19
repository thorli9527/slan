use crate::TunnelBackend;

/// 平台 tunnel backend 工厂。
pub trait TunnelBackendFactory: Send + Sync {
    type Backend: TunnelBackend;

    fn create(&self) -> Result<Self::Backend, String>;
}
