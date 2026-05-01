use thiserror::Error;

pub type ClientCoreResult<T> = Result<T, ClientCoreError>;

#[derive(Debug, Error)]
pub enum ClientCoreError {
    #[error("invalid command: {0}")]
    InvalidCommand(String),
    #[error("platform error: {0}")]
    Platform(String),
    #[error("control plane error: {0}")]
    ControlPlane(String),
}
