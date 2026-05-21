use thiserror::Error;

/// client-core 公共 Result 类型。
pub type ClientCoreResult<T> = Result<T, ClientCoreError>;

/// client-core 对外暴露的错误分类。
#[derive(Debug, Error)]
pub enum ClientCoreError {
    /// UI 或外部调用传入了无效命令。
    #[error("invalid command: {0}")]
    InvalidCommand(String),
    /// 平台层虚拟网卡、路由、DNS 或数据面操作失败。
    #[error("platform error: {0}")]
    Platform(String),
    /// 控制面登录、配置同步或服务端调用失败。
    #[error("control plane error: {0}")]
    ControlPlane(String),
}
