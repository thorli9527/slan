package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

// registerPublicRegistrationRoutes registers install-time lifecycle endpoints.
func registerPublicRegistrationRoutes(api *gin.RouterGroup, deps routerDeps) {
	api.POST("/devices/install-register", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RegisterDeviceRequest) (dto.Device, error) {
		return deps.Device.InstallRegister(req, c.ClientIP())
	}))
}

// registerRegistrationRoutes 注册设备与节点生命周期入口。
func registerRegistrationRoutes(protected *gin.RouterGroup, deps routerDeps) {
	devices := protected.Group("/devices")
	// POST /devices/register
	//
	// 为当前登录用户注册一台设备（需要鉴权）。
	//
	// 请求：RegisterDeviceRequest
	// - deviceId：可选；客户端可传入本地稳定 ID（服务端会校验不可使用保留前缀/保留值）
	// - name/platform/deviceVersion/publicKey：必填（用于设备展示与密钥标识）
	// - countryCode：可选
	//
	// 响应：201 Device
	// - deviceId：服务端最终采用的设备 ID
	// - mqtt：当 MQTT 启用时会下发 MQTTCredential(brokerUrl/clientId/username/password/topicPrefix/expiresAt)
	//
	// 副作用：
	// - 会确保设备在“当前活跃网络”具备基础编排（例如默认 network 里的成员/挂载准备）
	//
	// 典型错误：
	// - 401 UNAUTHORIZED：token 无效
	// - 400 INVALID_ARGUMENT：缺字段、deviceId 不合法
	devices.POST("/register", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RegisterDeviceRequest) (dto.Device, error) {
		rc := currentRouteContext(c)
		return deps.Device.Register(rc.user(), req)
	}))
	// GET /devices
	//
	// 返回当前用户可见/拥有的设备列表（需要鉴权）。
	//
	// 响应：200 ItemsResponse<Device>
	// - items：设备列表
	//
	// 说明：
	// - 列表会结合“用户可见网络”补齐网络维度视图（例如 active network 的 virtual IP / membership status）
	devices.GET("", respondWithItems(func(c *gin.Context) ([]dto.Device, error) {
		rc := currentRouteContext(c)
		return deps.Device.ListByUser(rc.user())
	}))
	// PUT /devices/:deviceId/networks/:networkId/state
	//
	// 设备运行态上报（需要鉴权）。
	//
	// 用途：
	// - 客户端周期性上报 controlReachable / networkOnline / tunnelUp / virtualIp 等信号
	// - 控制面用于设备在线态、网络启用态、诊断与质量统计
	//
	// 请求：
	// - path: deviceId / networkId
	// - body: DeviceNetworkStateRequest
	//
	// 响应：200 DeviceNetworkState
	//
	// 典型错误：
	// - 403 FORBIDDEN：deviceId 非当前用户所有，或 device 不是该 network 的 active member
	devices.PUT("/:deviceId/networks/:networkId/state", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.DeviceNetworkStateRequest) (dto.DeviceNetworkState, error) {
		rc := currentRouteContext(c)
		return deps.Device.SetDeviceNetworkState(rc.user(), c.Param("deviceId"), c.Param("networkId"), req)
	}))

	nodes := protected.Group("/nodes")
	// POST /nodes/register
	//
	// 为当前用户某台设备注册一个通信节点（需要鉴权）。
	//
	// 说明：
	// - node 是设备上的一个运行实例标识（用于控制通道、自身 peer 身份、端点上报等）
	//
	// 请求：RegisterNodeRequest(deviceId, nodeId, nodePublicKey, capabilities...)
	// 响应：201 Node(nodeId, deviceId, networkIds...)
	//
	// 典型错误：
	// - 403 FORBIDDEN：deviceId 不属于当前用户
	// - 400 INVALID_ARGUMENT：缺字段
	nodes.POST("/register", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RegisterNodeRequest) (dto.Node, error) {
		rc := currentRouteContext(c)
		return deps.Node.Register(rc.user(), req)
	}))
}
