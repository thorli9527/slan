package repository

var (
	_ UserRepository            = (*GormStore)(nil)
	_ UserSessionRepository     = (*GormStore)(nil)
	_ UserAliasRepository       = (*GormStore)(nil)
	_ DeviceRepository          = (*GormStore)(nil)
	_ NetworkRepository         = (*GormStore)(nil)
	_ OperatorRepository        = (*GormStore)(nil)
	_ OperatorSessionRepository = (*GormStore)(nil)
	_ AuditRepository           = (*GormStore)(nil)
	_ OpsRepository             = (*GormStore)(nil)
)
