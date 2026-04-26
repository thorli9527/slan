package repo

// ControlOutboundMessage 记录服务端下行到客户端的控制消息。
type ControlOutboundMessage struct {
	MessageID     string `gorm:"column:message_id;primaryKey"`
	TargetUserID  string `gorm:"column:target_user_id;index;not null;default:''"`
	TargetNodeID  string `gorm:"column:target_node_id;index;not null;default:''"`
	NetworkID     string `gorm:"column:network_id;index;not null;default:''"`
	MessageType   string `gorm:"column:message_type;index;not null"`
	RequestID     string `gorm:"column:request_id;not null;default:''"`
	PayloadJSON   string `gorm:"column:payload_json;type:text;not null"`
	AttemptCount  int    `gorm:"column:attempt_count;not null;default:0"`
	Status        string `gorm:"column:status;index;not null"`
	LastAttemptAt int64  `gorm:"column:last_attempt_at;not null;default:0"`
	// AckedAt 是旧版 ACK 流程遗留列；当前不再要求客户端回执。
	AckedAt   int64 `gorm:"column:acked_at;not null;default:0"`
	CreatedAt int64 `gorm:"column:created_at;not null"`
	UpdatedAt int64 `gorm:"column:updated_at;not null"`
}

func (ControlOutboundMessage) TableName() string { return "control_outbound_messages" }

// ControlMessageHistory 归档超过重试上限或已处理完毕的下行消息。
type ControlMessageHistory struct {
	HistoryID    uint64 `gorm:"column:history_id;primaryKey;autoIncrement"`
	MessageID    string `gorm:"column:message_id;index;not null"`
	TargetUserID string `gorm:"column:target_user_id;index;not null;default:''"`
	TargetNodeID string `gorm:"column:target_node_id;index;not null;default:''"`
	NetworkID    string `gorm:"column:network_id;index;not null;default:''"`
	MessageType  string `gorm:"column:message_type;index;not null"`
	RequestID    string `gorm:"column:request_id;not null;default:''"`
	PayloadJSON  string `gorm:"column:payload_json;type:text;not null"`
	AttemptCount int    `gorm:"column:attempt_count;not null;default:0"`
	FinalStatus  string `gorm:"column:final_status;index;not null"`
	// AckedAt 是旧版 ACK 流程遗留列；当前不再要求客户端回执。
	AckedAt       int64  `gorm:"column:acked_at;not null;default:0"`
	ArchivedAt    int64  `gorm:"column:archived_at;not null"`
	FailureReason string `gorm:"column:failure_reason;type:text;not null;default:''"`
}

func (ControlMessageHistory) TableName() string { return "control_message_history" }
