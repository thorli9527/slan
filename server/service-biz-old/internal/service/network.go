package service

import "github.com/slan/server/server-biz/api/dto"

// Network defines orchestration capabilities for logical networks, subnets, and attachments.
//
// Concepts:
// - Network: a logical private network (tenant-scoped by user membership).
// - Subnet: an address range under a network (one is usually default).
// - Member: a device's membership in a network (role/status).
// - Attachment: a device's attachment to a subnet (virtual IP, remark, status).
//
// Most operations require the caller's userID and enforce access rules:
// - Network owners can manage join keys, invites, member status, and policies.
// - Device owners can join/activate networks for their devices and update their own attachment remarks.
//
// Implementations should return service-level errors to keep HTTP error mapping stable.
type Network interface {
	// Create creates a new network and its default subnet, returning the created Network DTO.
	//
	// Typical errors:
	// - ErrInvalidArgument: invalid CIDR/name.
	// - ErrForbidden: plan restriction or policy.
	Create(userID string, req dto.CreateNetworkRequest) (dto.Network, error)

	// Home returns an aggregated "network home" view for fast client bootstrapping.
	Home(userID string) (dto.NetworkHome, error)

	// List returns networks visible to the user (owned or joined).
	List(userID string) ([]dto.Network, error)

	// Update updates mutable network fields (name, description, etc.).
	//
	// Typical errors:
	// - ErrForbidden: caller is not permitted to mutate the network.
	Update(userID, networkID string, req dto.UpdateNetworkRequest) (dto.Network, error)

	// UpdateDNS updates the network DNS configuration.
	UpdateDNS(userID, networkID string, req dto.UpdateNetworkDNSRequest) (dto.NetworkDetail, error)

	// UpdateJoinKey rotates/updates the network invitation join key.
	UpdateJoinKey(userID, networkID string, req dto.UpdateNetworkJoinKeyRequest) (dto.NetworkDetail, error)

	// Get returns the detailed network view, including subnets, members, routes, DNS, relay regions, and policies.
	Get(userID, networkID string) (dto.NetworkDetail, error)

	// Join ensures the given device becomes a member of the network and has an active attachment in the default subnet.
	//
	// Output:
	// - NetworkJoinResult(member, attachment, and optionally networkMap/dns/routes views depending on implementation).
	Join(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error)

	// JoinByKey joins a network using an invitation code (join key).
	//
	// Typical errors:
	// - ErrUnauthorized/ErrForbidden: invalid or unauthorized join key.
	JoinByKey(userID string, req dto.JoinNetworkByKeyRequest) (dto.NetworkJoinResult, error)

	// Switch updates the user's active network selection and returns the join result view for the selected network.
	Switch(userID, networkID string, req dto.SwitchNetworkRequest) (dto.NetworkJoinResult, error)

	// Activate marks the network as enabled for the given device and returns the runtime configuration needed by the client
	// (virtual IP, routes, DNS, relay candidates, peer snapshot, etc.).
	//
	// Notes:
	// - Implementations should ensure the device has an active membership and an active attachment with an assigned virtual IP.
	Activate(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error)

	// RelayCandidates returns relay candidate information for the given device in a network.
	//
	// Output:
	// - dto.NetworkMap subset or full map depending on implementation; typically contains relay regions/candidates.
	RelayCandidates(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkMap, error)

	// Deactivate marks the network as disabled for the given device.
	//
	// Side effects:
	// - Implementations may also update device runtime state (networkOnline/tunnelUp) and trigger control-plane cleanup.
	Deactivate(userID, networkID string, req dto.DeactivateNetworkRequest) error

	// PlanStatus returns the user's plan status, used by clients/console for gating features.
	PlanStatus(userID string) (dto.PlanStatus, error)

	// ListMembers returns network members visible to the caller.
	ListMembers(userID, networkID string) ([]dto.NetworkMember, error)

	// UpdateMemberStatus changes a member's status (e.g., active/suspended).
	UpdateMemberStatus(userID, networkID, memberID string, req dto.UpdateNetworkMemberStatusRequest) (dto.NetworkMember, error)

	// InviteMember creates an invitation for a user/device to join the network.
	InviteMember(userID, networkID string, req dto.InviteNetworkMemberRequest) (dto.NetworkMember, error)

	// ListAssignments returns the aggregated assignment view (member + attachments + remarks + virtual IP).
	ListAssignments(userID, networkID string) ([]dto.NetworkAssignment, error)

	// NetworkQuality returns aggregated network quality metrics.
	//
	// Input:
	// - hours: window size (caller-controlled clamped by HTTP layer).
	NetworkQuality(userID, networkID string, hours int) (dto.OpsNetworkQuality, error)

	ListRelayPolicyTemplates(userID, networkID string) ([]dto.RelayPolicyTemplate, error)
	CreateRelayPolicyTemplate(userID, networkID string, req dto.RelayPolicyTemplateRequest) (dto.RelayPolicyTemplate, error)
	UpdateRelayPolicyTemplate(userID, networkID, templateID string, req dto.RelayPolicyTemplateRequest) (dto.RelayPolicyTemplate, error)
	DeleteRelayPolicyTemplate(userID, networkID, templateID string) error
	RecordRelayPolicyDispatch(userID, networkID string, req dto.OpsRelayDataPlanePolicyRequest, resp dto.OpsRelayDataPlanePolicyResponse) error
	ListEnabledNetworkDeviceIDs(networkID string) ([]string, error)

	// ListSubnets returns all subnets in a network.
	ListSubnets(userID, networkID string) ([]dto.Subnet, error)

	// UpdateAttachmentIP sets or reserves a specific virtual IP for an attachment.
	UpdateAttachmentIP(userID, networkID, attachmentID string, req dto.UpdateAttachmentIPRequest) (dto.SubnetAttachment, error)

	// UpdateAttachmentRemark sets a human-friendly remark/alias for the attachment.
	UpdateAttachmentRemark(userID, networkID, attachmentID string, req dto.UpdateAttachmentRemarkRequest) (dto.NetworkAssignment, error)

	// UpdateAttachmentStatus changes attachment availability (e.g., active/suspended).
	UpdateAttachmentStatus(userID, networkID, attachmentID string, req dto.UpdateAttachmentStatusRequest) (dto.NetworkAssignment, error)
}

// Allocator defines server-side DHCP-style virtual IP allocation.
//
// Implementations must guarantee:
// - Uniqueness within a subnet/network for active attachments.
// - Idempotency for Allocate/Reserve per attachmentID when called repeatedly.
// - Safe release when attachments are removed/deactivated.
type Allocator interface {
	// Allocate chooses an available virtual IP for the attachment and returns it.
	//
	// Inputs:
	// - networkID/subnetID: scope
	// - attachmentID/deviceID: identity for idempotency/ownership checks
	Allocate(networkID, subnetID, attachmentID, deviceID string) (string, error)

	// Reserve tries to reserve a specific IP for the attachment, returning the final reserved IP.
	// Implementations may return a different IP when the requested one is unavailable and fallback is allowed.
	Reserve(networkID, subnetID, attachmentID, deviceID, ip string) (string, error)

	// Release releases any reserved/assigned IP for the given attachmentID.
	Release(attachmentID string) error
}
