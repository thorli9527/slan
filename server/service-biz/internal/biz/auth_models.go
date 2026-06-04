package biz

// User 是控制台用户/客户账号的核心身份实体。
type User struct {
	// UserID 是服务端生成的用户唯一 ID。
	UserID string `json:"userId"`
	// Email 是登录账号和客户识别主键。
	Email string `json:"email"`
	// Name 是用户显示名称。
	Name string `json:"name,omitempty"`
	// PasswordHash 仅服务端保存，不返回给外部接口。
	PasswordHash string `json:"-"`
	// Status 表示账号状态，例如 active/disabled。
	Status string `json:"status"`
	// CreatedAt 是创建时间，Unix 秒。
	CreatedAt int64 `json:"createdAt"`
	// UpdatedAt 是最近更新时间，Unix 秒。
	UpdatedAt int64 `json:"updatedAt"`
}

// UserSession 是用户登录后的访问会话。
type UserSession struct {
	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`
	Token     string `json:"token"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
}

// LoginFailure 记录登录失败计数，用于限流和临时封禁。
type LoginFailure struct {
	Key           string `json:"key"`
	FailedCount   int    `json:"failedCount"`
	FirstFailedAt int64  `json:"firstFailedAt"`
	LastFailedAt  int64  `json:"lastFailedAt"`
	BlockedUntil  int64  `json:"blockedUntil,omitempty"`
}

// AuditEvent 是服务端业务动作的审计记录。
type AuditEvent struct {
	EventID      string            `json:"eventId"`
	ActorType    string            `json:"actorType"`
	ActorID      string            `json:"actorId,omitempty"`
	ActorEmail   string            `json:"actorEmail,omitempty"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resourceType,omitempty"`
	ResourceID   string            `json:"resourceId,omitempty"`
	Status       string            `json:"status"`
	RemoteIP     string            `json:"remoteIp,omitempty"`
	Details      map[string]string `json:"details,omitempty"`
	CreatedAt    int64             `json:"createdAt"`
}

// AuditEventFilter 是审计查询条件。
type AuditEventFilter struct {
	ActorType    string
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	Status       string
	Limit        int
}

// AuthResponse 是登录、注册、续期接口返回的认证信息。
type AuthResponse struct {
	User    User        `json:"user"`
	Session UserSession `json:"session"`
}

// ConsoleLoginKey 是桌面/移动客户端发起控制台登录时的一次性登录凭据。
type ConsoleLoginKey struct {
	LoginKey   string `json:"loginKey"`
	UserID     string `json:"userId"`
	DeviceID   string `json:"deviceId,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	ConsumedAt int64  `json:"consumedAt,omitempty"`
	Status     string `json:"status"`
}

// UserAlias 是一个用户给另一个用户邮箱配置的本地显示别名。
type UserAlias struct {
	OwnerUserID string `json:"ownerUserId"`
	Email       string `json:"email"`
	Alias       string `json:"alias"`
	UpdatedAt   int64  `json:"updatedAt"`
}
