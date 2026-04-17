/// NAT 探测器能力定义。
pub trait NatDetector: Send + Sync {
    /// 基于 STUN 服务器列表探测 NAT 类型。
    fn detect(&self, stun_servers: &[String]) -> Result<crate::NatType, String>;
}
