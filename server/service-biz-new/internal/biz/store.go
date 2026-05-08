package biz

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ipamGlobalCIDR        = "10.0.0.0/8"
	ipamSubnetPrefix      = 20
	ipamSubnetSize        = 1 << (32 - ipamSubnetPrefix)
	ipamPoolLowWatermark  = 1000
	ipamMaxOffsetIn10CIDR = 1 << 24
	deviceInviteTTL       = 30 * time.Minute
)

var (
	errBadRequest = errors.New("bad request")
	errNotFound   = errors.New("not found")
	errConflict   = errors.New("conflict")
)

type Store struct {
	mu sync.Mutex

	nextUserID          int
	nextDeviceSeq       int
	nextWorkspaceSeq    int
	nextMemberSeq       int
	nextInviteSeq       int
	nextDeviceInviteSeq int
	nextOwnerSeq        int
	nextOwnerLogSeq     int
	nextZoneSeq         int
	nextRecordSeq       int
	nextSecuritySeq     int
	nextSecurityRuleSeq int
	nextPublicMapSeq    int
	nextConfigSeq       int
	nextIPSubnetSeq     int
	nextIPAddressSeq    int
	nextIPOffset        uint32

	users              map[string]User
	userByEmail        map[string]string
	sessions           map[string]UserSession
	devices            map[string]Device
	deviceOwners       map[string]DeviceOwner
	ownerLogs          map[string]DeviceOwnerChangeLog
	ipamSubnets        map[string]IPAMSubnet
	globalIPs          map[string]GlobalIPAddress
	workspaces         map[string]Workspace
	workspaceMembers   map[string]WorkspaceMember
	workspaceInvites   map[string]WorkspaceInvite
	deviceInvites      map[string]WorkspaceDeviceInvite
	deviceAccessGrants map[string]DeviceAccessGrant
	deviceInviteStore  deviceInviteStore
	workspaceDevices   map[string]WorkspaceDevice
	dnsZones           map[string]WorkspaceDNSZone
	dnsRecords         map[string]WorkspaceDNSRecord
	publicMappings     map[string]PublicDomainMapping
	securityGroups     map[string]SecurityGroup
	securityGroupDevs  map[string]SecurityGroupDevice
	securityGroupRules map[string]SecurityGroupRule
	runtimeStatuses    map[string]DeviceRuntimeStatus
	configVersions     map[string]NetworkConfigVersion
}

func NewStore() *Store {
	return NewStoreWithDeviceInviteStore(newDeviceInviteStoreFromEnv())
}

func NewStoreWithDeviceInviteStore(inviteStore deviceInviteStore) *Store {
	if inviteStore == nil {
		inviteStore = newMemoryDeviceInviteStore()
	}
	store := &Store{
		nextUserID:          1,
		nextDeviceSeq:       1,
		nextWorkspaceSeq:    1,
		nextMemberSeq:       1,
		nextInviteSeq:       1,
		nextDeviceInviteSeq: 1,
		nextOwnerSeq:        1,
		nextOwnerLogSeq:     1,
		nextZoneSeq:         1,
		nextRecordSeq:       1,
		nextSecuritySeq:     1,
		nextSecurityRuleSeq: 1,
		nextPublicMapSeq:    1,
		nextConfigSeq:       1,
		nextIPSubnetSeq:     1,
		nextIPAddressSeq:    1,
		nextIPOffset:        0,
		users:               make(map[string]User),
		userByEmail:         make(map[string]string),
		sessions:            make(map[string]UserSession),
		devices:             make(map[string]Device),
		deviceOwners:        make(map[string]DeviceOwner),
		ownerLogs:           make(map[string]DeviceOwnerChangeLog),
		ipamSubnets:         make(map[string]IPAMSubnet),
		globalIPs:           make(map[string]GlobalIPAddress),
		workspaces:          make(map[string]Workspace),
		workspaceMembers:    make(map[string]WorkspaceMember),
		workspaceInvites:    make(map[string]WorkspaceInvite),
		deviceInvites:       make(map[string]WorkspaceDeviceInvite),
		deviceAccessGrants:  make(map[string]DeviceAccessGrant),
		deviceInviteStore:   inviteStore,
		workspaceDevices:    make(map[string]WorkspaceDevice),
		dnsZones:            make(map[string]WorkspaceDNSZone),
		dnsRecords:          make(map[string]WorkspaceDNSRecord),
		publicMappings:      make(map[string]PublicDomainMapping),
		securityGroups:      make(map[string]SecurityGroup),
		securityGroupDevs:   make(map[string]SecurityGroupDevice),
		securityGroupRules:  make(map[string]SecurityGroupRule),
		runtimeStatuses:     make(map[string]DeviceRuntimeStatus),
		configVersions:      make(map[string]NetworkConfigVersion),
	}
	store.ensureIPPoolLocked(time.Now().Unix())
	return store
}

func (s *Store) RegisterUser(email, password, name string) (AuthResponse, WorkspaceMember, Workspace, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(password) == "" {
		return AuthResponse{}, WorkspaceMember{}, Workspace{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.userByEmail[email]; ok {
		return AuthResponse{}, WorkspaceMember{}, Workspace{}, errConflict
	}
	now := time.Now().Unix()
	user := User{
		UserID:       fmt.Sprintf("user-%06d", s.nextUserID),
		Email:        email,
		Name:         strings.TrimSpace(name),
		PasswordHash: hashPassword(password),
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.nextUserID++
	s.users[user.UserID] = user
	s.userByEmail[email] = user.UserID
	workspace := s.ensureDefaultWorkspaceForUserLocked(user.UserID, now)
	member := s.addWorkspaceMemberLocked(workspace.WorkspaceID, user.UserID, "owner", "active", now)
	session := s.createSessionLocked(user.UserID, now)
	return AuthResponse{User: user, Session: session}, member, workspace, nil
}

func (s *Store) LoginUser(email, password string) (AuthResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.userByEmail[email]
	if !ok {
		return AuthResponse{}, errNotFound
	}
	user := s.users[userID]
	if user.PasswordHash != hashPassword(password) || user.Status != "active" {
		return AuthResponse{}, errBadRequest
	}
	return AuthResponse{User: user, Session: s.createSessionLocked(user.UserID, time.Now().Unix())}, nil
}

func (s *Store) ChangeUserPassword(userID, oldPassword, newPassword string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" || strings.TrimSpace(oldPassword) == "" || strings.TrimSpace(newPassword) == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return errNotFound
	}
	if user.PasswordHash != hashPassword(oldPassword) {
		return errBadRequest
	}
	user.PasswordHash = hashPassword(newPassword)
	user.UpdatedAt = time.Now().Unix()
	s.users[userID] = user
	return nil
}

func (s *Store) RegisterDevice(ownerID, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, WorkspaceDevice, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return Device{}, WorkspaceDevice{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[ownerID]; !ok {
		return Device{}, WorkspaceDevice{}, errNotFound
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		deviceID = fmt.Sprintf("device-%06d", s.nextDeviceSeq)
		s.nextDeviceSeq++
	}
	now := time.Now().Unix()
	if existing, ok := s.devices[deviceID]; ok {
		if existing.OwnerID != ownerID {
			s.transferDeviceOwnerLocked(deviceID, existing.OwnerID, ownerID, "login_switch", now)
			existing.OwnerID = ownerID
			existing.UpdatedAt = now
			s.devices[deviceID] = existing
			defaultWorkspace := s.ensureDefaultWorkspaceForUserLocked(ownerID, now)
			return existing, s.addWorkspaceDeviceLocked(defaultWorkspace.WorkspaceID, deviceID, ownerID, alias, true, now), nil
		}
		return Device{}, WorkspaceDevice{}, errConflict
	}
	ip := s.allocateGlobalIPLocked(deviceID, now)
	device := Device{
		DeviceID:   deviceID,
		OwnerID:    ownerID,
		Name:       strings.TrimSpace(name),
		Platform:   strings.TrimSpace(platform),
		OSName:     strings.TrimSpace(osName),
		OSVersion:  strings.TrimSpace(osVersion),
		Alias:      strings.TrimSpace(alias),
		PublicKey:  strings.TrimSpace(publicKey),
		GlobalIP:   ip,
		GlobalName: sanitizeDNSLabel(deviceID) + ".vlan.com",
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.devices[device.DeviceID] = device
	s.addDeviceOwnerLocked(device.DeviceID, ownerID, now)
	defaultWorkspace := s.ensureDefaultWorkspaceForUserLocked(ownerID, now)
	workspaceDevice := s.addWorkspaceDeviceLocked(defaultWorkspace.WorkspaceID, device.DeviceID, ownerID, alias, true, now)
	s.runtimeStatuses[device.DeviceID] = DeviceRuntimeStatus{DeviceID: device.DeviceID, DeviceEnabled: true, LastReportAt: now}
	return device, workspaceDevice, nil
}

func (s *Store) ListUsers() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.users, func(a, b User) bool { return a.Email < b.Email })
}

func (s *Store) ListDevices(userID string) []Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Device, 0)
	for _, device := range s.devices {
		if userID == "" || device.OwnerID == userID {
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

func (s *Store) ListVisibleDevices(userID string) []Device {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return s.ListDevices("")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	deviceIDs := make(map[string]bool)
	for _, device := range s.devices {
		if device.OwnerID == userID {
			deviceIDs[device.DeviceID] = true
		}
	}
	for _, workspaceDevice := range s.workspaceDevices {
		if s.isWorkspaceMemberLocked(workspaceDevice.WorkspaceID, userID) {
			deviceIDs[workspaceDevice.DeviceID] = true
		}
	}
	for _, grant := range s.deviceAccessGrants {
		if grant.UserID == userID && grant.Status == "active" {
			deviceIDs[grant.DeviceID] = true
		}
	}
	out := make([]Device, 0, len(deviceIDs))
	for deviceID := range deviceIDs {
		if device, ok := s.devices[deviceID]; ok {
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

func (s *Store) ListDeviceOwnerLogs(deviceID string) []DeviceOwnerChangeLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]DeviceOwnerChangeLog, 0)
	for _, log := range s.ownerLogs {
		if deviceID == "" || log.DeviceID == deviceID {
			out = append(out, log)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChangedAt < out[j].ChangedAt })
	return out
}

func (s *Store) ListGlobalIPs() []GlobalIPAddress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.globalIPs, func(a, b GlobalIPAddress) bool { return a.Offset < b.Offset })
}

func (s *Store) ListIPAMSubnets() []IPAMSubnet {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.ipamSubnets, func(a, b IPAMSubnet) bool { return a.StartOffset < b.StartOffset })
}

func (s *Store) CreateWorkspace(ownerUserID, name, code, templateKey string) (Workspace, WorkspaceMember, SecurityGroup, WorkspaceDNSZone, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	name = strings.TrimSpace(name)
	if ownerUserID == "" || name == "" {
		return Workspace{}, WorkspaceMember{}, SecurityGroup{}, WorkspaceDNSZone{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[ownerUserID]; !ok {
		return Workspace{}, WorkspaceMember{}, SecurityGroup{}, WorkspaceDNSZone{}, errNotFound
	}
	now := time.Now().Unix()
	id := fmt.Sprintf("workspace-%06d", s.nextWorkspaceSeq)
	s.nextWorkspaceSeq++
	workspace := Workspace{WorkspaceID: id, OwnerUserID: ownerUserID, Name: name, Code: defaultString(sanitizeDNSLabel(code), sanitizeDNSLabel(name)), TemplateKey: defaultString(templateKey, "custom"), Status: "enabled", CreatedAt: now, UpdatedAt: now}
	s.workspaces[id] = workspace
	member := s.addWorkspaceMemberLocked(id, ownerUserID, "owner", "active", now)
	group := s.addSecurityGroupLocked(id, "默认安全组", "网络默认安全组", "deny", now)
	zone := s.addDNSZoneLocked(id, workspaceZoneName(workspace), false, now)
	return workspace, member, group, zone, nil
}

func (s *Store) ListWorkspaces(userID string) []Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Workspace, 0)
	for _, workspace := range s.workspaces {
		if workspace.Status == "deleted" {
			continue
		}
		if userID == "" || s.isWorkspaceMemberLocked(workspace.WorkspaceID, userID) {
			out = append(out, workspace)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WorkspaceID < out[j].WorkspaceID })
	return out
}

func (s *Store) UpdateWorkspace(workspaceID, name, status string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspace, ok := s.workspaces[workspaceID]
	if !ok {
		return Workspace{}, errNotFound
	}
	if name = strings.TrimSpace(name); name != "" {
		workspace.Name = name
	}
	if status = strings.TrimSpace(status); status != "" {
		workspace.Status = status
	}
	workspace.UpdatedAt = time.Now().Unix()
	s.workspaces[workspaceID] = workspace
	return workspace, nil
}

func (s *Store) UpdateWorkspaceFull(workspaceID, name, code, status string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspace, ok := s.workspaces[workspaceID]
	if !ok {
		return Workspace{}, errNotFound
	}
	if name = strings.TrimSpace(name); name != "" {
		workspace.Name = name
	}
	if code = sanitizeDNSLabel(code); code != "" {
		for _, existing := range s.workspaces {
			if existing.WorkspaceID != workspaceID && existing.OwnerUserID == workspace.OwnerUserID && existing.Code == code && existing.Status != "deleted" {
				return Workspace{}, errConflict
			}
		}
		workspace.Code = code
		workspace.TemplateKey = defaultString(workspace.TemplateKey, code)
	}
	if status = strings.TrimSpace(status); status != "" {
		workspace.Status = status
	}
	workspace.UpdatedAt = time.Now().Unix()
	s.workspaces[workspaceID] = workspace
	return workspace, nil
}

func (s *Store) InviteWorkspaceMember(workspaceID, inviterUserID, inviteeEmail, role string) (WorkspaceInvite, error) {
	inviteeEmail = strings.ToLower(strings.TrimSpace(inviteeEmail))
	if inviteeEmail == "" {
		return WorkspaceInvite{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[workspaceID]; !ok {
		return WorkspaceInvite{}, errNotFound
	}
	inviteeUserID := s.userByEmail[inviteeEmail]
	now := time.Now().Unix()
	invite := WorkspaceInvite{
		InviteID:      fmt.Sprintf("invite-%06d", s.nextInviteSeq),
		WorkspaceID:   workspaceID,
		InviterUserID: inviterUserID,
		InviteeUserID: inviteeUserID,
		InviteeEmail:  inviteeEmail,
		Role:          defaultString(role, "member"),
		Status:        "pending",
		CreatedAt:     now,
	}
	s.nextInviteSeq++
	s.workspaceInvites[invite.InviteID] = invite
	return invite, nil
}

func (s *Store) AcceptWorkspaceInvite(inviteID, userID string) (WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	invite, ok := s.workspaceInvites[inviteID]
	if !ok || invite.Status != "pending" {
		return WorkspaceMember{}, errNotFound
	}
	if invite.InviteeUserID != "" && invite.InviteeUserID != userID {
		return WorkspaceMember{}, errBadRequest
	}
	if _, ok := s.users[userID]; !ok {
		return WorkspaceMember{}, errNotFound
	}
	now := time.Now().Unix()
	invite.InviteeUserID = userID
	invite.Status = "accepted"
	invite.HandledAt = now
	s.workspaceInvites[inviteID] = invite
	return s.addWorkspaceMemberLocked(invite.WorkspaceID, userID, invite.Role, "active", now), nil
}

func (s *Store) ListWorkspaceMembers(workspaceID string) []WorkspaceMember {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WorkspaceMember, 0)
	for _, member := range s.workspaceMembers {
		if member.WorkspaceID == workspaceID {
			member.Email = s.users[member.UserID].Email
			out = append(out, member)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out
}

func (s *Store) CreateDeviceInvite(inviterUserID string, ttlSeconds int64) (WorkspaceDeviceInvite, error) {
	inviterUserID = strings.TrimSpace(inviterUserID)
	if inviterUserID == "" {
		return WorkspaceDeviceInvite{}, errBadRequest
	}
	s.mu.Lock()
	if _, ok := s.users[inviterUserID]; !ok {
		s.mu.Unlock()
		return WorkspaceDeviceInvite{}, errNotFound
	}
	now := time.Now().Unix()
	code, err := secureInviteCode()
	if err != nil {
		s.mu.Unlock()
		return WorkspaceDeviceInvite{}, err
	}
	ttl := deviceInviteTTL
	if ttlSeconds > 0 && ttlSeconds < int64(deviceInviteTTL.Seconds()) {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	invite := WorkspaceDeviceInvite{
		InviteID:      fmt.Sprintf("device-invite-%06d", s.nextDeviceInviteSeq),
		InviterUserID: inviterUserID,
		InviteCode:    code,
		Status:        "pending",
		CreatedAt:     now,
		ExpiresAt:     now + int64(ttl.Seconds()),
	}
	s.nextDeviceInviteSeq++
	s.mu.Unlock()
	if err := s.deviceInviteStore.Save(invite, ttl); err != nil {
		return WorkspaceDeviceInvite{}, err
	}
	return invite, nil
}

func secureInviteCode() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(raw[:])), nil
}

func (s *Store) AcceptDeviceInvite(inviteCode, deviceID, actorUserID string) (DeviceAccessGrant, WorkspaceDeviceInvite, error) {
	inviteCode = strings.ToUpper(strings.TrimSpace(inviteCode))
	deviceID = strings.TrimSpace(deviceID)
	actorUserID = strings.TrimSpace(actorUserID)
	if inviteCode == "" || deviceID == "" || actorUserID == "" {
		return DeviceAccessGrant{}, WorkspaceDeviceInvite{}, errBadRequest
	}
	invite, err := s.deviceInviteStore.Consume(inviteCode)
	if err != nil {
		return DeviceAccessGrant{}, WorkspaceDeviceInvite{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if invite.WorkspaceID != "" {
		return DeviceAccessGrant{}, WorkspaceDeviceInvite{}, errBadRequest
	}
	device, ok := s.devices[deviceID]
	if !ok {
		return DeviceAccessGrant{}, WorkspaceDeviceInvite{}, errNotFound
	}
	if device.OwnerID != actorUserID {
		return DeviceAccessGrant{}, WorkspaceDeviceInvite{}, errBadRequest
	}
	if invite.InviterUserID == actorUserID {
		return DeviceAccessGrant{}, WorkspaceDeviceInvite{}, errConflict
	}
	key := invite.InviterUserID + "|" + deviceID
	if existing, ok := s.deviceAccessGrants[key]; ok && existing.Status == "active" {
		return DeviceAccessGrant{}, WorkspaceDeviceInvite{}, errConflict
	}
	now := time.Now().Unix()
	grant := DeviceAccessGrant{
		GrantID:    fmt.Sprintf("device-grant-%06d", s.nextDeviceInviteSeq),
		DeviceID:   deviceID,
		UserID:     invite.InviterUserID,
		GrantedBy:  actorUserID,
		InviteCode: inviteCode,
		Status:     "active",
		CreatedAt:  now,
	}
	s.deviceAccessGrants[key] = grant
	invite.Status = "accepted"
	invite.AcceptedDeviceID = deviceID
	invite.AcceptedUserID = actorUserID
	invite.AcceptedAt = now
	s.deviceInvites[inviteCode] = invite
	return grant, invite, nil
}

func (s *Store) AddWorkspaceDevice(workspaceID, deviceID, actorUserID, alias string, enabled bool) (WorkspaceDevice, error) {
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return WorkspaceDevice{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[workspaceID]; !ok {
		return WorkspaceDevice{}, errNotFound
	}
	device, ok := s.devices[deviceID]
	if !ok {
		return WorkspaceDevice{}, errNotFound
	}
	if !s.canUserAddWorkspaceDeviceLocked(actorUserID, device.DeviceID, device.OwnerID) {
		return WorkspaceDevice{}, errBadRequest
	}
	if _, ok := s.workspaceDevices[workspaceID+"|"+deviceID]; ok {
		return WorkspaceDevice{}, errConflict
	}
	return s.addWorkspaceDeviceLocked(workspaceID, deviceID, device.OwnerID, alias, enabled, time.Now().Unix()), nil
}

func (s *Store) UpdateWorkspaceDeviceAlias(workspaceID, deviceID, alias string) (WorkspaceDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := workspaceID + "|" + deviceID
	workspaceDevice, ok := s.workspaceDevices[key]
	if !ok {
		return WorkspaceDevice{}, errNotFound
	}
	now := time.Now().Unix()
	workspaceDevice.Alias = strings.TrimSpace(alias)
	workspaceDevice.UpdatedAt = now
	s.workspaceDevices[key] = workspaceDevice
	return workspaceDevice, nil
}

func (s *Store) RemoveWorkspaceDevice(workspaceID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := workspaceID + "|" + deviceID
	if _, ok := s.workspaceDevices[key]; !ok {
		return errNotFound
	}
	delete(s.workspaceDevices, key)
	return nil
}

func (s *Store) ListWorkspaceDevices(workspaceID string) []WorkspaceDevice {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WorkspaceDevice, 0)
	for _, device := range s.workspaceDevices {
		if workspaceID == "" || device.WorkspaceID == workspaceID {
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

func (s *Store) AddDNSZone(workspaceID, zoneName string, exposeGlobal bool) (WorkspaceDNSZone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[workspaceID]; !ok {
		return WorkspaceDNSZone{}, errNotFound
	}
	return s.addDNSZoneLocked(workspaceID, zoneName, exposeGlobal, time.Now().Unix()), nil
}

func (s *Store) UpdateDNSZone(workspaceID, zoneID, zoneName string, exposeGlobal bool) (WorkspaceDNSZone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	zone, ok := s.dnsZones[zoneID]
	if !ok || zone.WorkspaceID != workspaceID {
		return WorkspaceDNSZone{}, errNotFound
	}
	if zoneName = strings.TrimSpace(zoneName); zoneName != "" {
		zone.ZoneName = zoneName
	}
	zone.ExposeGlobal = exposeGlobal
	s.dnsZones[zoneID] = zone
	return zone, nil
}

func (s *Store) DeleteDNSZone(workspaceID, zoneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	zone, ok := s.dnsZones[zoneID]
	if !ok || zone.WorkspaceID != workspaceID {
		return errNotFound
	}
	delete(s.dnsZones, zoneID)
	for recordID, record := range s.dnsRecords {
		if record.ZoneID == zoneID {
			delete(s.dnsRecords, recordID)
		}
	}
	return nil
}

func (s *Store) AddDNSRecord(workspaceID, zoneID, name, recordType, targetDeviceID, targetIP, cname, port string, ttl int) (WorkspaceDNSRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return WorkspaceDNSRecord{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	zone, ok := s.dnsZones[zoneID]
	if !ok || zone.WorkspaceID != workspaceID {
		return WorkspaceDNSRecord{}, errNotFound
	}
	if targetDeviceID != "" {
		device, ok := s.devices[targetDeviceID]
		if !ok {
			return WorkspaceDNSRecord{}, errNotFound
		}
		if targetIP == "" {
			targetIP = device.GlobalIP
		}
	}
	now := time.Now().Unix()
	record := WorkspaceDNSRecord{
		RecordID:       fmt.Sprintf("dns-%06d", s.nextRecordSeq),
		ZoneID:         zoneID,
		WorkspaceID:    workspaceID,
		Name:           name,
		FQDN:           sanitizeDNSLabel(name) + "." + strings.TrimSuffix(zone.ZoneName, "."),
		RecordType:     defaultString(recordType, "A"),
		TargetDeviceID: strings.TrimSpace(targetDeviceID),
		TargetIP:       strings.TrimSpace(targetIP),
		CNAME:          strings.TrimSpace(cname),
		Port:           strings.TrimSpace(port),
		TTL:            defaultInt(ttl, 60),
		Status:         "active",
		CreatedAt:      now,
	}
	s.nextRecordSeq++
	s.dnsRecords[record.RecordID] = record
	return record, nil
}

func (s *Store) UpdateDNSRecord(workspaceID, recordID, name, recordType, targetDeviceID, targetIP, cname, port string, ttl int) (WorkspaceDNSRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return WorkspaceDNSRecord{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.dnsRecords[recordID]
	if !ok || record.WorkspaceID != workspaceID {
		return WorkspaceDNSRecord{}, errNotFound
	}
	zone, ok := s.dnsZones[record.ZoneID]
	if !ok {
		return WorkspaceDNSRecord{}, errNotFound
	}
	if targetDeviceID != "" {
		device, ok := s.devices[targetDeviceID]
		if !ok {
			return WorkspaceDNSRecord{}, errNotFound
		}
		if targetIP == "" {
			targetIP = device.GlobalIP
		}
	}
	record.Name = name
	record.FQDN = sanitizeDNSLabel(name) + "." + strings.TrimSuffix(zone.ZoneName, ".")
	record.RecordType = defaultString(recordType, "A")
	record.TargetDeviceID = strings.TrimSpace(targetDeviceID)
	record.TargetIP = strings.TrimSpace(targetIP)
	record.CNAME = strings.TrimSpace(cname)
	record.Port = strings.TrimSpace(port)
	record.TTL = defaultInt(ttl, 60)
	s.dnsRecords[recordID] = record
	return record, nil
}

func (s *Store) DeleteDNSRecord(workspaceID, recordID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.dnsRecords[recordID]
	if !ok || record.WorkspaceID != workspaceID {
		return errNotFound
	}
	delete(s.dnsRecords, recordID)
	return nil
}

func (s *Store) ListDNSZones(workspaceID string) []WorkspaceDNSZone {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WorkspaceDNSZone, 0)
	for _, zone := range s.dnsZones {
		if workspaceID == "" || zone.WorkspaceID == workspaceID {
			out = append(out, zone)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ZoneName < out[j].ZoneName })
	return out
}

func (s *Store) ListDNSRecords(workspaceID string) []WorkspaceDNSRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WorkspaceDNSRecord, 0)
	for _, record := range s.dnsRecords {
		if workspaceID == "" || record.WorkspaceID == workspaceID {
			out = append(out, record)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FQDN < out[j].FQDN })
	return out
}

func (s *Store) ListPublicMappings(workspaceID string) []PublicDomainMapping {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]PublicDomainMapping, 0)
	for _, mapping := range s.publicMappings {
		if workspaceID == "" || mapping.WorkspaceID == workspaceID {
			out = append(out, mapping)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublicDomain < out[j].PublicDomain })
	return out
}

func (s *Store) UpsertPublicMapping(mappingID, workspaceID, alias, publicDomain, sourceRecord, deviceID, protocol, port, externalPort, status string) (PublicDomainMapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[workspaceID]; !ok {
		return PublicDomainMapping{}, errNotFound
	}
	deviceID = strings.TrimSpace(deviceID)
	device, ok := s.devices[deviceID]
	if !ok {
		return PublicDomainMapping{}, errNotFound
	}
	now := time.Now().Unix()
	mapping := PublicDomainMapping{}
	if mappingID != "" {
		existing, ok := s.publicMappings[mappingID]
		if !ok || existing.WorkspaceID != workspaceID {
			return PublicDomainMapping{}, errNotFound
		}
		mapping = existing
	} else {
		mapping.MappingID = fmt.Sprintf("pub-%06d", s.nextPublicMapSeq)
		mapping.WorkspaceID = workspaceID
		mapping.CreatedAt = now
		s.nextPublicMapSeq++
	}
	mapping.Alias = sanitizeDNSLabel(alias)
	mapping.PublicDomain = strings.TrimSpace(publicDomain)
	mapping.SourceRecord = defaultString(strings.TrimSpace(sourceRecord), defaultString(device.Alias, device.DeviceID))
	mapping.DeviceID = deviceID
	mapping.Protocol = defaultString(protocol, "HTTP")
	mapping.ExternalPort = strings.TrimSpace(externalPort)
	mapping.Port = defaultString(strings.TrimSpace(port), mapping.ExternalPort)
	mapping.Status = defaultString(status, "enabled")
	mapping.UpdatedAt = now
	s.publicMappings[mapping.MappingID] = mapping
	return mapping, nil
}

func (s *Store) DeletePublicMapping(workspaceID, mappingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mapping, ok := s.publicMappings[mappingID]
	if !ok || mapping.WorkspaceID != workspaceID {
		return errNotFound
	}
	delete(s.publicMappings, mappingID)
	return nil
}

func (s *Store) CreateSecurityGroup(workspaceID, name, description, defaultPolicy string) (SecurityGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[workspaceID]; !ok {
		return SecurityGroup{}, errNotFound
	}
	return s.addSecurityGroupLocked(workspaceID, name, description, defaultString(defaultPolicy, "deny"), time.Now().Unix()), nil
}

func (s *Store) DeleteSecurityGroup(workspaceID, securityGroupID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	group, ok := s.securityGroups[securityGroupID]
	if !ok || group.WorkspaceID != workspaceID {
		return errNotFound
	}
	delete(s.securityGroups, securityGroupID)
	for ruleID, rule := range s.securityGroupRules {
		if rule.SecurityGroupID == securityGroupID {
			delete(s.securityGroupRules, ruleID)
		}
	}
	for key, device := range s.securityGroupDevs {
		if device.SecurityGroupID == securityGroupID {
			delete(s.securityGroupDevs, key)
		}
	}
	return nil
}

func (s *Store) AddSecurityGroupRule(securityGroupID, direction, action, protocol, peerType, peerValue, description string, priority, portFrom, portTo int, enabled bool) (SecurityGroupRule, error) {
	if securityGroupID == "" || direction == "" || action == "" {
		return SecurityGroupRule{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.securityGroups[securityGroupID]; !ok {
		return SecurityGroupRule{}, errNotFound
	}
	rule := SecurityGroupRule{
		RuleID:          fmt.Sprintf("sgr-%06d", s.nextSecurityRuleSeq),
		SecurityGroupID: securityGroupID,
		Direction:       direction,
		Priority:        defaultInt(priority, 100),
		Action:          action,
		Protocol:        defaultString(protocol, "all"),
		PortFrom:        portFrom,
		PortTo:          portTo,
		PeerType:        defaultString(peerType, "workspace"),
		PeerValue:       strings.TrimSpace(peerValue),
		Description:     strings.TrimSpace(description),
		Enabled:         enabled,
		CreatedAt:       time.Now().Unix(),
	}
	s.nextSecurityRuleSeq++
	s.securityGroupRules[rule.RuleID] = rule
	return rule, nil
}

func (s *Store) UpdateSecurityGroupRule(ruleID, direction, action, protocol, peerType, peerValue, description string, priority, portFrom, portTo int, enabled bool) (SecurityGroupRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, ok := s.securityGroupRules[ruleID]
	if !ok {
		return SecurityGroupRule{}, errNotFound
	}
	rule.Direction = defaultString(direction, rule.Direction)
	rule.Priority = defaultInt(priority, rule.Priority)
	rule.Action = defaultString(action, rule.Action)
	rule.Protocol = defaultString(protocol, rule.Protocol)
	rule.PortFrom = portFrom
	rule.PortTo = portTo
	rule.PeerType = defaultString(peerType, rule.PeerType)
	rule.PeerValue = strings.TrimSpace(peerValue)
	rule.Description = strings.TrimSpace(description)
	rule.Enabled = enabled
	s.securityGroupRules[ruleID] = rule
	return rule, nil
}

func (s *Store) DeleteSecurityGroupRule(ruleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.securityGroupRules[ruleID]; !ok {
		return errNotFound
	}
	delete(s.securityGroupRules, ruleID)
	return nil
}

func (s *Store) ListSecurityGroups(workspaceID string) []SecurityGroup {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SecurityGroup, 0)
	for _, group := range s.securityGroups {
		if workspaceID == "" || group.WorkspaceID == workspaceID {
			out = append(out, group)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SecurityGroupID < out[j].SecurityGroupID })
	return out
}

func (s *Store) ListSecurityGroupRules(securityGroupID string) []SecurityGroupRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SecurityGroupRule, 0)
	for _, rule := range s.securityGroupRules {
		if securityGroupID == "" || rule.SecurityGroupID == securityGroupID {
			out = append(out, rule)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Priority < out[j].Priority })
	return out
}

func (s *Store) GlobalDNS() []GlobalDNSRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]GlobalDNSRecord, 0, len(s.devices))
	for _, device := range s.devices {
		out = append(out, GlobalDNSRecord{Name: device.GlobalName, DeviceID: device.DeviceID, Value: device.GlobalIP})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) NetworkConfig(workspaceID, deviceID string) (NetworkConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.devices[deviceID]
	if !ok {
		return NetworkConfig{}, errNotFound
	}
	if _, ok := s.workspaces[workspaceID]; !ok {
		return NetworkConfig{}, errNotFound
	}
	peers := make([]Device, 0)
	for _, membership := range s.workspaceDevices {
		if membership.WorkspaceID == workspaceID && membership.Enabled && membership.Status == "active" && membership.DeviceID != deviceID {
			peers = append(peers, s.devices[membership.DeviceID])
		}
	}
	groups := make([]SecurityGroup, 0)
	for _, group := range s.securityGroups {
		if group.WorkspaceID == workspaceID && group.Status == "active" {
			groups = append(groups, group)
		}
	}
	rules := make([]SecurityGroupRule, 0)
	for _, rule := range s.securityGroupRules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	return NetworkConfig{
		WorkspaceID:    workspaceID,
		DeviceID:       deviceID,
		GlobalIP:       device.GlobalIP,
		GlobalName:     device.GlobalName,
		Peers:          peers,
		SecurityGroups: groups,
		Rules:          rules,
		DNSZones:       s.listDNSZonesLocked(workspaceID),
		DNSRecords:     s.listDNSRecordsLocked(workspaceID),
	}, nil
}

func (s *Store) createSessionLocked(userID string, now int64) UserSession {
	session := UserSession{SessionID: fmt.Sprintf("session-%s-%d", userID, now), UserID: userID, Token: fmt.Sprintf("token-%s-%d", userID, now), CreatedAt: now, ExpiresAt: now + 86400*30}
	s.sessions[session.Token] = session
	return session
}

func (s *Store) ensureDefaultWorkspaceForUserLocked(userID string, now int64) Workspace {
	id := "default-" + userID
	if workspace, ok := s.workspaces[id]; ok {
		return workspace
	}
	workspace := Workspace{WorkspaceID: id, OwnerUserID: userID, Name: "默认网络", Code: "default", TemplateKey: "default", Status: "enabled", Default: true, CreatedAt: now, UpdatedAt: now}
	s.workspaces[id] = workspace
	s.addSecurityGroupLocked(id, "默认安全组", "默认网络安全组", "deny", now)
	s.addDNSZoneLocked(id, workspaceZoneName(workspace), false, now)
	return workspace
}

func (s *Store) addWorkspaceMemberLocked(workspaceID, userID, role, status string, now int64) WorkspaceMember {
	member := WorkspaceMember{MemberID: fmt.Sprintf("member-%06d", s.nextMemberSeq), WorkspaceID: workspaceID, UserID: userID, Role: defaultString(role, "member"), Status: defaultString(status, "active"), JoinedAt: now}
	s.nextMemberSeq++
	s.workspaceMembers[workspaceID+"|"+userID] = member
	return member
}

func (s *Store) addDeviceOwnerLocked(deviceID, userID string, now int64) DeviceOwner {
	owner := DeviceOwner{OwnerRecordID: fmt.Sprintf("owner-%06d", s.nextOwnerSeq), DeviceID: deviceID, UserID: userID, Status: "active", BoundAt: now}
	s.nextOwnerSeq++
	s.deviceOwners[deviceID] = owner
	return owner
}

func (s *Store) transferDeviceOwnerLocked(deviceID, fromUserID, toUserID, reason string, now int64) {
	owner := s.deviceOwners[deviceID]
	owner.Status = "transferred"
	owner.UnboundAt = now
	s.deviceOwners[deviceID+"|old|"+fmt.Sprint(now)] = owner
	s.addDeviceOwnerLocked(deviceID, toUserID, now)
	log := DeviceOwnerChangeLog{LogID: fmt.Sprintf("owner-log-%06d", s.nextOwnerLogSeq), DeviceID: deviceID, FromUserID: fromUserID, ToUserID: toUserID, Reason: reason, ChangedAt: now}
	s.nextOwnerLogSeq++
	s.ownerLogs[log.LogID] = log
}

func (s *Store) addWorkspaceDeviceLocked(workspaceID, deviceID, ownerUserID, alias string, enabled bool, now int64) WorkspaceDevice {
	status := "active"
	if !enabled {
		status = "disabled"
	}
	workspaceDevice := WorkspaceDevice{WorkspaceDeviceID: fmt.Sprintf("workspace-device-%s-%s", workspaceID, deviceID), WorkspaceID: workspaceID, DeviceID: deviceID, OwnerUserID: ownerUserID, Alias: strings.TrimSpace(alias), Enabled: enabled, Status: status, CreatedAt: now, UpdatedAt: now}
	s.workspaceDevices[workspaceID+"|"+deviceID] = workspaceDevice
	return workspaceDevice
}

func (s *Store) addDNSZoneLocked(workspaceID, zoneName string, exposeGlobal bool, now int64) WorkspaceDNSZone {
	zoneName = strings.TrimSpace(zoneName)
	if zoneName == "" {
		zoneName = "default.lan"
	}
	zone := WorkspaceDNSZone{ZoneID: fmt.Sprintf("zone-%06d", s.nextZoneSeq), WorkspaceID: workspaceID, ZoneName: zoneName, ExposeGlobal: exposeGlobal, Status: "active", CreatedAt: now}
	s.nextZoneSeq++
	s.dnsZones[zone.ZoneID] = zone
	return zone
}

func (s *Store) addSecurityGroupLocked(workspaceID, name, description, defaultPolicy string, now int64) SecurityGroup {
	group := SecurityGroup{SecurityGroupID: fmt.Sprintf("sg-%06d", s.nextSecuritySeq), WorkspaceID: workspaceID, Name: defaultString(name, "默认安全组"), Description: description, DefaultPolicy: defaultString(defaultPolicy, "deny"), Status: "active", CreatedAt: now}
	s.nextSecuritySeq++
	s.securityGroups[group.SecurityGroupID] = group
	return group
}

func (s *Store) isWorkspaceMemberLocked(workspaceID, userID string) bool {
	member, ok := s.workspaceMembers[workspaceID+"|"+userID]
	return ok && member.Status == "active"
}

func (s *Store) canUserAddWorkspaceDeviceLocked(userID, deviceID, ownerUserID string) bool {
	if userID == "" {
		return false
	}
	if ownerUserID == userID {
		return true
	}
	if grant, ok := s.deviceAccessGrants[userID+"|"+deviceID]; ok && grant.Status == "active" {
		return true
	}
	for _, workspaceDevice := range s.workspaceDevices {
		if workspaceDevice.DeviceID == deviceID && s.isWorkspaceMemberLocked(workspaceDevice.WorkspaceID, userID) {
			return true
		}
	}
	return false
}

func (s *Store) allocateGlobalIPLocked(deviceID string, now int64) string {
	s.ensureIPPoolLocked(now)
	var selected GlobalIPAddress
	for _, address := range s.globalIPs {
		if address.Status != "available" {
			continue
		}
		if selected.IP == "" || address.Offset < selected.Offset {
			selected = address
		}
	}
	if selected.IP == "" {
		return ""
	}
	selected.DeviceID = deviceID
	selected.Status = "assigned"
	selected.AssignedAt = now
	selected.ReleasedAt = 0
	s.globalIPs[selected.IP] = selected
	s.ensureIPPoolLocked(now)
	return selected.IP
}

func (s *Store) ensureIPPoolLocked(now int64) {
	for s.availableIPCountLocked() < ipamPoolLowWatermark {
		if !s.generateNextIPSubnetLocked(now) {
			return
		}
	}
}

func (s *Store) availableIPCountLocked() int {
	count := 0
	for _, address := range s.globalIPs {
		if address.Status == "available" {
			count++
		}
	}
	return count
}

func (s *Store) generateNextIPSubnetLocked(now int64) bool {
	startOffset := s.nextIPOffset
	if startOffset >= ipamMaxOffsetIn10CIDR {
		return false
	}
	endOffset := startOffset + ipamSubnetSize - 1
	if endOffset >= ipamMaxOffsetIn10CIDR {
		endOffset = ipamMaxOffsetIn10CIDR - 1
	}
	baseIP := ipFrom10Offset(startOffset)
	subnetID := fmt.Sprintf("ipam-subnet-%06d", s.nextIPSubnetSeq)
	s.nextIPSubnetSeq++
	subnet := IPAMSubnet{
		SubnetID:     subnetID,
		CIDRBlock:    netip.PrefixFrom(netip.MustParseAddr(baseIP), ipamSubnetPrefix).String(),
		BaseIP:       baseIP,
		PrefixLength: ipamSubnetPrefix,
		StartOffset:  startOffset,
		EndOffset:    endOffset,
		Status:       "active",
		CreatedAt:    now,
	}
	for offset := startOffset; offset <= endOffset; offset++ {
		if offset == startOffset || offset == endOffset {
			continue
		}
		ip := ipFrom10Offset(offset)
		s.globalIPs[ip] = GlobalIPAddress{
			AddressID: fmt.Sprintf("ipam-address-%06d", s.nextIPAddressSeq),
			SubnetID:  subnetID,
			IP:        ip,
			CIDRBlock: subnet.CIDRBlock,
			Offset:    offset,
			Status:    "available",
			CreatedAt: now,
		}
		s.nextIPAddressSeq++
		subnet.GeneratedCapacity++
	}
	s.ipamSubnets[subnetID] = subnet
	s.nextIPOffset = endOffset + 1
	return true
}

func ipFrom10Offset(offset uint32) string {
	return netip.AddrFrom4([4]byte{10, byte(offset >> 16), byte(offset >> 8), byte(offset)}).String()
}

func (s *Store) listDNSZonesLocked(workspaceID string) []WorkspaceDNSZone {
	out := make([]WorkspaceDNSZone, 0)
	for _, zone := range s.dnsZones {
		if zone.WorkspaceID == workspaceID {
			out = append(out, zone)
		}
	}
	return out
}

func (s *Store) listDNSRecordsLocked(workspaceID string) []WorkspaceDNSRecord {
	out := make([]WorkspaceDNSRecord, 0)
	for _, record := range s.dnsRecords {
		if record.WorkspaceID == workspaceID {
			out = append(out, record)
		}
	}
	return out
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte("slan:" + password))
	return hex.EncodeToString(sum[:])
}

func workspaceZoneName(workspace Workspace) string {
	if workspace.Code == "default" {
		return "default.lan"
	}
	return sanitizeDNSLabel(workspace.Code) + ".internal"
}

func sortedValues[T any](values map[string]T, less func(a, b T) bool) []T {
	out := make([]T, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func sanitizeDNSLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "x"
	}
	return value
}
