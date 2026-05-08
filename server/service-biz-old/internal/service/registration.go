package service

import "github.com/slan/server/server-biz/api/dto"

// Device defines the device lifecycle surface.
//
// Device is an end-user owned entity representing a physical/virtual machine running the client.
// A device can have one or more nodes (runtime instances) and can be attached to one or more networks.
//
// Notes:
// - Most APIs are user-scoped (require a userID from the HTTP auth middleware).
// - Register may also provision/repair default network membership/attachments depending on implementation.
type Device interface {
	// Register creates or updates a device record for the given user.
	//
	// Input:
	// - userID: current authenticated user.
	// - req: RegisterDeviceRequest(name/platform/publicKey required; deviceId optional)
	//
	// Output:
	// - dto.Device with computed views such as networkIds and optional mqtt credential (if MQTT is enabled).
	//
	// Side effects (implementation dependent):
	// - Persists the device record (upsert).
	// - Ensures device is provisioned in the user's active network.
	// - May allocate/repair attachment virtual IPs.
	//
	// Typical errors:
	// - ErrUnauthorized: userID invalid.
	// - ErrInvalidArgument: missing/invalid fields.
	Register(userID string, req dto.RegisterDeviceRequest) (dto.Device, error)

	// InstallRegister creates or updates an unbound physical device record.
	//
	// Caller:
	// - Client installer / local service startup before user login.
	//
	// Notes:
	// - No user auth is required.
	// - The device is stored with an empty userID until browser login binds the
	//   same stable deviceId to the authenticated user.
	InstallRegister(req dto.RegisterDeviceRequest, clientIP string) (dto.Device, error)

	// ListByUser returns devices visible to the given user.
	//
	// Output:
	// - A slice of dto.Device; each device may include network-scoped views (membership status, current virtual IP, etc.).
	ListByUser(userID string) ([]dto.Device, error)

	// SetDeviceNetworkState upserts the runtime state reported by a device for a given network.
	//
	// Use case:
	// - The client periodically reports controlReachable/networkOnline/tunnelUp/virtualIp.
	//
	// Typical errors:
	// - ErrForbidden: device does not belong to userID or is not an active member in the network.
	// - ErrNotFound: device/network not found.
	SetDeviceNetworkState(userID, deviceID, networkID string, req dto.DeviceNetworkStateRequest) (dto.DeviceNetworkState, error)

	// MarkMQTTReachable marks a device as control-channel reachable after the broker accepts its MQTT credential.
	//
	// Caller:
	// - MQTT auth callback handler (e.g., BifroMQ auth provider endpoint).
	//
	// Typical errors:
	// - ErrNotFound: device does not exist.
	MarkMQTTReachable(deviceID string) error
}

// Node defines the node lifecycle surface.
//
// Node is a runtime instance of a device (e.g., a desktop service, a mobile session, or another process).
// Nodes are used by the control plane to identify peers, endpoints, and connection state.
type Node interface {
	// Register creates or updates a node record under the given user.
	//
	// Input:
	// - userID: current authenticated user.
	// - req: RegisterNodeRequest(deviceId/nodeId/nodePublicKey required; capabilities optional)
	//
	// Output:
	// - dto.Node
	//
	// Typical errors:
	// - ErrForbidden: deviceId is owned by another user.
	// - ErrInvalidArgument: missing/invalid fields.
	Register(userID string, req dto.RegisterNodeRequest) (dto.Node, error)
}
