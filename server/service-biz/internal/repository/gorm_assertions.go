package repository

var (
	_ CustomerRepository        = (*GormStore)(nil)
	_ DeviceRepository          = (*GormStore)(nil)
	_ NetworkRepository         = (*GormStore)(nil)
	_ OperatorRepository        = (*GormStore)(nil)
	_ OperatorSessionRepository = (*GormStore)(nil)
	_ AuditRepository           = (*GormStore)(nil)
	_ OpsNodeRepository         = (*GormStore)(nil)
)
