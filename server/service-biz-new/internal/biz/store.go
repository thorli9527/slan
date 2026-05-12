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
	errBadRequest   = errors.New("bad request")
	errUnauthorized = errors.New("unauthorized")
	errNotFound     = errors.New("not found")
	errConflict     = errors.New("conflict")
)

type Store struct {
	mu sync.Mutex

	nextUserID          int
	nextDeviceSeq       int
	nextNetworkSeq      int
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
	nextOperatorSeq     int
	nextRelayNodeSeq    int
	nextProductSeq      int
	nextOrderSeq        int
	nextRenewalSeq      int

	users              map[string]User
	userByEmail        map[string]string
	userAliases        map[string]UserAlias
	sessions           map[string]UserSession
	operators          map[string]OperatorUser
	operatorByEmail    map[string]string
	operatorSessions   map[string]OperatorSession
	devices            map[string]Device
	deviceOwners       map[string]DeviceOwner
	ownerLogs          map[string]DeviceOwnerChangeLog
	ipamSubnets        map[string]IPAMSubnet
	globalIPs          map[string]GlobalIPAddress
	networks           map[string]Network
	deviceInvites      map[string]DeviceInvite
	deviceAccessGrants map[string]DeviceAccessGrant
	deviceInviteStore  deviceInviteStore
	loginCallbacks     map[string]DeviceLoginCallback
	authCallbackStore  authCallbackStore
	networkDevices     map[string]NetworkDevice
	dnsZones           map[string]NetworkDNSZone
	dnsRecords         map[string]NetworkDNSRecord
	publicMappings     map[string]PublicDomainMapping
	securityGroups     map[string]SecurityGroup
	securityGroupRules map[string]SecurityGroupRule
	runtimeStatuses    map[string]DeviceRuntimeStatus
	configVersions     map[string]NetworkConfigVersion
	customerPlans      map[string]CustomerPlanAssignment
	customerProfiles   map[string]CustomerProfile
	opsPlans           map[string]OpsPlan
	products           map[string]Product
	orders             map[string]Order
	renewals           map[string]Renewal
	relayNodes         map[string]OpsRelayNode
}

func NewStore() *Store {
	return NewStoreWithDeviceInviteStore(newDeviceInviteStoreFromEnv())
}

func NewStoreWithDeviceInviteStore(inviteStore deviceInviteStore) *Store {
	if inviteStore == nil {
		inviteStore = newMemoryDeviceInviteStore()
	}
	authCallbackStore, _ := inviteStore.(authCallbackStore)
	if authCallbackStore == nil {
		authCallbackStore = newMemoryDeviceInviteStore()
	}
	store := &Store{
		nextUserID:          1,
		nextDeviceSeq:       1,
		nextNetworkSeq:      1,
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
		nextOperatorSeq:     1,
		nextRelayNodeSeq:    1,
		nextProductSeq:      1,
		nextOrderSeq:        1,
		nextRenewalSeq:      1,
		users:               make(map[string]User),
		userByEmail:         make(map[string]string),
		userAliases:         make(map[string]UserAlias),
		sessions:            make(map[string]UserSession),
		operators:           make(map[string]OperatorUser),
		operatorByEmail:     make(map[string]string),
		operatorSessions:    make(map[string]OperatorSession),
		devices:             make(map[string]Device),
		deviceOwners:        make(map[string]DeviceOwner),
		ownerLogs:           make(map[string]DeviceOwnerChangeLog),
		ipamSubnets:         make(map[string]IPAMSubnet),
		globalIPs:           make(map[string]GlobalIPAddress),
		networks:            make(map[string]Network),
		deviceInvites:       make(map[string]DeviceInvite),
		deviceAccessGrants:  make(map[string]DeviceAccessGrant),
		deviceInviteStore:   inviteStore,
		loginCallbacks:      make(map[string]DeviceLoginCallback),
		authCallbackStore:   authCallbackStore,
		networkDevices:      make(map[string]NetworkDevice),
		dnsZones:            make(map[string]NetworkDNSZone),
		dnsRecords:          make(map[string]NetworkDNSRecord),
		publicMappings:      make(map[string]PublicDomainMapping),
		securityGroups:      make(map[string]SecurityGroup),
		securityGroupRules:  make(map[string]SecurityGroupRule),
		runtimeStatuses:     make(map[string]DeviceRuntimeStatus),
		configVersions:      make(map[string]NetworkConfigVersion),
		customerPlans:       make(map[string]CustomerPlanAssignment),
		customerProfiles:    make(map[string]CustomerProfile),
		opsPlans:            make(map[string]OpsPlan),
		products:            make(map[string]Product),
		orders:              make(map[string]Order),
		renewals:            make(map[string]Renewal),
		relayNodes:          make(map[string]OpsRelayNode),
	}
	store.ensureIPPoolLocked(time.Now().Unix())
	store.seedOpsDefaultsLocked(time.Now().Unix())
	return store
}

func (s *Store) RegisterUser(email, password, name string) (AuthResponse, Network, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(password) == "" {
		return AuthResponse{}, Network{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.userByEmail[email]; ok {
		return AuthResponse{}, Network{}, errConflict
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
	network := s.ensureDefaultNetworkForUserLocked(user.UserID, now)
	session := s.createSessionLocked(user.UserID, now)
	return AuthResponse{User: user, Session: session}, network, nil
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

func (s *Store) AuthByToken(token string) (AuthResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return AuthResponse{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		if session.Token != token {
			continue
		}
		if session.ExpiresAt < time.Now().Unix() {
			return AuthResponse{}, errNotFound
		}
		user, ok := s.users[session.UserID]
		if !ok || user.Status != "active" {
			return AuthResponse{}, errNotFound
		}
		return AuthResponse{User: user, Session: session}, nil
	}
	return AuthResponse{}, errNotFound
}

func (s *Store) CreateDeviceLoginCallback(callbackID, deviceID, platform string, ttl time.Duration) (DeviceLoginCallback, error) {
	callbackID = strings.TrimSpace(callbackID)
	if callbackID == "" {
		generated, err := secureCallbackID()
		if err != nil {
			return DeviceLoginCallback{}, err
		}
		callbackID = generated
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	now := time.Now().Unix()
	callback := DeviceLoginCallback{
		CallbackID: callbackID,
		DeviceID:   strings.TrimSpace(deviceID),
		Platform:   strings.TrimSpace(platform),
		Status:     "pending",
		CreatedAt:  now,
		ExpiresAt:  now + int64(ttl.Seconds()),
	}
	if err := s.authCallbackStore.SaveCallback(callback, ttl, true); err != nil {
		return DeviceLoginCallback{}, err
	}
	s.mu.Lock()
	s.loginCallbacks[callback.CallbackID] = callback
	s.mu.Unlock()
	return callback, nil
}

func (s *Store) CompleteDeviceLoginCallback(callbackID, accessToken, deviceID, action string) (DeviceLoginCallback, error) {
	callbackID = strings.TrimSpace(callbackID)
	if callbackID == "" {
		return DeviceLoginCallback{}, errBadRequest
	}
	auth, err := s.AuthByToken(accessToken)
	if err != nil {
		return DeviceLoginCallback{}, err
	}
	callback, err := s.authCallbackStore.LoadCallback(callbackID)
	if err != nil {
		now := time.Now().Unix()
		callback = DeviceLoginCallback{CallbackID: callbackID, Status: "pending", CreatedAt: now, ExpiresAt: now + int64((10 * time.Minute).Seconds())}
	}
	if callback.Status == "completed" {
		return DeviceLoginCallback{}, errConflict
	}
	if callback.ExpiresAt < time.Now().Unix() {
		return DeviceLoginCallback{}, errNotFound
	}
	if strings.TrimSpace(deviceID) == "" {
		deviceID = callback.DeviceID
	}
	if strings.TrimSpace(deviceID) != "" {
		s.bindExistingDeviceToUser(deviceID, auth.User.UserID)
	}
	refreshToken := auth.Session.Token
	payload := AuthCallbackPayload{
		CallbackID:   callbackID,
		AccessToken:  auth.Session.Token,
		RefreshToken: &refreshToken,
		UserID:       auth.User.UserID,
		UserLabel:    defaultString(auth.User.Email, auth.User.UserID),
		DeviceID:     optionalStringPtr(deviceID),
		ExpiresIn:    uint64(maxInt64(auth.Session.ExpiresAt-time.Now().Unix(), 0)),
		Action:       defaultString(action, "login"),
	}
	callback.Status = "completed"
	callback.UserID = auth.User.UserID
	callback.DeviceID = strings.TrimSpace(deviceID)
	callback.CompletedAt = time.Now().Unix()
	callback.Payload = &payload
	ttl := time.Duration(maxInt64(callback.ExpiresAt-time.Now().Unix(), 60)) * time.Second
	if err := s.authCallbackStore.SaveCallback(callback, ttl, false); err != nil {
		return DeviceLoginCallback{}, err
	}
	s.mu.Lock()
	s.loginCallbacks[callback.CallbackID] = callback
	s.mu.Unlock()
	return callback, nil
}

func (s *Store) DeviceLoginCallbackStatus(callbackID string) (DeviceLoginCallback, bool, error) {
	callbackID = strings.TrimSpace(callbackID)
	if callbackID == "" {
		return DeviceLoginCallback{}, false, errBadRequest
	}
	callback, err := s.authCallbackStore.LoadCallback(callbackID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return DeviceLoginCallback{CallbackID: callbackID, Status: "pending"}, false, nil
		}
		return DeviceLoginCallback{}, false, err
	}
	return callback, callback.Status == "completed" && callback.Payload != nil, nil
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

func (s *Store) RegisterDevice(ownerID, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, NetworkDevice, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return Device{}, NetworkDevice{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[ownerID]; !ok {
		return Device{}, NetworkDevice{}, errNotFound
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
			defaultNetwork := s.ensureDefaultNetworkForUserLocked(ownerID, now)
			return existing, s.addNetworkDeviceLocked(defaultNetwork.NetworkID, deviceID, ownerID, alias, true, now), nil
		}
		return Device{}, NetworkDevice{}, errConflict
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
	defaultNetwork := s.ensureDefaultNetworkForUserLocked(ownerID, now)
	networkDevice := s.addNetworkDeviceLocked(defaultNetwork.NetworkID, device.DeviceID, ownerID, alias, true, now)
	s.runtimeStatuses[device.DeviceID] = DeviceRuntimeStatus{DeviceID: device.DeviceID, DeviceEnabled: true, LastReportAt: now}
	return device, networkDevice, nil
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
			out = append(out, s.deviceWithOwnerEmailLocked(device))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

func (s *Store) RenewDevice(deviceID, userID string, networkEnabled bool, rxBytesTotal, txBytesTotal uint64) (Device, []NetworkConfig, error) {
	deviceID = strings.TrimSpace(deviceID)
	userID = strings.TrimSpace(userID)
	if deviceID == "" || userID == "" {
		return Device{}, nil, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.devices[deviceID]
	if !ok {
		return Device{}, nil, errNotFound
	}
	if !s.canUserSeeDeviceLocked(userID, deviceID, device.OwnerID) {
		return Device{}, nil, errNotFound
	}
	now := time.Now().Unix()
	device.Status = "active"
	device.UpdatedAt = now
	s.devices[deviceID] = device
	status := s.runtimeStatuses[deviceID]
	status.DeviceID = deviceID
	status.HeartbeatOnline = true
	status.NetworkEnabled = networkEnabled
	status.DeviceEnabled = true
	status.RxBytesTotal = rxBytesTotal
	status.TxBytesTotal = txBytesTotal
	status.LastSeenAt = now
	status.LastReportAt = now
	s.runtimeStatuses[deviceID] = status
	configs, err := s.networkConfigsForDeviceLocked(deviceID)
	if err != nil {
		return Device{}, nil, err
	}
	return s.deviceWithOwnerEmailLocked(device), configs, nil
}

func (s *Store) GetDevice(deviceID string) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.devices[strings.TrimSpace(deviceID)]
	if !ok {
		return Device{}, errNotFound
	}
	return s.deviceWithOwnerEmailLocked(device), nil
}

func (s *Store) DeviceQuota(userID string) (DeviceQuota, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DeviceQuota{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[userID]; !ok {
		return DeviceQuota{}, errNotFound
	}
	return s.deviceQuotaLocked(userID), nil
}

func (s *Store) RemoveVisibleDevice(deviceID, actorUserID string) error {
	deviceID = strings.TrimSpace(deviceID)
	actorUserID = strings.TrimSpace(actorUserID)
	if deviceID == "" || actorUserID == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.devices[deviceID]
	if !ok {
		return errNotFound
	}
	if device.OwnerID != actorUserID {
		key := actorUserID + "|" + deviceID
		if grant, ok := s.deviceAccessGrants[key]; ok && grant.Status == "active" {
			grant.Status = "revoked"
			s.deviceAccessGrants[key] = grant
			return nil
		}
		return errNotFound
	}
	return s.removeDeviceLocked(deviceID)
}

func (s *Store) removeDeviceLocked(deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return errBadRequest
	}
	if _, ok := s.devices[deviceID]; !ok {
		return errNotFound
	}
	delete(s.devices, deviceID)
	delete(s.runtimeStatuses, deviceID)
	for key, owner := range s.deviceOwners {
		if owner.DeviceID == deviceID {
			delete(s.deviceOwners, key)
		}
	}
	for key, networkDevice := range s.networkDevices {
		if networkDevice.DeviceID == deviceID {
			delete(s.networkDevices, key)
		}
	}
	for key, grant := range s.deviceAccessGrants {
		if grant.DeviceID == deviceID {
			delete(s.deviceAccessGrants, key)
		}
	}
	for ipID, ip := range s.globalIPs {
		if ip.DeviceID == deviceID {
			ip.DeviceID = ""
			ip.Status = "available"
			ip.ReleasedAt = time.Now().Unix()
			s.globalIPs[ipID] = ip
		}
	}
	return nil
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
	for _, grant := range s.deviceAccessGrants {
		if grant.UserID == userID && grant.Status == "active" {
			deviceIDs[grant.DeviceID] = true
		}
	}
	out := make([]Device, 0, len(deviceIDs))
	for deviceID := range deviceIDs {
		if device, ok := s.devices[deviceID]; ok {
			out = append(out, s.deviceWithOwnerEmailLocked(device))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

func (s *Store) UpdateDeviceAlias(deviceID, actorUserID, alias string) (Device, error) {
	deviceID = strings.TrimSpace(deviceID)
	actorUserID = strings.TrimSpace(actorUserID)
	alias = strings.TrimSpace(alias)
	if deviceID == "" || actorUserID == "" || alias == "" {
		return Device{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.devices[deviceID]
	if !ok {
		return Device{}, errNotFound
	}
	if !s.canUserSeeDeviceLocked(actorUserID, deviceID, device.OwnerID) {
		return Device{}, errNotFound
	}
	device.Alias = alias
	device.UpdatedAt = time.Now().Unix()
	s.devices[deviceID] = device
	return s.deviceWithOwnerEmailLocked(device), nil
}

func (s *Store) ListUserAliases(ownerUserID string) []UserAlias {
	ownerUserID = strings.TrimSpace(ownerUserID)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]UserAlias, 0)
	for _, alias := range s.userAliases {
		if ownerUserID == "" || alias.OwnerUserID == ownerUserID {
			out = append(out, alias)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	return out
}

func (s *Store) UpsertUserAlias(ownerUserID, email, alias string) (UserAlias, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	email = strings.ToLower(strings.TrimSpace(email))
	alias = strings.TrimSpace(alias)
	if ownerUserID == "" || email == "" || alias == "" {
		return UserAlias{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[ownerUserID]; !ok {
		return UserAlias{}, errNotFound
	}
	item := UserAlias{OwnerUserID: ownerUserID, Email: email, Alias: alias, UpdatedAt: time.Now().Unix()}
	s.userAliases[ownerUserID+"|"+email] = item
	return item, nil
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

func (s *Store) CreateNetwork(ownerUserID, name, code, templateKey string) (Network, SecurityGroup, NetworkDNSZone, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	name = strings.TrimSpace(name)
	if ownerUserID == "" || name == "" {
		return Network{}, SecurityGroup{}, NetworkDNSZone{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[ownerUserID]; !ok {
		return Network{}, SecurityGroup{}, NetworkDNSZone{}, errNotFound
	}
	now := time.Now().Unix()
	id := fmt.Sprintf("network-%06d", s.nextNetworkSeq)
	s.nextNetworkSeq++
	network := Network{NetworkID: id, OwnerUserID: ownerUserID, Name: name, Code: defaultString(sanitizeDNSLabel(code), sanitizeDNSLabel(name)), TemplateKey: defaultString(templateKey, "custom"), Status: "enabled", CreatedAt: now, UpdatedAt: now}
	s.networks[id] = network
	group := s.addSecurityGroupLocked(id, "默认安全组", "网络默认安全组", "deny", now)
	zone := s.addDNSZoneLocked(id, networkZoneName(network), false, now)
	return network, group, zone, nil
}

func (s *Store) ListNetworks(userID string) []Network {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Network, 0)
	for _, network := range s.networks {
		if network.Status == "deleted" {
			continue
		}
		if userID == "" || network.OwnerUserID == userID {
			out = append(out, network)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NetworkID < out[j].NetworkID })
	return out
}

func (s *Store) UpdateNetwork(networkID, name, status string) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	network, ok := s.networks[networkID]
	if !ok {
		return Network{}, errNotFound
	}
	if name = strings.TrimSpace(name); name != "" {
		network.Name = name
	}
	if status = strings.TrimSpace(status); status != "" {
		network.Status = status
	}
	network.UpdatedAt = time.Now().Unix()
	s.networks[networkID] = network
	return network, nil
}

func (s *Store) UpdateNetworkFull(networkID, name, code, status string) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	network, ok := s.networks[networkID]
	if !ok {
		return Network{}, errNotFound
	}
	if name = strings.TrimSpace(name); name != "" {
		network.Name = name
	}
	if code = sanitizeDNSLabel(code); code != "" {
		for _, existing := range s.networks {
			if existing.NetworkID != networkID && existing.OwnerUserID == network.OwnerUserID && existing.Code == code && existing.Status != "deleted" {
				return Network{}, errConflict
			}
		}
		network.Code = code
		network.TemplateKey = defaultString(network.TemplateKey, code)
	}
	if status = strings.TrimSpace(status); status != "" {
		network.Status = status
	}
	network.UpdatedAt = time.Now().Unix()
	s.networks[networkID] = network
	return network, nil
}

func (s *Store) CreateDeviceInvite(inviterUserID string, ttlSeconds int64) (DeviceInvite, error) {
	inviterUserID = strings.TrimSpace(inviterUserID)
	if inviterUserID == "" {
		return DeviceInvite{}, errBadRequest
	}
	s.mu.Lock()
	if _, ok := s.users[inviterUserID]; !ok {
		s.mu.Unlock()
		return DeviceInvite{}, errNotFound
	}
	if quota := s.deviceQuotaLocked(inviterUserID); quota.TotalDeviceLimit > 0 && quota.RemainingDevices <= 0 {
		s.mu.Unlock()
		return DeviceInvite{}, errConflict
	}
	now := time.Now().Unix()
	code, err := secureInviteCode()
	if err != nil {
		s.mu.Unlock()
		return DeviceInvite{}, err
	}
	ttl := deviceInviteTTL
	if ttlSeconds > 0 && ttlSeconds < int64(deviceInviteTTL.Seconds()) {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	invite := DeviceInvite{
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
		return DeviceInvite{}, err
	}
	s.mu.Lock()
	s.deviceInvites[invite.InviteCode] = invite
	s.mu.Unlock()
	return invite, nil
}

func (s *Store) ListDeviceInvites(inviterUserID string) []DeviceInvite {
	inviterUserID = strings.TrimSpace(inviterUserID)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]DeviceInvite, 0)
	for _, invite := range s.deviceInvites {
		if inviterUserID == "" || invite.InviterUserID == inviterUserID {
			out = append(out, invite)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func secureInviteCode() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(raw[:])), nil
}

func secureCallbackID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "cb-" + hex.EncodeToString(raw[:]), nil
}

func (s *Store) AcceptDeviceInvite(inviteCode, deviceID, actorUserID string) (DeviceAccessGrant, DeviceInvite, error) {
	inviteCode = strings.ToUpper(strings.TrimSpace(inviteCode))
	deviceID = strings.TrimSpace(deviceID)
	actorUserID = strings.TrimSpace(actorUserID)
	if inviteCode == "" || deviceID == "" || actorUserID == "" {
		return DeviceAccessGrant{}, DeviceInvite{}, errBadRequest
	}
	invite, err := s.deviceInviteStore.Consume(inviteCode)
	if err != nil {
		return DeviceAccessGrant{}, DeviceInvite{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.devices[deviceID]
	if !ok {
		return DeviceAccessGrant{}, DeviceInvite{}, errNotFound
	}
	if device.OwnerID != actorUserID {
		return DeviceAccessGrant{}, DeviceInvite{}, errBadRequest
	}
	if invite.InviterUserID == actorUserID {
		return DeviceAccessGrant{}, DeviceInvite{}, errConflict
	}
	if quota := s.deviceQuotaLocked(invite.InviterUserID); quota.TotalDeviceLimit > 0 && quota.RemainingDevices <= 0 {
		return DeviceAccessGrant{}, DeviceInvite{}, errConflict
	}
	key := invite.InviterUserID + "|" + deviceID
	if existing, ok := s.deviceAccessGrants[key]; ok && existing.Status == "active" {
		return DeviceAccessGrant{}, DeviceInvite{}, errConflict
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

func (s *Store) AddNetworkDevice(networkID, deviceID, actorUserID, alias string, enabled bool) (NetworkDevice, error) {
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return NetworkDevice{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.networks[networkID]; !ok {
		return NetworkDevice{}, errNotFound
	}
	device, ok := s.devices[deviceID]
	if !ok {
		return NetworkDevice{}, errNotFound
	}
	if !s.canUserAddNetworkDeviceLocked(actorUserID, device.DeviceID, device.OwnerID) {
		return NetworkDevice{}, errBadRequest
	}
	if _, ok := s.networkDevices[networkID+"|"+deviceID]; ok {
		return NetworkDevice{}, errConflict
	}
	return s.addNetworkDeviceLocked(networkID, deviceID, device.OwnerID, alias, enabled, time.Now().Unix()), nil
}

func (s *Store) UpdateNetworkDevice(networkID, deviceID, alias string, enabled *bool) (NetworkDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := networkID + "|" + deviceID
	networkDevice, ok := s.networkDevices[key]
	if !ok {
		return NetworkDevice{}, errNotFound
	}
	now := time.Now().Unix()
	networkDevice.Alias = strings.TrimSpace(alias)
	if enabled != nil {
		networkDevice.Enabled = *enabled
		if *enabled {
			networkDevice.Status = "active"
		} else {
			networkDevice.Status = "disabled"
		}
	}
	networkDevice.UpdatedAt = now
	s.networkDevices[key] = networkDevice
	return networkDevice, nil
}

func (s *Store) RemoveNetworkDevice(networkID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := networkID + "|" + deviceID
	if _, ok := s.networkDevices[key]; !ok {
		return errNotFound
	}
	delete(s.networkDevices, key)
	return nil
}

func (s *Store) ListNetworkDevices(networkID string) []NetworkDevice {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]NetworkDevice, 0)
	for _, device := range s.networkDevices {
		if networkID == "" || device.NetworkID == networkID {
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

func (s *Store) AddDNSZone(networkID, zoneName string, exposeGlobal bool) (NetworkDNSZone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.networks[networkID]; !ok {
		return NetworkDNSZone{}, errNotFound
	}
	return s.addDNSZoneLocked(networkID, zoneName, exposeGlobal, time.Now().Unix()), nil
}

func (s *Store) UpdateDNSZone(networkID, zoneID, zoneName string, exposeGlobal bool) (NetworkDNSZone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	zone, ok := s.dnsZones[zoneID]
	if !ok || zone.NetworkID != networkID {
		return NetworkDNSZone{}, errNotFound
	}
	if zoneName = strings.TrimSpace(zoneName); zoneName != "" {
		zone.ZoneName = zoneName
	}
	zone.ExposeGlobal = exposeGlobal
	s.dnsZones[zoneID] = zone
	return zone, nil
}

func (s *Store) DeleteDNSZone(networkID, zoneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	zone, ok := s.dnsZones[zoneID]
	if !ok || zone.NetworkID != networkID {
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

func (s *Store) AddDNSRecord(networkID, zoneID, name, recordType, targetDeviceID, targetIP, cname, port string, ttl int) (NetworkDNSRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return NetworkDNSRecord{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	zone, ok := s.dnsZones[zoneID]
	if !ok || zone.NetworkID != networkID {
		return NetworkDNSRecord{}, errNotFound
	}
	if targetDeviceID != "" {
		device, ok := s.devices[targetDeviceID]
		if !ok {
			return NetworkDNSRecord{}, errNotFound
		}
		if targetIP == "" {
			targetIP = device.GlobalIP
		}
	}
	now := time.Now().Unix()
	record := NetworkDNSRecord{
		RecordID:       fmt.Sprintf("dns-%06d", s.nextRecordSeq),
		ZoneID:         zoneID,
		NetworkID:      networkID,
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

func (s *Store) UpdateDNSRecord(networkID, recordID, name, recordType, targetDeviceID, targetIP, cname, port string, ttl int) (NetworkDNSRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return NetworkDNSRecord{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.dnsRecords[recordID]
	if !ok || record.NetworkID != networkID {
		return NetworkDNSRecord{}, errNotFound
	}
	zone, ok := s.dnsZones[record.ZoneID]
	if !ok {
		return NetworkDNSRecord{}, errNotFound
	}
	if targetDeviceID != "" {
		device, ok := s.devices[targetDeviceID]
		if !ok {
			return NetworkDNSRecord{}, errNotFound
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

func (s *Store) DeleteDNSRecord(networkID, recordID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.dnsRecords[recordID]
	if !ok || record.NetworkID != networkID {
		return errNotFound
	}
	delete(s.dnsRecords, recordID)
	return nil
}

func (s *Store) ListDNSZones(networkID string) []NetworkDNSZone {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]NetworkDNSZone, 0)
	for _, zone := range s.dnsZones {
		if networkID == "" || zone.NetworkID == networkID {
			out = append(out, zone)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ZoneName < out[j].ZoneName })
	return out
}

func (s *Store) ListDNSRecords(networkID string) []NetworkDNSRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]NetworkDNSRecord, 0)
	for _, record := range s.dnsRecords {
		if networkID == "" || record.NetworkID == networkID {
			out = append(out, record)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FQDN < out[j].FQDN })
	return out
}

func (s *Store) ListPublicMappings(networkID string) []PublicDomainMapping {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]PublicDomainMapping, 0)
	for _, mapping := range s.publicMappings {
		if networkID == "" || mapping.NetworkID == networkID {
			out = append(out, mapping)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublicDomain < out[j].PublicDomain })
	return out
}

func (s *Store) UpsertPublicMapping(mappingID, networkID, alias, publicDomain, sourceRecord, deviceID, protocol, port, externalPort, status string) (PublicDomainMapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.networks[networkID]; !ok {
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
		if !ok || existing.NetworkID != networkID {
			return PublicDomainMapping{}, errNotFound
		}
		mapping = existing
	} else {
		mapping.MappingID = fmt.Sprintf("pub-%06d", s.nextPublicMapSeq)
		mapping.NetworkID = networkID
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

func (s *Store) DeletePublicMapping(networkID, mappingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mapping, ok := s.publicMappings[mappingID]
	if !ok || mapping.NetworkID != networkID {
		return errNotFound
	}
	delete(s.publicMappings, mappingID)
	return nil
}

func (s *Store) CreateSecurityGroup(networkID, name, description, defaultPolicy string) (SecurityGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.networks[networkID]; !ok {
		return SecurityGroup{}, errNotFound
	}
	return s.addSecurityGroupLocked(networkID, name, description, defaultString(defaultPolicy, "deny"), time.Now().Unix()), nil
}

func (s *Store) DeleteSecurityGroup(networkID, securityGroupID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	group, ok := s.securityGroups[securityGroupID]
	if !ok || group.NetworkID != networkID {
		return errNotFound
	}
	delete(s.securityGroups, securityGroupID)
	for ruleID, rule := range s.securityGroupRules {
		if rule.SecurityGroupID == securityGroupID {
			delete(s.securityGroupRules, ruleID)
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
		PeerType:        defaultString(peerType, "network"),
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

func (s *Store) SecurityGroupNetworkID(securityGroupID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	group, ok := s.securityGroups[securityGroupID]
	if !ok {
		return "", errNotFound
	}
	return group.NetworkID, nil
}

func (s *Store) SecurityRuleNetworkID(ruleID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, ok := s.securityGroupRules[ruleID]
	if !ok {
		return "", errNotFound
	}
	group, ok := s.securityGroups[rule.SecurityGroupID]
	if !ok {
		return "", errNotFound
	}
	return group.NetworkID, nil
}

func (s *Store) GetSecurityRule(ruleID string) (SecurityGroupRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, ok := s.securityGroupRules[ruleID]
	if !ok {
		return SecurityGroupRule{}, errNotFound
	}
	return rule, nil
}

func (s *Store) ListSecurityGroups(networkID string) []SecurityGroup {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SecurityGroup, 0)
	for _, group := range s.securityGroups {
		if networkID == "" || group.NetworkID == networkID {
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

func (s *Store) ListActiveNetworkDeviceIDs(networkID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0)
	for _, device := range s.networkDevices {
		if device.NetworkID == networkID && device.Enabled && device.Status == "active" {
			out = append(out, device.DeviceID)
		}
	}
	sort.Strings(out)
	return out
}

func (s *Store) HasActiveNetworkDevice(networkID, deviceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.networkDevices[networkID+"|"+deviceID]
	return ok && device.Enabled && device.Status == "active"
}

func (s *Store) NextNetworkConfigVersion(networkID string, now int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	version := int64(1)
	for _, item := range s.configVersions {
		if item.NetworkID == networkID && item.ConfigVersion >= version {
			version = item.ConfigVersion + 1
		}
	}
	config := NetworkConfigVersion{
		ConfigID:      fmt.Sprintf("config-%06d", s.nextConfigSeq),
		NetworkID:     networkID,
		ConfigVersion: version,
		ConfigHash:    fmt.Sprintf("%s-%d", networkID, version),
		PushedAt:      now,
		Status:        "pushed",
	}
	s.nextConfigSeq++
	s.configVersions[config.ConfigID] = config
	return version
}

func (s *Store) currentNetworkConfigVersionLocked(networkID string) int64 {
	version := int64(1)
	for _, item := range s.configVersions {
		if item.NetworkID == networkID && item.ConfigVersion > version {
			version = item.ConfigVersion
		}
	}
	return version
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

func (s *Store) NetworkConfig(networkID, deviceID string) (NetworkConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.networkConfigLocked(networkID, deviceID)
}

func (s *Store) NetworkConfigsForDevice(deviceID string) ([]NetworkConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.networkConfigsForDeviceLocked(deviceID)
}

func (s *Store) networkConfigsForDeviceLocked(deviceID string) ([]NetworkConfig, error) {
	if _, ok := s.devices[deviceID]; !ok {
		return nil, errNotFound
	}
	networkIDs := make([]string, 0)
	for _, membership := range s.networkDevices {
		if membership.DeviceID != deviceID || !membership.Enabled || membership.Status != "active" {
			continue
		}
		network, ok := s.networks[membership.NetworkID]
		if !ok || network.Status != "enabled" {
			continue
		}
		networkIDs = append(networkIDs, membership.NetworkID)
	}
	sort.Strings(networkIDs)
	out := make([]NetworkConfig, 0, len(networkIDs))
	for _, networkID := range networkIDs {
		config, err := s.networkConfigLocked(networkID, deviceID)
		if err != nil {
			return nil, err
		}
		out = append(out, config)
	}
	return out, nil
}

func (s *Store) networkConfigLocked(networkID, deviceID string) (NetworkConfig, error) {
	device, ok := s.devices[deviceID]
	if !ok {
		return NetworkConfig{}, errNotFound
	}
	network, ok := s.networks[networkID]
	if !ok {
		return NetworkConfig{}, errNotFound
	}
	membership, ok := s.networkDevices[networkID+"|"+deviceID]
	if !ok || !membership.Enabled || membership.Status != "active" {
		return NetworkConfig{}, errNotFound
	}
	peers := make([]Device, 0)
	for _, membership := range s.networkDevices {
		if membership.NetworkID == networkID && membership.Enabled && membership.Status == "active" && membership.DeviceID != deviceID {
			peers = append(peers, s.devices[membership.DeviceID])
		}
	}
	groups := make([]SecurityGroup, 0)
	groupIDs := make(map[string]bool)
	for _, group := range s.securityGroups {
		if group.NetworkID == networkID && group.Status == "active" {
			groups = append(groups, group)
			groupIDs[group.SecurityGroupID] = true
		}
	}
	rules := make([]SecurityGroupRule, 0)
	for _, rule := range s.securityGroupRules {
		if rule.Enabled && groupIDs[rule.SecurityGroupID] {
			rules = append(rules, rule)
		}
	}
	relayCandidates := s.activeRelayCandidatesLocked()
	if len(relayCandidates) == 0 {
		relayCandidates = configuredRelayCandidates()
	}
	return NetworkConfig{
		NetworkID:       networkID,
		NetworkName:     network.Name,
		NetworkCode:     network.Code,
		ConfigVersion:   s.currentNetworkConfigVersionLocked(networkID),
		DeviceID:        deviceID,
		GlobalIP:        device.GlobalIP,
		GlobalName:      device.GlobalName,
		Peers:           peers,
		SecurityGroups:  groups,
		Rules:           rules,
		DNSZones:        s.listDNSZonesLocked(networkID),
		DNSRecords:      s.listDNSRecordsLocked(networkID),
		RelayCandidates: relayCandidates,
	}, nil
}

func (s *Store) createSessionLocked(userID string, now int64) UserSession {
	session := UserSession{SessionID: fmt.Sprintf("session-%s-%d", userID, now), UserID: userID, Token: fmt.Sprintf("token-%s-%d", userID, now), CreatedAt: now, ExpiresAt: now + 86400*30}
	s.sessions[session.Token] = session
	return session
}

func (s *Store) ensureDefaultNetworkForUserLocked(userID string, now int64) Network {
	id := "default-" + userID
	if network, ok := s.networks[id]; ok {
		return network
	}
	network := Network{NetworkID: id, OwnerUserID: userID, Name: "默认网络", Code: "default", TemplateKey: "default", Status: "enabled", Default: true, CreatedAt: now, UpdatedAt: now}
	s.networks[id] = network
	s.addSecurityGroupLocked(id, "默认安全组", "默认网络安全组", "deny", now)
	s.addDNSZoneLocked(id, networkZoneName(network), false, now)
	return network
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

func (s *Store) bindExistingDeviceToUser(deviceID, userID string) {
	deviceID = strings.TrimSpace(deviceID)
	userID = strings.TrimSpace(userID)
	if deviceID == "" || userID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.devices[deviceID]
	if !ok {
		return
	}
	if _, ok := s.users[userID]; !ok {
		return
	}
	now := time.Now().Unix()
	if device.OwnerID != userID {
		s.transferDeviceOwnerLocked(deviceID, device.OwnerID, userID, "browser_login", now)
		device.OwnerID = userID
		device.UpdatedAt = now
		s.devices[deviceID] = device
	}
	network := s.ensureDefaultNetworkForUserLocked(userID, now)
	s.networkDevices[network.NetworkID+"|"+deviceID] = NetworkDevice{
		NetworkDeviceID: fmt.Sprintf("network-device-%s-%s", network.NetworkID, deviceID),
		NetworkID:       network.NetworkID,
		DeviceID:        deviceID,
		OwnerUserID:     userID,
		Alias:           device.Alias,
		Enabled:         true,
		Status:          "active",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func (s *Store) addNetworkDeviceLocked(networkID, deviceID, ownerUserID, alias string, enabled bool, now int64) NetworkDevice {
	status := "active"
	if !enabled {
		status = "disabled"
	}
	networkDevice := NetworkDevice{NetworkDeviceID: fmt.Sprintf("network-device-%s-%s", networkID, deviceID), NetworkID: networkID, DeviceID: deviceID, OwnerUserID: ownerUserID, Alias: strings.TrimSpace(alias), Enabled: enabled, Status: status, CreatedAt: now, UpdatedAt: now}
	s.networkDevices[networkID+"|"+deviceID] = networkDevice
	return networkDevice
}

func (s *Store) addDNSZoneLocked(networkID, zoneName string, exposeGlobal bool, now int64) NetworkDNSZone {
	zoneName = strings.TrimSpace(zoneName)
	if zoneName == "" {
		zoneName = "default.lan"
	}
	zone := NetworkDNSZone{ZoneID: fmt.Sprintf("zone-%06d", s.nextZoneSeq), NetworkID: networkID, ZoneName: zoneName, ExposeGlobal: exposeGlobal, Status: "active", CreatedAt: now}
	s.nextZoneSeq++
	s.dnsZones[zone.ZoneID] = zone
	return zone
}

func (s *Store) addSecurityGroupLocked(networkID, name, description, defaultPolicy string, now int64) SecurityGroup {
	group := SecurityGroup{SecurityGroupID: fmt.Sprintf("sg-%06d", s.nextSecuritySeq), NetworkID: networkID, Name: defaultString(name, "默认安全组"), Description: description, DefaultPolicy: defaultString(defaultPolicy, "deny"), Status: "active", CreatedAt: now}
	s.nextSecuritySeq++
	s.securityGroups[group.SecurityGroupID] = group
	return group
}

func (s *Store) canUserAddNetworkDeviceLocked(userID, deviceID, ownerUserID string) bool {
	return s.canUserSeeDeviceLocked(userID, deviceID, ownerUserID)
}

func (s *Store) canUserSeeDeviceLocked(userID, deviceID, ownerUserID string) bool {
	if userID == "" {
		return false
	}
	if ownerUserID == userID {
		return true
	}
	if grant, ok := s.deviceAccessGrants[userID+"|"+deviceID]; ok && grant.Status == "active" {
		return true
	}
	return false
}

func (s *Store) deviceWithOwnerEmailLocked(device Device) Device {
	if user, ok := s.users[device.OwnerID]; ok {
		device.OwnerEmail = user.Email
	}
	return device
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

func (s *Store) listDNSZonesLocked(networkID string) []NetworkDNSZone {
	out := make([]NetworkDNSZone, 0)
	for _, zone := range s.dnsZones {
		if zone.NetworkID == networkID {
			out = append(out, zone)
		}
	}
	return out
}

func (s *Store) listDNSRecordsLocked(networkID string) []NetworkDNSRecord {
	out := make([]NetworkDNSRecord, 0)
	for _, record := range s.dnsRecords {
		if record.NetworkID == networkID {
			out = append(out, record)
		}
	}
	return out
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte("slan:" + password))
	return hex.EncodeToString(sum[:])
}

func networkZoneName(network Network) string {
	if network.Code == "default" {
		return "default.lan"
	}
	return sanitizeDNSLabel(network.Code) + ".internal"
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

func optionalStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func maxInt64(value, fallback int64) int64 {
	if value < fallback {
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
