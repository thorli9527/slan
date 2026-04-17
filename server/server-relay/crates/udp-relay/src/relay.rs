use relay_core::{RelayError, RelaySession, RelayTicket, TicketValidator};
use session::SessionStore;

use crate::{ForwardedPacket, RelayPacket};

/// UDP relay 实例。
pub struct UdpRelay<S, V>
where
    S: SessionStore,
    V: TicketValidator,
{
    store: S,
    validator: V,
}

impl<S, V> UdpRelay<S, V>
where
    S: SessionStore,
    V: TicketValidator,
{
    /// 创建新的 UDP relay。
    pub fn new(store: S, validator: V) -> Self {
        Self { store, validator }
    }

    /// 直接挂载一个已存在的 relay session。
    pub fn attach(&self, session: RelaySession) -> Result<(), RelayError> {
        self.store.create(session)
    }

    /// 使用票据校验后创建 relay session。
    pub fn attach_with_ticket(
        &self,
        ticket: &RelayTicket,
        session: RelaySession,
    ) -> Result<(), RelayError> {
        self.validator.validate(ticket)?;
        self.attach(session)
    }

    /// 查询指定 relay session。
    pub fn session(&self, session_id: &str) -> Result<RelaySession, RelayError> {
        self.store
            .get(session_id)
            .ok_or(RelayError::SessionNotFound)
    }

    /// 删除指定 relay session。
    pub fn detach(&self, session_id: &str) -> Result<(), RelayError> {
        self.store.remove(session_id)
    }

    /// 将一个设备发来的 UDP 负载转发给会话另一端。
    pub fn forward(&self, packet: RelayPacket) -> Result<ForwardedPacket, RelayError> {
        if packet.payload.is_empty() {
            return Err(RelayError::EmptyPayload);
        }

        let session = self.session(&packet.session_id)?;
        let peer = session.peer_of(&packet.from_device_id)?;
        Ok(ForwardedPacket {
            session_id: packet.session_id,
            to_device_id: peer.to_string(),
            payload: packet.payload,
        })
    }
}
