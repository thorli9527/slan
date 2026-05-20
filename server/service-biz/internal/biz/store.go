package biz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	ipamGlobalCIDR        = "10.0.0.0/8"
	ipamSubnetPrefix      = 20
	ipamSubnetSize        = 1 << (32 - ipamSubnetPrefix)
	ipamPoolLowWatermark  = 1000
	ipamMaxOffsetIn10CIDR = 1 << 24
	deviceInviteTTL       = 30 * time.Minute
	userSessionTTL        = 7 * 24 * time.Hour
	deviceSessionTTL      = 30 * 24 * time.Hour
	activePeerTTL         = 180 * time.Second
	preloginDeviceTTL     = 15 * time.Minute
	prepareRateLimitMax   = 30
	loginRateLimitWindow  = 15 * time.Minute
	loginRateLimitBlock   = 15 * time.Minute
	loginRateLimitMaxFail = 5
)

var (
	errBadRequest   = errors.New("bad request")
	errUnauthorized = errors.New("unauthorized")
	errNotFound     = errors.New("not found")
	errConflict     = errors.New("conflict")
	errUnavailable  = errors.New("unavailable")
	errRateLimited  = errors.New("rate limited")
)

type Store struct {
	mu sync.Mutex
	db *sql.DB

	nextUserID           int
	nextUserSessionSeq   int
	nextDeviceSeq        int
	nextNetworkSeq       int
	nextDeviceInviteSeq  int
	nextDeviceSessionSeq int
	nextBootstrapKeySeq  int
	nextOwnerSeq         int
	nextOwnerLogSeq      int
	nextZoneSeq          int
	nextRecordSeq        int
	nextSecuritySeq      int
	nextSecurityRuleSeq  int
	nextPublicMapSeq     int
	nextConfigSeq        int
	nextIPSubnetSeq      int
	nextIPAddressSeq     int
	nextIPOffset         uint32
	nextOperatorSeq      int
	nextRelayNodeSeq     int
	nextPunchNodeSeq     int
	nextProductSeq       int
	nextOrderSeq         int
	nextRenewalSeq       int
	nextDownloadSeq      int
	nextAuditSeq         int

	users                    map[string]User
	userByEmail              map[string]string
	userAliases              map[string]UserAlias
	sessions                 map[string]UserSession
	loginFailures            map[string]LoginFailure
	operators                map[string]OperatorUser
	operatorByEmail          map[string]string
	operatorSessions         map[string]OperatorSession
	devices                  map[string]Device
	deviceOwners             map[string]DeviceOwner
	ownerLogs                map[string]DeviceOwnerChangeLog
	ipamSubnets              map[string]IPAMSubnet
	globalIPs                map[string]GlobalIPAddress
	networks                 map[string]Network
	deviceInvites            map[string]DeviceInvite
	deviceAccessGrants       map[string]DeviceAccessGrant
	deviceInviteStore        deviceInviteStore
	deviceSessions           map[string]DeviceSession
	deviceSessionByToken     map[string]string
	deviceBootstrapKeys      map[string]DeviceBootstrapKey
	deviceBootstrapKeyStore  deviceBootstrapKeyStore
	consoleLoginKeys         map[string]ConsoleLoginKey
	controlDeliveries        map[string]MQTTControlDelivery
	controlDeliveryStorePath string
	networkDevices           map[string]NetworkDevice
	dnsZones                 map[string]NetworkDNSZone
	dnsRecords               map[string]NetworkDNSRecord
	publicMappings           map[string]PublicDomainMapping
	securityGroups           map[string]SecurityGroup
	securityGroupRules       map[string]SecurityGroupRule
	runtimeStatuses          map[string]DeviceRuntimeStatus
	deviceEndpoints          map[string][]DeviceEndpoint
	configVersions           map[string]NetworkConfigVersion
	customerPlans            map[string]CustomerPlanAssignment
	customerProfiles         map[string]CustomerProfile
	opsPlans                 map[string]OpsPlan
	products                 map[string]Product
	orders                   map[string]Order
	renewals                 map[string]Renewal
	relayNodes               map[string]OpsRelayNode
	punchNodes               map[string]OpsPunchNode
	clientDownloads          map[string]ClientDownload
	auditEvents              map[string]AuditEvent
}

func NewStore() *Store {
	return NewStoreWithDeviceInviteStore(newDeviceInviteStoreFromEnv())
}

func NewStoreWithPostgres(db *sql.DB) *Store {
	store := NewStoreWithDeviceInviteStore(newDeviceInviteStoreFromEnv())
	store.db = db
	if db != nil {
		postgresInviteStore := &postgresDeviceInviteStore{db: db}
		store.deviceInviteStore = postgresInviteStore
		store.deviceBootstrapKeyStore = postgresInviteStore
		store.mu.Lock()
		if err := store.loadPostgresCoreLocked(context.Background()); err != nil {
			log.Printf("service-biz load postgres core state failed: %v", err)
		}
		store.ensureIPPoolLocked(time.Now().Unix())
		store.mu.Unlock()
	}
	return store
}

func NewStoreWithDeviceInviteStore(inviteStore deviceInviteStore) *Store {
	if inviteStore == nil {
		inviteStore = newMemoryDeviceInviteStore()
	}
	bootstrapKeyStore, _ := inviteStore.(deviceBootstrapKeyStore)
	if bootstrapKeyStore == nil {
		bootstrapKeyStore = newMemoryDeviceInviteStore()
	}
	store := &Store{
		nextUserID:              1,
		nextUserSessionSeq:      1,
		nextDeviceSeq:           1,
		nextNetworkSeq:          1,
		nextDeviceInviteSeq:     1,
		nextDeviceSessionSeq:    1,
		nextBootstrapKeySeq:     1,
		nextOwnerSeq:            1,
		nextOwnerLogSeq:         1,
		nextZoneSeq:             1,
		nextRecordSeq:           1,
		nextSecuritySeq:         1,
		nextSecurityRuleSeq:     1,
		nextPublicMapSeq:        1,
		nextConfigSeq:           1,
		nextIPSubnetSeq:         1,
		nextIPAddressSeq:        1,
		nextIPOffset:            0,
		nextOperatorSeq:         1,
		nextRelayNodeSeq:        1,
		nextPunchNodeSeq:        1,
		nextProductSeq:          1,
		nextOrderSeq:            1,
		nextRenewalSeq:          1,
		nextDownloadSeq:         1,
		nextAuditSeq:            1,
		users:                   make(map[string]User),
		userByEmail:             make(map[string]string),
		userAliases:             make(map[string]UserAlias),
		sessions:                make(map[string]UserSession),
		loginFailures:           make(map[string]LoginFailure),
		operators:               make(map[string]OperatorUser),
		operatorByEmail:         make(map[string]string),
		operatorSessions:        make(map[string]OperatorSession),
		devices:                 make(map[string]Device),
		deviceOwners:            make(map[string]DeviceOwner),
		ownerLogs:               make(map[string]DeviceOwnerChangeLog),
		ipamSubnets:             make(map[string]IPAMSubnet),
		globalIPs:               make(map[string]GlobalIPAddress),
		networks:                make(map[string]Network),
		deviceInvites:           make(map[string]DeviceInvite),
		deviceAccessGrants:      make(map[string]DeviceAccessGrant),
		deviceInviteStore:       inviteStore,
		deviceSessions:          make(map[string]DeviceSession),
		deviceSessionByToken:    make(map[string]string),
		deviceBootstrapKeys:     make(map[string]DeviceBootstrapKey),
		deviceBootstrapKeyStore: bootstrapKeyStore,
		consoleLoginKeys:        make(map[string]ConsoleLoginKey),
		controlDeliveries:       make(map[string]MQTTControlDelivery),
		networkDevices:          make(map[string]NetworkDevice),
		dnsZones:                make(map[string]NetworkDNSZone),
		dnsRecords:              make(map[string]NetworkDNSRecord),
		publicMappings:          make(map[string]PublicDomainMapping),
		securityGroups:          make(map[string]SecurityGroup),
		securityGroupRules:      make(map[string]SecurityGroupRule),
		runtimeStatuses:         make(map[string]DeviceRuntimeStatus),
		deviceEndpoints:         make(map[string][]DeviceEndpoint),
		configVersions:          make(map[string]NetworkConfigVersion),
		customerPlans:           make(map[string]CustomerPlanAssignment),
		customerProfiles:        make(map[string]CustomerProfile),
		opsPlans:                make(map[string]OpsPlan),
		products:                make(map[string]Product),
		orders:                  make(map[string]Order),
		renewals:                make(map[string]Renewal),
		relayNodes:              make(map[string]OpsRelayNode),
		punchNodes:              make(map[string]OpsPunchNode),
		clientDownloads:         make(map[string]ClientDownload),
		auditEvents:             make(map[string]AuditEvent),
	}
	store.ensureIPPoolLocked(time.Now().Unix())
	store.controlDeliveryStorePath = controlDeliveryStorePathFromEnv()
	if err := store.loadControlDeliveriesLocked(); err != nil {
		log.Printf("service-biz load mqtt control deliveries failed: %v", err)
	}
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
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return AuthResponse{}, Network{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	network, group, zone := s.ensureDefaultNetworkResourcesForUserLocked(user.UserID, now)
	session := s.createSessionLocked(user.UserID, now)
	if err := s.persistPostgresUserRegisterTxLocked(ctx, postgresTx, user, network, group, zone, session); err != nil {
		return AuthResponse{}, Network{}, err
	}
	postgresTx = nil
	return AuthResponse{User: user, Session: session}, network, nil
}

func (s *Store) LoginUser(email, password string) (AuthResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return AuthResponse{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	userID, ok := s.userByEmail[email]
	if !ok {
		return AuthResponse{}, errNotFound
	}
	user := s.users[userID]
	if !verifyPasswordHash(user.PasswordHash, password) || user.Status != "active" {
		return AuthResponse{}, errBadRequest
	}
	session := s.createSessionLocked(user.UserID, time.Now().Unix())
	if err := s.persistPostgresUserSessionTxLocked(ctx, postgresTx, session); err != nil {
		return AuthResponse{}, err
	}
	postgresTx = nil
	return AuthResponse{User: user, Session: session}, nil
}

func (s *Store) AuthByToken(token string) (AuthResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return AuthResponse{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return AuthResponse{}, err
	}
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

func (s *Store) RenewUserSession(token string) (AuthResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return AuthResponse{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return AuthResponse{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	session, ok := s.sessions[token]
	if !ok || session.ExpiresAt < time.Now().Unix() {
		return AuthResponse{}, errNotFound
	}
	user, ok := s.users[session.UserID]
	if !ok || user.Status != "active" {
		return AuthResponse{}, errNotFound
	}
	session.ExpiresAt = time.Now().Add(userSessionTTL).Unix()
	s.sessions[token] = session
	if err := s.persistPostgresUserSessionRenewTxLocked(ctx, postgresTx, session); err != nil {
		return AuthResponse{}, err
	}
	postgresTx = nil
	return AuthResponse{User: user, Session: session}, nil
}

func (s *Store) LogoutSessions(accessToken, deviceToken string) error {
	accessToken = strings.TrimSpace(accessToken)
	deviceToken = strings.TrimSpace(deviceToken)
	if accessToken == "" && deviceToken == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	logoutUserID := ""
	if accessToken != "" {
		for sessionID, session := range s.sessions {
			if session.Token == accessToken {
				logoutUserID = session.UserID
				delete(s.sessions, sessionID)
			}
		}
	}
	if logoutUserID != "" {
		for sessionID, session := range s.deviceSessions {
			if session.UserID != logoutUserID || session.State != "active" {
				continue
			}
			session.State = "revoked"
			s.deviceSessions[sessionID] = session
			delete(s.deviceSessionByToken, session.DeviceToken)
		}
	}
	if deviceToken != "" {
		if sessionID, ok := s.deviceSessionByToken[deviceToken]; ok {
			if session, ok := s.deviceSessions[sessionID]; ok {
				session.State = "revoked"
				s.deviceSessions[sessionID] = session
			}
			delete(s.deviceSessionByToken, deviceToken)
		}
	}
	if err := s.persistPostgresLogoutTxLocked(ctx, postgresTx, accessToken, deviceToken, logoutUserID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) CreateConsoleLoginKey(accessToken, deviceID string, ttl time.Duration) (ConsoleLoginKey, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ConsoleLoginKey{}, errBadRequest
	}
	auth, err := s.AuthByToken(accessToken)
	if err != nil {
		return ConsoleLoginKey{}, err
	}
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	token, err := secureTokenHex(24)
	if err != nil {
		return ConsoleLoginKey{}, err
	}
	now := time.Now().Unix()
	key := ConsoleLoginKey{
		LoginKey:  "clk-" + token,
		UserID:    auth.User.UserID,
		DeviceID:  deviceID,
		CreatedAt: now,
		ExpiresAt: now + int64(ttl.Seconds()),
		Status:    "unused",
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return ConsoleLoginKey{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok || device.OwnerID != auth.User.UserID || device.Status != "active" {
		return ConsoleLoginKey{}, errNotFound
	}
	s.consoleLoginKeys[key.LoginKey] = key
	if err := s.persistPostgresConsoleLoginKeyTxLocked(ctx, postgresTx, key); err != nil {
		return ConsoleLoginKey{}, err
	}
	postgresTx = nil
	return key, nil
}

func (s *Store) ConsumeConsoleLoginKey(loginKey string) (AuthResponse, error) {
	loginKey = strings.TrimSpace(loginKey)
	if loginKey == "" {
		return AuthResponse{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return AuthResponse{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	key, ok := s.consoleLoginKeys[loginKey]
	if !ok {
		return AuthResponse{}, errNotFound
	}
	now := time.Now().Unix()
	if key.Status != "unused" || key.ExpiresAt < now {
		return AuthResponse{}, errNotFound
	}
	user, ok := s.users[key.UserID]
	if !ok || user.Status != "active" {
		return AuthResponse{}, errNotFound
	}
	key.Status = "used"
	key.ConsumedAt = now
	s.consoleLoginKeys[loginKey] = key
	session := s.createSessionLocked(user.UserID, now)
	if err := s.persistPostgresConsoleLoginConsumeTxLocked(ctx, postgresTx, key, session); err != nil {
		return AuthResponse{}, err
	}
	postgresTx = nil
	return AuthResponse{User: user, Session: session}, nil
}

func (s *Store) userByID(userID string) (User, error) {
	userID = strings.TrimSpace(userID)
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok || user.Status != "active" {
		return User{}, errNotFound
	}
	return user, nil
}

func (s *Store) DeviceAuthByToken(token string) (DeviceSession, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return DeviceSession{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return DeviceSession{}, err
	}
	sessionID, ok := s.deviceSessionByToken[token]
	if !ok {
		return DeviceSession{}, errUnauthorized
	}
	session, ok := s.deviceSessions[sessionID]
	if !ok || session.State != "active" || session.DeviceTokenExpiresAt < time.Now().Unix() {
		return DeviceSession{}, errUnauthorized
	}
	return session, nil
}

func (s *Store) CreateDeviceBootstrapKey(createdByUserID, networkID, deviceAlias string, ttlSeconds int64) (DeviceBootstrapKey, error) {
	createdByUserID = strings.TrimSpace(createdByUserID)
	networkID = strings.TrimSpace(networkID)
	if createdByUserID == "" || networkID == "" {
		return DeviceBootstrapKey{}, errBadRequest
	}
	if ttlSeconds <= 0 {
		ttlSeconds = int64((30 * time.Minute).Seconds())
	}
	if ttlSeconds > int64((24 * time.Hour).Seconds()) {
		ttlSeconds = int64((24 * time.Hour).Seconds())
	}
	keyValue, err := secureTokenHex(32)
	if err != nil {
		return DeviceBootstrapKey{}, err
	}
	keyValue = "sk_" + keyValue
	now := time.Now().Unix()
	s.mu.Lock()
	if _, ok := s.users[createdByUserID]; !ok {
		s.mu.Unlock()
		return DeviceBootstrapKey{}, errNotFound
	}
	network, ok := s.networks[networkID]
	if !ok || network.OwnerUserID != createdByUserID {
		s.mu.Unlock()
		return DeviceBootstrapKey{}, errNotFound
	}
	keyID := fmt.Sprintf("device-bootstrap-%06d", s.nextBootstrapKeySeq)
	s.nextBootstrapKeySeq++
	s.mu.Unlock()
	key := DeviceBootstrapKey{
		KeyID:           keyID,
		Key:             keyValue,
		KeyHash:         bootstrapKeyHash(keyValue),
		CreatedByUserID: createdByUserID,
		NetworkID:       networkID,
		DeviceAlias:     strings.TrimSpace(deviceAlias),
		CreatedAt:       now,
		ExpiresAt:       now + ttlSeconds,
		Status:          "unused",
	}
	ttl := time.Duration(ttlSeconds) * time.Second
	if err := s.deviceBootstrapKeyStore.SaveBootstrapKey(key, ttl); err != nil {
		return DeviceBootstrapKey{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deviceBootstrapKeys[key.KeyID] = key
	return key, nil
}

func (s *Store) ListDeviceBootstrapKeys(createdByUserID string) []DeviceBootstrapKey {
	createdByUserID = strings.TrimSpace(createdByUserID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres bootstrap keys failed: %v", err)
	}
	out := make([]DeviceBootstrapKey, 0)
	now := time.Now().Unix()
	for _, key := range s.deviceBootstrapKeys {
		if createdByUserID != "" && key.CreatedByUserID != createdByUserID {
			continue
		}
		key.Key = ""
		if key.Status == "unused" && key.ExpiresAt < now {
			key.Status = "expired"
		}
		out = append(out, key)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) RevokeDeviceBootstrapKey(keyID, actorUserID string) (DeviceBootstrapKey, error) {
	keyID = strings.TrimSpace(keyID)
	actorUserID = strings.TrimSpace(actorUserID)
	if keyID == "" || actorUserID == "" {
		return DeviceBootstrapKey{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return DeviceBootstrapKey{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	key, ok := s.deviceBootstrapKeys[keyID]
	if !ok || key.CreatedByUserID != actorUserID {
		return DeviceBootstrapKey{}, errNotFound
	}
	if key.Status == "used" {
		return DeviceBootstrapKey{}, errConflict
	}
	key.Status = "revoked"
	key.RevokedAt = time.Now().Unix()
	key.Key = ""
	s.deviceBootstrapKeys[keyID] = key
	if err := s.persistPostgresBootstrapKeyRevokeTxLocked(ctx, postgresTx, key); err != nil {
		return DeviceBootstrapKey{}, err
	}
	postgresTx = nil
	return key, nil
}

func (s *Store) BootstrapDeviceSession(sessionKey, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, DeviceSession, []NetworkConfig, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	deviceID = strings.TrimSpace(deviceID)
	if sessionKey == "" || deviceID == "" {
		return Device{}, DeviceSession{}, nil, errBadRequest
	}
	bootstrapKey, err := s.deviceBootstrapKeyStore.ConsumeBootstrapKey(bootstrapKeyHash(sessionKey))
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	now := time.Now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if stored, ok := s.deviceBootstrapKeys[bootstrapKey.KeyID]; ok {
		if stored.Status != "unused" || stored.RevokedAt > 0 || stored.ExpiresAt < now {
			return Device{}, DeviceSession{}, nil, errNotFound
		}
	}
	user, ok := s.users[bootstrapKey.CreatedByUserID]
	if !ok || user.Status != "active" {
		return Device{}, DeviceSession{}, nil, errNotFound
	}
	network, ok := s.networks[bootstrapKey.NetworkID]
	if !ok || network.OwnerUserID != user.UserID || network.Status != "enabled" {
		return Device{}, DeviceSession{}, nil, errNotFound
	}
	if existing, ok := s.devices[deviceID]; ok && existing.OwnerID != user.UserID && !samePhysicalDeviceIdentity(existing, publicKey) {
		return Device{}, DeviceSession{}, nil, errConflict
	}
	device := s.upsertBootstrapDeviceLocked(user.UserID, deviceID, name, platform, osName, osVersion, defaultString(alias, bootstrapKey.DeviceAlias), publicKey, now)
	membership := s.addNetworkDeviceLocked(network.NetworkID, device.DeviceID, user.UserID, defaultString(alias, bootstrapKey.DeviceAlias), true, now)
	configs, err := s.networkConfigsForDeviceLocked(device.DeviceID)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	session, err := s.createDeviceSessionLocked(device.DeviceID, user.UserID, configs, now)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	bootstrapKey.Status = "used"
	bootstrapKey.UsedAt = now
	bootstrapKey.UsedByDeviceID = device.DeviceID
	bootstrapKey.Key = ""
	s.deviceBootstrapKeys[bootstrapKey.KeyID] = bootstrapKey
	if err := s.persistPostgresBootstrapDeviceSessionTxLocked(ctx, postgresTx, device, membership, session, bootstrapKey); err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), session, configs, nil
}

func (s *Store) BindDeviceSession(accessToken, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, DeviceSession, []NetworkConfig, error) {
	auth, err := s.AuthByToken(accessToken)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return Device{}, DeviceSession{}, nil, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	previousOwnerID := ""
	if ok && device.OwnerID != auth.User.UserID {
		if !samePhysicalDeviceIdentity(device, publicKey) {
			return Device{}, DeviceSession{}, nil, errConflict
		}
		previousOwnerID = device.OwnerID
		s.transferDeviceOwnerLocked(deviceID, device.OwnerID, auth.User.UserID, "device_session_bind", time.Now().Unix())
		s.removeDeviceFromUserNetworksLocked(deviceID, device.OwnerID)
		s.revokeDeviceSessionsLocked(deviceID, device.OwnerID)
		device.OwnerID = auth.User.UserID
		device.Status = "active"
		device.UpdatedAt = time.Now().Unix()
		s.devices[deviceID] = device
	}
	if !ok {
		device = s.upsertBootstrapDeviceLocked(auth.User.UserID, deviceID, name, platform, osName, osVersion, alias, publicKey, time.Now().Unix())
		defaultNetwork := s.ensureDefaultNetworkForUserLocked(auth.User.UserID, time.Now().Unix())
		s.addNetworkDeviceLocked(defaultNetwork.NetworkID, deviceID, auth.User.UserID, alias, true, time.Now().Unix())
	} else {
		device.Status = "active"
		device.UpdatedAt = time.Now().Unix()
		s.devices[deviceID] = device
		s.addDeviceOwnerLocked(deviceID, auth.User.UserID, time.Now().Unix())
	}
	configs, err := s.networkConfigsForDeviceLocked(deviceID)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	s.revokeDeviceSessionsLocked(deviceID, auth.User.UserID)
	session, err := s.createDeviceSessionLocked(deviceID, auth.User.UserID, configs, time.Now().Unix())
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	membership := NetworkDevice{}
	for _, item := range s.networkDevices {
		if item.DeviceID == deviceID && item.OwnerUserID == auth.User.UserID && item.Enabled && item.Status == "active" {
			membership = item
			break
		}
	}
	if membership.NetworkDeviceID == "" {
		defaultNetwork := s.ensureDefaultNetworkForUserLocked(auth.User.UserID, time.Now().Unix())
		membership = s.addNetworkDeviceLocked(defaultNetwork.NetworkID, deviceID, auth.User.UserID, alias, true, time.Now().Unix())
	}
	if err := s.persistPostgresDeviceBindSessionTxLocked(ctx, postgresTx, device, membership, session, previousOwnerID); err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), session, configs, nil
}

func samePhysicalDeviceIdentity(existing Device, publicKey string) bool {
	existingKey := strings.TrimSpace(existing.PublicKey)
	nextKey := strings.TrimSpace(publicKey)
	return existingKey != "" && nextKey != "" && existingKey == nextKey
}

func (s *Store) RenewDeviceSession(deviceToken string, networkEnabled bool, rxBytesTotal, txBytesTotal uint64) (Device, DeviceSession, []NetworkConfig, error) {
	deviceToken = strings.TrimSpace(deviceToken)
	if deviceToken == "" {
		return Device{}, DeviceSession{}, nil, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	sessionID, ok := s.deviceSessionByToken[deviceToken]
	if !ok {
		return Device{}, DeviceSession{}, nil, errUnauthorized
	}
	session, ok := s.deviceSessions[sessionID]
	if !ok || session.State != "active" || session.DeviceTokenExpiresAt < time.Now().Unix() {
		return Device{}, DeviceSession{}, nil, errUnauthorized
	}
	device, ok := s.devices[session.DeviceID]
	if !ok {
		return Device{}, DeviceSession{}, nil, errNotFound
	}
	now := time.Now().Unix()
	device.Status = "active"
	device.UpdatedAt = now
	s.devices[device.DeviceID] = device
	status := s.runtimeStatuses[device.DeviceID]
	status.DeviceID = device.DeviceID
	status.HeartbeatOnline = true
	status.NetworkEnabled = networkEnabled
	status.DeviceEnabled = true
	status.RxBytesTotal = rxBytesTotal
	status.TxBytesTotal = txBytesTotal
	status.LastSeenAt = now
	status.LastReportAt = now
	s.runtimeStatuses[device.DeviceID] = status
	configs, err := s.networkConfigsForDeviceLocked(device.DeviceID)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	session.LastRenewedAt = now
	session.DeviceTokenExpiresAt = now + int64(deviceSessionTTL.Seconds())
	session.ActiveNetworkIDs = networkIDsFromConfigs(configs)
	s.deviceSessions[session.SessionID] = session
	if err := s.persistPostgresDeviceSessionRenewTxLocked(ctx, postgresTx, device, status, session); err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), session, configs, nil
}

func (s *Store) CompleteDeviceLoginForDevice(deviceID, accessToken, action string) (DeviceUserLoginPayload, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return DeviceUserLoginPayload{}, errBadRequest
	}
	auth, err := s.AuthByToken(accessToken)
	if err != nil {
		return DeviceUserLoginPayload{}, err
	}
	if err := s.bindExistingDeviceToUser(deviceID, auth.User.UserID); err != nil {
		return DeviceUserLoginPayload{}, err
	}
	refreshToken := auth.Session.Token
	return DeviceUserLoginPayload{
		AccessToken:  auth.Session.Token,
		RefreshToken: &refreshToken,
		UserID:       auth.User.UserID,
		UserLabel:    defaultString(auth.User.Email, auth.User.UserID),
		DeviceID:     optionalStringPtr(deviceID),
		ExpiresIn:    uint64(maxInt64(auth.Session.ExpiresAt-time.Now().Unix(), 0)),
		Action:       defaultString(action, "login"),
	}, nil
}

func (s *Store) ChangeUserPassword(userID, oldPassword, newPassword string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" || strings.TrimSpace(oldPassword) == "" || strings.TrimSpace(newPassword) == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	user, ok := s.users[userID]
	if !ok {
		return errNotFound
	}
	if !verifyPasswordHash(user.PasswordHash, oldPassword) {
		return errBadRequest
	}
	user.PasswordHash = hashPassword(newPassword)
	user.UpdatedAt = time.Now().Unix()
	s.users[userID] = user
	if err := s.persistPostgresUserPasswordTxLocked(ctx, postgresTx, user); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) RegisterDevice(ownerID, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, NetworkDevice, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return Device{}, NetworkDevice{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, NetworkDevice{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
		if strings.TrimSpace(existing.OwnerID) == "" {
			existing.OwnerID = ownerID
			if strings.TrimSpace(existing.GlobalIP) == "" {
				existing.GlobalIP = s.allocateGlobalIPLocked(deviceID, now)
			}
			existing.GlobalName = sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain()
			existing.Name = defaultString(strings.TrimSpace(name), existing.Name)
			existing.Platform = defaultString(strings.TrimSpace(platform), existing.Platform)
			existing.OSName = defaultString(strings.TrimSpace(osName), existing.OSName)
			existing.OSVersion = defaultString(strings.TrimSpace(osVersion), existing.OSVersion)
			existing.Alias = defaultString(strings.TrimSpace(alias), existing.Alias)
			existing.PublicKey = defaultString(strings.TrimSpace(publicKey), existing.PublicKey)
			existing.Status = "active"
			existing.UpdatedAt = now
			s.devices[deviceID] = existing
			s.addDeviceOwnerLocked(deviceID, ownerID, now)
			defaultNetwork := s.ensureDefaultNetworkForUserLocked(ownerID, now)
			networkDevice := s.addNetworkDeviceLocked(defaultNetwork.NetworkID, deviceID, ownerID, alias, true, now)
			s.runtimeStatuses[deviceID] = DeviceRuntimeStatus{DeviceID: deviceID, DeviceEnabled: true, LastReportAt: now}
			if err := s.persistPostgresDeviceRegisterTxLocked(ctx, postgresTx, existing, networkDevice, ""); err != nil {
				return Device{}, NetworkDevice{}, err
			}
			postgresTx = nil
			return s.deviceWithOwnerEmailLocked(existing), networkDevice, nil
		}
		if existing.OwnerID != ownerID {
			return Device{}, NetworkDevice{}, errConflict
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
		GlobalName: sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain(),
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.devices[device.DeviceID] = device
	s.addDeviceOwnerLocked(device.DeviceID, ownerID, now)
	defaultNetwork := s.ensureDefaultNetworkForUserLocked(ownerID, now)
	networkDevice := s.addNetworkDeviceLocked(defaultNetwork.NetworkID, device.DeviceID, ownerID, alias, true, now)
	s.runtimeStatuses[device.DeviceID] = DeviceRuntimeStatus{DeviceID: device.DeviceID, DeviceEnabled: true, LastReportAt: now}
	if err := s.persistPostgresDeviceRegisterTxLocked(ctx, postgresTx, device, networkDevice, ""); err != nil {
		return Device{}, NetworkDevice{}, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), networkDevice, nil
}

func (s *Store) PrepareDeviceLoginDevice(deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, error) {
	deviceID = strings.TrimSpace(deviceID)
	publicKey = strings.TrimSpace(publicKey)
	if deviceID == "" {
		return Device{}, errBadRequest
	}
	if !validPrepareDevicePublicKey(deviceID, publicKey) {
		return Device{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if err := s.expirePreloginDevicesLocked(ctx, postgresTx, time.Now().Unix()); err != nil {
		return Device{}, err
	}
	if existing, ok := s.devices[deviceID]; ok {
		if strings.TrimSpace(existing.OwnerID) != "" {
			if strings.TrimSpace(existing.PublicKey) != "" && strings.TrimSpace(existing.PublicKey) != publicKey {
				return Device{}, errConflict
			}
			return s.deviceWithOwnerEmailLocked(existing), nil
		}
		existing.Name = defaultString(strings.TrimSpace(name), existing.Name)
		existing.Platform = defaultString(strings.TrimSpace(platform), existing.Platform)
		existing.OSName = defaultString(strings.TrimSpace(osName), existing.OSName)
		existing.OSVersion = defaultString(strings.TrimSpace(osVersion), existing.OSVersion)
		existing.Alias = defaultString(strings.TrimSpace(alias), existing.Alias)
		existing.PublicKey = defaultString(publicKey, existing.PublicKey)
		existing.Status = "active"
		existing.UpdatedAt = time.Now().Unix()
		s.devices[deviceID] = existing
		if err := s.persistPostgresDeviceRegisterTxLocked(ctx, postgresTx, existing, NetworkDevice{}, ""); err != nil {
			return Device{}, err
		}
		postgresTx = nil
		return s.deviceWithOwnerEmailLocked(existing), nil
	}
	now := time.Now().Unix()
	device := Device{
		DeviceID:   deviceID,
		Name:       strings.TrimSpace(name),
		Platform:   strings.TrimSpace(platform),
		OSName:     strings.TrimSpace(osName),
		OSVersion:  strings.TrimSpace(osVersion),
		Alias:      strings.TrimSpace(alias),
		PublicKey:  publicKey,
		GlobalName: sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain(),
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.devices[device.DeviceID] = device
	s.runtimeStatuses[device.DeviceID] = DeviceRuntimeStatus{DeviceID: device.DeviceID, DeviceEnabled: true, LastReportAt: now}
	if err := s.persistPostgresDeviceRegisterTxLocked(ctx, postgresTx, device, NetworkDevice{}, ""); err != nil {
		return Device{}, err
	}
	postgresTx = nil
	return device, nil
}

func (s *Store) expirePreloginDevicesLocked(ctx context.Context, tx *sql.Tx, now int64) error {
	cutoff := now - int64(preloginDeviceTTL.Seconds())
	expired := make([]string, 0)
	for deviceID, device := range s.devices {
		if strings.TrimSpace(device.OwnerID) != "" {
			continue
		}
		if device.CreatedAt <= 0 || device.CreatedAt > cutoff {
			continue
		}
		expired = append(expired, deviceID)
	}
	for _, deviceID := range expired {
		if err := s.removeDeviceLocked(deviceID); err != nil {
			return err
		}
		if err := s.deletePostgresDeviceTxLocked(ctx, tx, deviceID); err != nil {
			return err
		}
	}
	return nil
}

func validPrepareDevicePublicKey(deviceID, publicKey string) bool {
	publicKey = strings.TrimSpace(publicKey)
	if publicKey == "" || publicKey == "client-v2-"+strings.TrimSpace(deviceID) {
		return false
	}
	if strings.HasPrefix(publicKey, "pk_") {
		token := strings.TrimPrefix(publicKey, "pk_")
		return len(token) >= 64 && asciiHex(token)
	}
	return len(publicKey) >= 32
}

func asciiHex(value string) bool {
	for _, ch := range value {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return value != ""
}

func (s *Store) upsertBootstrapDeviceLocked(ownerID, deviceID, name, platform, osName, osVersion, alias, publicKey string, now int64) Device {
	if existing, ok := s.devices[deviceID]; ok {
		if strings.TrimSpace(existing.OwnerID) == "" {
			s.addDeviceOwnerLocked(deviceID, ownerID, now)
			existing.OwnerID = ownerID
			if strings.TrimSpace(existing.GlobalIP) == "" {
				existing.GlobalIP = s.allocateGlobalIPLocked(deviceID, now)
			}
			existing.GlobalName = sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain()
		} else if existing.OwnerID != ownerID {
			s.transferDeviceOwnerLocked(deviceID, existing.OwnerID, ownerID, "bootstrap_session_key", now)
			existing.OwnerID = ownerID
		}
		existing.Name = defaultString(strings.TrimSpace(name), existing.Name)
		existing.Platform = defaultString(strings.TrimSpace(platform), existing.Platform)
		existing.OSName = defaultString(strings.TrimSpace(osName), existing.OSName)
		existing.OSVersion = defaultString(strings.TrimSpace(osVersion), existing.OSVersion)
		existing.Alias = defaultString(strings.TrimSpace(alias), existing.Alias)
		existing.PublicKey = defaultString(strings.TrimSpace(publicKey), existing.PublicKey)
		existing.Status = "active"
		existing.UpdatedAt = now
		s.devices[deviceID] = existing
		return existing
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
		GlobalName: sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain(),
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.devices[deviceID] = device
	s.addDeviceOwnerLocked(device.DeviceID, ownerID, now)
	s.runtimeStatuses[device.DeviceID] = DeviceRuntimeStatus{DeviceID: device.DeviceID, DeviceEnabled: true, LastReportAt: now}
	return device
}

func (s *Store) createDeviceSessionLocked(deviceID, userID string, configs []NetworkConfig, now int64) (DeviceSession, error) {
	deviceToken, err := secureTokenHex(32)
	if err != nil {
		return DeviceSession{}, err
	}
	refreshToken, err := secureTokenHex(32)
	if err != nil {
		return DeviceSession{}, err
	}
	session := DeviceSession{
		SessionID:            fmt.Sprintf("device-session-%06d", s.nextDeviceSessionSeq),
		DeviceID:             deviceID,
		UserID:               userID,
		DeviceToken:          "dt_" + deviceToken,
		DeviceTokenExpiresAt: now + int64(deviceSessionTTL.Seconds()),
		DeviceRefreshToken:   "drt_" + refreshToken,
		RegisteredAt:         now,
		LastRenewedAt:        now,
		ActiveNetworkIDs:     networkIDsFromConfigs(configs),
		State:                "active",
	}
	s.nextDeviceSessionSeq++
	s.deviceSessions[session.SessionID] = session
	s.deviceSessionByToken[session.DeviceToken] = session.SessionID
	return session, nil
}

func (s *Store) revokeDeviceSessionsLocked(deviceID, userID string) {
	for sessionID, session := range s.deviceSessions {
		if session.DeviceID != deviceID || session.UserID != userID || session.State != "active" {
			continue
		}
		session.State = "revoked"
		s.deviceSessions[sessionID] = session
		delete(s.deviceSessionByToken, session.DeviceToken)
	}
}

func (s *Store) removeDeviceFromUserNetworksLocked(deviceID, userID string) {
	for key, membership := range s.networkDevices {
		if membership.DeviceID == deviceID && membership.OwnerUserID == userID {
			delete(s.networkDevices, key)
		}
	}
}

func networkIDsFromConfigs(configs []NetworkConfig) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(configs))
	for _, config := range configs {
		networkID := strings.TrimSpace(config.NetworkID)
		if networkID == "" || seen[networkID] {
			continue
		}
		seen[networkID] = true
		out = append(out, networkID)
	}
	return out
}

func (s *Store) ListUsers() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres users failed: %v", err)
	}
	return sortedValues(s.users, func(a, b User) bool { return a.Email < b.Email })
}

func (s *Store) ListDevices(userID string) []Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres devices failed: %v", err)
	}
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
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, nil, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresDeviceRuntimeTxLocked(ctx, postgresTx, device, status); err != nil {
		return Device{}, nil, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), configs, nil
}

func (s *Store) ReportDeviceRuntime(deviceID string, networkEnabled bool, rxBytesTotal, txBytesTotal uint64) DeviceRuntimeReportResult {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return DeviceRuntimeReportResult{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		log.Printf("service-biz prepare runtime status write failed device=%s: %v", deviceID, err)
		return DeviceRuntimeReportResult{}
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok {
		return DeviceRuntimeReportResult{}
	}
	now := time.Now().Unix()
	previous := s.runtimeStatuses[deviceID]
	status := s.runtimeStatuses[deviceID]
	status.DeviceID = deviceID
	status.HeartbeatOnline = true
	status.NetworkEnabled = networkEnabled
	status.DeviceEnabled = device.Status == "active"
	status.RxBytesTotal = rxBytesTotal
	status.TxBytesTotal = txBytesTotal
	status.LastSeenAt = now
	status.LastReportAt = now
	s.runtimeStatuses[deviceID] = status
	if err := s.persistPostgresDeviceRuntimeTxLocked(ctx, postgresTx, device, status); err != nil {
		log.Printf("service-biz persist runtime status failed device=%s: %v", deviceID, err)
		return DeviceRuntimeReportResult{}
	}
	postgresTx = nil
	networkIDs := make([]string, 0)
	for _, membership := range s.networkDevices {
		if membership.DeviceID == deviceID && membership.Enabled && membership.Status == "active" {
			networkIDs = append(networkIDs, membership.NetworkID)
		}
	}
	sort.Strings(networkIDs)
	return DeviceRuntimeReportResult{
		DeviceID:              deviceID,
		NetworkEnabled:        networkEnabled,
		NetworkEnabledChanged: previous.LastReportAt == 0 || previous.NetworkEnabled != networkEnabled,
		NetworkIDs:            networkIDs,
		ChangedAt:             now,
	}
}

func (s *Store) ReportDeviceEndpoint(networkID, deviceID string, endpoints []DeviceEndpoint) (bool, error) {
	networkID = strings.TrimSpace(networkID)
	deviceID = strings.TrimSpace(deviceID)
	if networkID == "" || deviceID == "" {
		return false, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.networkDevices[networkID+"|"+deviceID]; !ok {
		return false, errNotFound
	}
	now := time.Now().Unix()
	cleaned := make([]DeviceEndpoint, 0, len(endpoints))
	seen := make(map[string]bool)
	for _, endpoint := range endpoints {
		endpoint.Type = strings.TrimSpace(endpoint.Type)
		endpoint.Address = strings.TrimSpace(endpoint.Address)
		if endpoint.Type == "" {
			endpoint.Type = "direct_udp"
		}
		if endpoint.Address == "" || strings.Contains(endpoint.Address, "://") {
			continue
		}
		if endpoint.UpdatedAt == 0 {
			endpoint.UpdatedAt = now
		}
		key := endpoint.Type + "|" + endpoint.Address
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, endpoint)
	}
	key := networkID + "|" + deviceID
	previous := s.deviceEndpoints[key]
	if len(cleaned) == 0 {
		delete(s.deviceEndpoints, key)
		return len(previous) > 0, nil
	}
	s.deviceEndpoints[key] = cleaned
	return deviceEndpointsChanged(previous, cleaned), nil
}

func deviceEndpointsChanged(previous, next []DeviceEndpoint) bool {
	if len(previous) != len(next) {
		return true
	}
	for index := range previous {
		if previous[index].Type != next[index].Type || previous[index].Address != next[index].Address {
			return true
		}
	}
	return false
}

func (s *Store) GetDevice(deviceID string) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return Device{}, err
	}
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return DeviceQuota{}, err
	}
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
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok {
		return errNotFound
	}
	if device.OwnerID != actorUserID {
		key := actorUserID + "|" + deviceID
		if grant, ok := s.deviceAccessGrants[key]; ok && grant.Status == "active" {
			grant.Status = "revoked"
			s.deviceAccessGrants[key] = grant
			if err := s.persistPostgresDeviceAccessGrantRevokeTxLocked(ctx, postgresTx, grant); err != nil {
				return err
			}
			postgresTx = nil
			return nil
		}
		return errNotFound
	}
	if err := s.removeDeviceLocked(deviceID); err != nil {
		return err
	}
	if err := s.persistPostgresDeviceDeleteTxLocked(ctx, postgresTx, deviceID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
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
	for sessionID, session := range s.deviceSessions {
		if session.DeviceID == deviceID {
			delete(s.deviceSessions, sessionID)
			delete(s.deviceSessionByToken, session.DeviceToken)
		}
	}
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres visible devices failed: %v", err)
	}
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
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresDeviceAliasTxLocked(ctx, postgresTx, device); err != nil {
		return Device{}, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), nil
}

func (s *Store) ListUserAliases(ownerUserID string) []UserAlias {
	ownerUserID = strings.TrimSpace(ownerUserID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres user aliases failed: %v", err)
	}
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
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return UserAlias{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.users[ownerUserID]; !ok {
		return UserAlias{}, errNotFound
	}
	item := UserAlias{OwnerUserID: ownerUserID, Email: email, Alias: alias, UpdatedAt: time.Now().Unix()}
	s.userAliases[ownerUserID+"|"+email] = item
	if err := s.persistPostgresUserAliasTxLocked(ctx, postgresTx, item); err != nil {
		return UserAlias{}, err
	}
	postgresTx = nil
	return item, nil
}

func (s *Store) ListDeviceOwnerLogs(deviceID string) []DeviceOwnerChangeLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres owner logs failed: %v", err)
	}
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres global ips failed: %v", err)
	}
	return sortedValues(s.globalIPs, func(a, b GlobalIPAddress) bool { return a.Offset < b.Offset })
}

func (s *Store) RecordMQTTControlAck(deviceID, deliveryID, taskID, action, status, errorText string, processedAtMs int64, now int64) (MQTTControlDelivery, error) {
	deviceID = strings.TrimSpace(deviceID)
	deliveryID = strings.TrimSpace(deliveryID)
	taskID = strings.TrimSpace(taskID)
	action = strings.TrimSpace(action)
	status = strings.TrimSpace(status)
	errorText = strings.TrimSpace(errorText)
	if deviceID == "" || deliveryID == "" || status == "" {
		return MQTTControlDelivery{}, errBadRequest
	}
	if status != "succeeded" && status != "failed" {
		return MQTTControlDelivery{}, errBadRequest
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return MQTTControlDelivery{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.devices[deviceID]; !ok {
		return MQTTControlDelivery{}, errNotFound
	}
	delivery := s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)]
	if delivery.DeliveryID == "" {
		return MQTTControlDelivery{}, errNotFound
	}
	if delivery.AckedAt > 0 && (delivery.Status == "succeeded" || delivery.Status == "failed") {
		return delivery, nil
	}
	if delivery.Status == "expired" || (delivery.ExpiresAt > 0 && now >= delivery.ExpiresAt) {
		return MQTTControlDelivery{}, errConflict
	}
	delivery.DeliveryID = deliveryID
	delivery.DeviceID = deviceID
	delivery.TaskID = defaultString(taskID, delivery.TaskID)
	delivery.Action = defaultString(delivery.Action, action)
	delivery.Status = status
	delivery.Error = errorText
	delivery.ProcessedAtMs = processedAtMs
	delivery.AckedAt = now
	delivery.UpdatedAt = now
	if delivery.MessageType == "" {
		delivery.MessageType = "control_ack"
	}
	if delivery.ExpiresAt <= 0 {
		delivery.ExpiresAt = now + int64(defaultControlMessageTTLSeconds)
	}
	s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)] = delivery
	if err := s.persistControlDeliveriesLocked(); err != nil {
		return MQTTControlDelivery{}, err
	}
	if err := s.persistPostgresControlDeliveryTxLocked(ctx, postgresTx, delivery); err != nil {
		return MQTTControlDelivery{}, err
	}
	if postgresTx != nil {
		if err := postgresTx.Commit(); err != nil {
			return MQTTControlDelivery{}, err
		}
	}
	postgresTx = nil
	return delivery, nil
}

func (s *Store) PrepareMQTTControlDelivery(deviceID, deliveryID, messageType, action string, payload any, createdAt, expiresAt int64) (MQTTControlDelivery, error) {
	deviceID = strings.TrimSpace(deviceID)
	deliveryID = strings.TrimSpace(deliveryID)
	messageType = strings.TrimSpace(messageType)
	action = strings.TrimSpace(action)
	if deviceID == "" || deliveryID == "" || messageType == "" || expiresAt <= createdAt {
		return MQTTControlDelivery{}, errBadRequest
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return MQTTControlDelivery{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return MQTTControlDelivery{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.devices[deviceID]; !ok {
		return MQTTControlDelivery{}, errNotFound
	}
	delivery := s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)]
	if delivery.DeliveryID == "" {
		delivery = MQTTControlDelivery{
			DeliveryID: deliveryID,
			DeviceID:   deviceID,
			Status:     "pending",
			CreatedAt:  createdAt,
			ExpiresAt:  expiresAt,
			UpdatedAt:  createdAt,
		}
	}
	delivery.MessageType = messageType
	delivery.Action = action
	delivery.Payload = payloadJSON
	if delivery.CreatedAt <= 0 {
		delivery.CreatedAt = createdAt
	}
	if delivery.ExpiresAt <= 0 {
		delivery.ExpiresAt = expiresAt
	}
	if delivery.Status == "" {
		delivery.Status = "pending"
	}
	delivery.UpdatedAt = createdAt
	s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)] = delivery
	if err := s.persistControlDeliveriesLocked(); err != nil {
		return MQTTControlDelivery{}, err
	}
	if err := s.persistPostgresControlDeliveryTxLocked(ctx, postgresTx, delivery); err != nil {
		return MQTTControlDelivery{}, err
	}
	if postgresTx != nil {
		if err := postgresTx.Commit(); err != nil {
			return MQTTControlDelivery{}, err
		}
	}
	postgresTx = nil
	return delivery, nil
}

func (s *Store) RecordMQTTControlPublishResult(deviceID, deliveryID string, published bool, errorText string, now int64) (MQTTControlDelivery, error) {
	deviceID = strings.TrimSpace(deviceID)
	deliveryID = strings.TrimSpace(deliveryID)
	errorText = strings.TrimSpace(errorText)
	if deviceID == "" || deliveryID == "" {
		return MQTTControlDelivery{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return MQTTControlDelivery{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	delivery, ok := s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)]
	if !ok {
		return MQTTControlDelivery{}, errNotFound
	}
	delivery.AttemptCount++
	delivery.Error = errorText
	delivery.UpdatedAt = now
	if delivery.ExpiresAt > 0 && now >= delivery.ExpiresAt {
		delivery.Status = "expired"
	} else if published {
		delivery.Status = "published"
		delivery.PublishedAt = now
	} else {
		delivery.Status = "retrying"
	}
	s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)] = delivery
	if err := s.persistControlDeliveriesLocked(); err != nil {
		return MQTTControlDelivery{}, err
	}
	if err := s.persistPostgresControlDeliveryTxLocked(ctx, postgresTx, delivery); err != nil {
		return MQTTControlDelivery{}, err
	}
	if postgresTx != nil {
		if err := postgresTx.Commit(); err != nil {
			return MQTTControlDelivery{}, err
		}
	}
	postgresTx = nil
	return delivery, nil
}

func (s *Store) ListRetryableMQTTControlDeliveries(now int64, limit int) []MQTTControlDelivery {
	if limit <= 0 {
		limit = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres mqtt deliveries failed: %v", err)
	}
	out := make([]MQTTControlDelivery, 0)
	for _, delivery := range s.controlDeliveries {
		if delivery.Status != "pending" && delivery.Status != "retrying" && delivery.Status != "published" {
			continue
		}
		if delivery.ExpiresAt > 0 && now >= delivery.ExpiresAt {
			continue
		}
		if !mqttControlDeliveryDue(delivery, now) {
			continue
		}
		out = append(out, delivery)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			return out[i].DeliveryID < out[j].DeliveryID
		}
		return out[i].UpdatedAt < out[j].UpdatedAt
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) ClaimMQTTControlDeliveryRetry(deviceID, deliveryID string, previousUpdatedAt, now int64) (bool, error) {
	deviceID = strings.TrimSpace(deviceID)
	deliveryID = strings.TrimSpace(deliveryID)
	if deviceID == "" || deliveryID == "" || previousUpdatedAt <= 0 || now <= 0 {
		return false, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := controlDeliveryKey(deviceID, deliveryID)
	delivery, ok := s.controlDeliveries[key]
	if !ok {
		return false, nil
	}
	if delivery.UpdatedAt != previousUpdatedAt {
		return false, nil
	}
	if delivery.Status != "pending" && delivery.Status != "retrying" && delivery.Status != "published" {
		return false, nil
	}
	if delivery.ExpiresAt > 0 && now >= delivery.ExpiresAt {
		return false, s.expireMQTTControlDeliveryLocked(key, delivery, now)
	}
	if !mqttControlDeliveryDue(delivery, now) {
		return false, nil
	}
	if s.db != nil {
		claimed, err := s.claimPostgresControlDeliveryRetryLocked(context.Background(), deviceID, deliveryID, previousUpdatedAt, now)
		if err != nil || !claimed {
			return claimed, err
		}
	}
	delivery.Status = "retrying"
	delivery.UpdatedAt = now
	s.controlDeliveries[key] = delivery
	if err := s.persistControlDeliveriesLocked(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) expireMQTTControlDeliveryLocked(key string, delivery MQTTControlDelivery, now int64) error {
	delivery.Status = "expired"
	delivery.UpdatedAt = now
	s.controlDeliveries[key] = delivery
	if err := s.persistControlDeliveriesLocked(); err != nil {
		return err
	}
	return s.persistPostgresExpiredControlDeliveriesLocked([]MQTTControlDelivery{delivery})
}

func (s *Store) persistPostgresExpiredControlDeliveriesLocked(deliveries []MQTTControlDelivery) error {
	if s.db == nil || len(deliveries) == 0 {
		return nil
	}
	ctx := context.Background()
	tx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(tx)
	for _, delivery := range deliveries {
		s.controlDeliveries[controlDeliveryKey(delivery.DeviceID, delivery.DeliveryID)] = delivery
		if err := s.persistPostgresControlDeliveryTxLocked(ctx, tx, delivery); err != nil {
			return err
		}
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}

func (s *Store) ExpireMQTTControlDeliveries(now int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	expiredDeliveries := make([]MQTTControlDelivery, 0)
	for key, delivery := range s.controlDeliveries {
		if delivery.Status != "pending" && delivery.Status != "retrying" && delivery.Status != "published" {
			continue
		}
		if delivery.ExpiresAt <= 0 || now < delivery.ExpiresAt {
			continue
		}
		delivery.Status = "expired"
		delivery.UpdatedAt = now
		s.controlDeliveries[key] = delivery
		expiredDeliveries = append(expiredDeliveries, delivery)
	}
	if len(expiredDeliveries) == 0 {
		return 0
	}
	if err := s.persistControlDeliveriesLocked(); err != nil {
		log.Printf("service-biz expire mqtt deliveries local persist failed: %v", err)
	}
	if err := s.persistPostgresExpiredControlDeliveriesLocked(expiredDeliveries); err != nil {
		log.Printf("service-biz expire mqtt deliveries postgres persist failed: %v", err)
	}
	return len(expiredDeliveries)
}

func mqttControlDeliveryDue(delivery MQTTControlDelivery, now int64) bool {
	if delivery.AttemptCount <= 0 {
		return true
	}
	return now >= delivery.UpdatedAt+int64(mqttControlRetryDelay(delivery.AttemptCount).Seconds())
}

func mqttControlRetryDelay(attemptCount int) time.Duration {
	switch {
	case attemptCount <= 1:
		return time.Second
	case attemptCount == 2:
		return 3 * time.Second
	case attemptCount == 3:
		return 6 * time.Second
	default:
		return 15 * time.Second
	}
}

func (s *Store) GetMQTTControlDelivery(deliveryID string) (MQTTControlDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		delivery, ok, err := s.loadPostgresControlDeliveryByIDLocked(context.Background(), strings.TrimSpace(deliveryID))
		if err != nil {
			return MQTTControlDelivery{}, err
		}
		if ok {
			return logicalMQTTControlDeliveryState(delivery, timeNow().Unix()), nil
		}
	}
	delivery, ok := s.controlDeliveries[strings.TrimSpace(deliveryID)]
	if !ok {
		for _, delivery := range s.controlDeliveries {
			if delivery.DeliveryID == strings.TrimSpace(deliveryID) {
				return logicalMQTTControlDeliveryState(delivery, timeNow().Unix()), nil
			}
		}
		return MQTTControlDelivery{}, errNotFound
	}
	return logicalMQTTControlDeliveryState(delivery, timeNow().Unix()), nil
}

func (s *Store) GetMQTTControlDeliveryForDevice(deviceID, deliveryID string) (MQTTControlDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		delivery, ok, err := s.loadPostgresControlDeliveryForDeviceLocked(context.Background(), strings.TrimSpace(deviceID), strings.TrimSpace(deliveryID))
		if err != nil {
			return MQTTControlDelivery{}, err
		}
		if ok {
			return logicalMQTTControlDeliveryState(delivery, timeNow().Unix()), nil
		}
	}
	delivery, ok := s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)]
	if !ok {
		return MQTTControlDelivery{}, errNotFound
	}
	return logicalMQTTControlDeliveryState(delivery, timeNow().Unix()), nil
}

func controlDeliveryKey(deviceID, deliveryID string) string {
	return strings.TrimSpace(deviceID) + "\x00" + strings.TrimSpace(deliveryID)
}

func logicalMQTTControlDeliveryState(delivery MQTTControlDelivery, now int64) MQTTControlDelivery {
	if delivery.ExpiresAt <= 0 || now < delivery.ExpiresAt {
		return delivery
	}
	if delivery.Status != "pending" && delivery.Status != "retrying" && delivery.Status != "published" {
		return delivery
	}
	delivery.Status = "expired"
	return delivery
}

func controlDeliveryStorePathFromEnv() string {
	dir := strings.TrimSpace(os.Getenv("SLAN_BIZ_STATE_DIR"))
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "mqtt-control-deliveries.json")
}

func (s *Store) loadControlDeliveriesLocked() error {
	if strings.TrimSpace(s.controlDeliveryStorePath) == "" {
		return nil
	}
	payload, err := os.ReadFile(s.controlDeliveryStorePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var deliveries []MQTTControlDelivery
	if err := json.Unmarshal(payload, &deliveries); err != nil {
		return err
	}
	for _, delivery := range deliveries {
		if delivery.DeviceID == "" || delivery.DeliveryID == "" {
			continue
		}
		s.controlDeliveries[controlDeliveryKey(delivery.DeviceID, delivery.DeliveryID)] = delivery
	}
	return nil
}

func (s *Store) persistControlDeliveriesLocked() error {
	if strings.TrimSpace(s.controlDeliveryStorePath) == "" {
		return nil
	}
	deliveries := make([]MQTTControlDelivery, 0, len(s.controlDeliveries))
	for _, delivery := range s.controlDeliveries {
		deliveries = append(deliveries, delivery)
	}
	sort.Slice(deliveries, func(i, j int) bool {
		if deliveries[i].DeviceID == deliveries[j].DeviceID {
			return deliveries[i].DeliveryID < deliveries[j].DeliveryID
		}
		return deliveries[i].DeviceID < deliveries[j].DeviceID
	})
	payload, err := json.MarshalIndent(deliveries, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.controlDeliveryStorePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmpPath := s.controlDeliveryStorePath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.controlDeliveryStorePath)
}

func (s *Store) ListIPAMSubnets() []IPAMSubnet {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres ipam subnets failed: %v", err)
	}
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
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Network{}, SecurityGroup{}, NetworkDNSZone{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresNetworkCreateTxLocked(ctx, postgresTx, network, group, zone); err != nil {
		return Network{}, SecurityGroup{}, NetworkDNSZone{}, err
	}
	postgresTx = nil
	return network, group, zone, nil
}

func (s *Store) ListNetworks(userID string) []Network {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres networks failed: %v", err)
	}
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
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Network{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresNetworkUpdateTxLocked(ctx, postgresTx, network); err != nil {
		return Network{}, err
	}
	postgresTx = nil
	return network, nil
}

func (s *Store) UpdateNetworkFull(networkID, name, code, status string) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Network{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresNetworkUpdateTxLocked(ctx, postgresTx, network); err != nil {
		return Network{}, err
	}
	postgresTx = nil
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
	defer s.mu.Unlock()
	s.deviceInvites[invite.InviteCode] = invite
	return invite, nil
}

func (s *Store) ListDeviceInvites(inviterUserID string) []DeviceInvite {
	inviterUserID = strings.TrimSpace(inviterUserID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres device invites failed: %v", err)
	}
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
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return DeviceAccessGrant{}, DeviceInvite{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresDeviceInviteAcceptTxLocked(ctx, postgresTx, invite, grant); err != nil {
		return DeviceAccessGrant{}, DeviceInvite{}, err
	}
	postgresTx = nil
	return grant, invite, nil
}

func (s *Store) AddNetworkDevice(networkID, deviceID, actorUserID, alias string, enabled bool) (NetworkDevice, error) {
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return NetworkDevice{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDevice{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	networkDevice := s.addNetworkDeviceLocked(networkID, deviceID, device.OwnerID, alias, enabled, time.Now().Unix())
	if err := s.persistPostgresNetworkDeviceUpsertTxLocked(ctx, postgresTx, networkDevice); err != nil {
		return NetworkDevice{}, err
	}
	postgresTx = nil
	return networkDevice, nil
}

func (s *Store) UpdateNetworkDevice(networkID, deviceID, alias string, enabled *bool) (NetworkDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDevice{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresNetworkDeviceUpsertTxLocked(ctx, postgresTx, networkDevice); err != nil {
		return NetworkDevice{}, err
	}
	postgresTx = nil
	return networkDevice, nil
}

func (s *Store) RemoveNetworkDevice(networkID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	key := networkID + "|" + deviceID
	if _, ok := s.networkDevices[key]; !ok {
		return errNotFound
	}
	delete(s.networkDevices, key)
	if err := s.persistPostgresNetworkDeviceDeleteTxLocked(ctx, postgresTx, networkID, deviceID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) ListNetworkDevices(networkID string) []NetworkDevice {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres network devices failed: %v", err)
	}
	out := make([]NetworkDevice, 0)
	for _, device := range s.networkDevices {
		if networkID == "" || device.NetworkID == networkID {
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

func (s *Store) ListNetworkDevicesForDevice(deviceID string) []NetworkDevice {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres network devices for device failed: %v", err)
	}
	out := make([]NetworkDevice, 0)
	for _, device := range s.networkDevices {
		if device.DeviceID == deviceID {
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NetworkID == out[j].NetworkID {
			return out[i].NetworkDeviceID < out[j].NetworkDeviceID
		}
		return out[i].NetworkID < out[j].NetworkID
	})
	return out
}

func (s *Store) ListNetworkDevicesForUser(userID string) []NetworkDevice {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres network devices for user failed: %v", err)
	}
	out := make([]NetworkDevice, 0)
	for _, device := range s.networkDevices {
		if device.OwnerUserID == userID {
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NetworkID == out[j].NetworkID {
			if out[i].DeviceID == out[j].DeviceID {
				return out[i].NetworkDeviceID < out[j].NetworkDeviceID
			}
			return out[i].DeviceID < out[j].DeviceID
		}
		return out[i].NetworkID < out[j].NetworkID
	})
	return out
}

func (s *Store) AddDNSZone(networkID, zoneName string, exposeGlobal bool) (NetworkDNSZone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDNSZone{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.networks[networkID]; !ok {
		return NetworkDNSZone{}, errNotFound
	}
	zone := s.addDNSZoneLocked(networkID, zoneName, exposeGlobal, time.Now().Unix())
	if err := s.persistPostgresDNSZoneUpsertTxLocked(ctx, postgresTx, zone); err != nil {
		return NetworkDNSZone{}, err
	}
	postgresTx = nil
	return zone, nil
}

func (s *Store) UpdateDNSZone(networkID, zoneID, zoneName string, exposeGlobal bool) (NetworkDNSZone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDNSZone{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	zone, ok := s.dnsZones[zoneID]
	if !ok || zone.NetworkID != networkID {
		return NetworkDNSZone{}, errNotFound
	}
	if zoneName = strings.TrimSpace(zoneName); zoneName != "" {
		zone.ZoneName = zoneName
	}
	zone.ExposeGlobal = exposeGlobal
	s.dnsZones[zoneID] = zone
	if err := s.persistPostgresDNSZoneUpsertTxLocked(ctx, postgresTx, zone); err != nil {
		return NetworkDNSZone{}, err
	}
	postgresTx = nil
	return zone, nil
}

func (s *Store) DeleteDNSZone(networkID, zoneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresDNSZoneDeleteTxLocked(ctx, postgresTx, networkID, zoneID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) AddDNSRecord(networkID, zoneID, name, recordType, targetDeviceID, targetIP, cname, port string, ttl int) (NetworkDNSRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return NetworkDNSRecord{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDNSRecord{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresDNSRecordUpsertTxLocked(ctx, postgresTx, record); err != nil {
		return NetworkDNSRecord{}, err
	}
	postgresTx = nil
	return record, nil
}

func (s *Store) UpdateDNSRecord(networkID, recordID, name, recordType, targetDeviceID, targetIP, cname, port string, ttl int) (NetworkDNSRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return NetworkDNSRecord{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDNSRecord{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresDNSRecordUpsertTxLocked(ctx, postgresTx, record); err != nil {
		return NetworkDNSRecord{}, err
	}
	postgresTx = nil
	return record, nil
}

func (s *Store) DeleteDNSRecord(networkID, recordID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	record, ok := s.dnsRecords[recordID]
	if !ok || record.NetworkID != networkID {
		return errNotFound
	}
	delete(s.dnsRecords, recordID)
	if err := s.persistPostgresDNSRecordDeleteTxLocked(ctx, postgresTx, networkID, recordID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) ListDNSZones(networkID string) []NetworkDNSZone {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres dns zones failed: %v", err)
	}
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres dns records failed: %v", err)
	}
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres public mappings failed: %v", err)
	}
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
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return PublicDomainMapping{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresPublicMappingUpsertTxLocked(ctx, postgresTx, mapping); err != nil {
		return PublicDomainMapping{}, err
	}
	postgresTx = nil
	return mapping, nil
}

func (s *Store) DeletePublicMapping(networkID, mappingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	mapping, ok := s.publicMappings[mappingID]
	if !ok || mapping.NetworkID != networkID {
		return errNotFound
	}
	delete(s.publicMappings, mappingID)
	if err := s.persistPostgresPublicMappingDeleteTxLocked(ctx, postgresTx, networkID, mappingID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) CreateSecurityGroup(networkID, name, description, defaultPolicy string) (SecurityGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return SecurityGroup{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.networks[networkID]; !ok {
		return SecurityGroup{}, errNotFound
	}
	group := s.addSecurityGroupLocked(networkID, name, description, defaultString(defaultPolicy, "deny"), time.Now().Unix())
	if err := s.persistPostgresSecurityGroupUpsertTxLocked(ctx, postgresTx, group); err != nil {
		return SecurityGroup{}, err
	}
	postgresTx = nil
	return group, nil
}

func (s *Store) DeleteSecurityGroup(networkID, securityGroupID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresSecurityGroupDeleteTxLocked(ctx, postgresTx, networkID, securityGroupID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) AddSecurityGroupRule(securityGroupID, direction, action, protocol, peerType, peerValue, description string, priority, portFrom, portTo int, enabled bool) (SecurityGroupRule, error) {
	if securityGroupID == "" || direction == "" || action == "" {
		return SecurityGroupRule{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return SecurityGroupRule{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresSecurityGroupRuleUpsertTxLocked(ctx, postgresTx, rule); err != nil {
		return SecurityGroupRule{}, err
	}
	postgresTx = nil
	return rule, nil
}

func (s *Store) UpdateSecurityGroupRule(ruleID, direction, action, protocol, peerType, peerValue, description string, priority, portFrom, portTo int, enabled bool) (SecurityGroupRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return SecurityGroupRule{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
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
	if err := s.persistPostgresSecurityGroupRuleUpsertTxLocked(ctx, postgresTx, rule); err != nil {
		return SecurityGroupRule{}, err
	}
	postgresTx = nil
	return rule, nil
}

func (s *Store) DeleteSecurityGroupRule(ruleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.securityGroupRules[ruleID]; !ok {
		return errNotFound
	}
	delete(s.securityGroupRules, ruleID)
	if err := s.persistPostgresSecurityGroupRuleDeleteTxLocked(ctx, postgresTx, ruleID); err != nil {
		return err
	}
	postgresTx = nil
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return SecurityGroupRule{}, err
	}
	rule, ok := s.securityGroupRules[ruleID]
	if !ok {
		return SecurityGroupRule{}, errNotFound
	}
	return rule, nil
}

func (s *Store) ListSecurityGroups(networkID string) []SecurityGroup {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres security groups failed: %v", err)
	}
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres security rules failed: %v", err)
	}
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres active devices failed: %v", err)
	}
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres active device failed: %v", err)
		return false
	}
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres global dns failed: %v", err)
	}
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
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return NetworkConfig{}, err
	}
	return s.networkConfigLocked(networkID, deviceID)
}

func (s *Store) NetworkConfigsForDevice(deviceID string) ([]NetworkConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return nil, err
	}
	return s.networkConfigsForDeviceLocked(deviceID)
}

func (s *Store) networkConfigsForDeviceLocked(deviceID string) ([]NetworkConfig, error) {
	device, ok := s.devices[deviceID]
	if !ok {
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
	if len(networkIDs) == 0 && strings.TrimSpace(device.OwnerID) != "" {
		defaultNetwork := s.ensureDefaultNetworkForUserLocked(device.OwnerID, time.Now().Unix())
		membership := s.addNetworkDeviceLocked(defaultNetwork.NetworkID, deviceID, device.OwnerID, device.Alias, true, time.Now().Unix())
		if membership.Status == "active" && membership.Enabled {
			networkIDs = append(networkIDs, defaultNetwork.NetworkID)
		}
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
	now := time.Now().Unix()
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
		if network.OwnerUserID != "" && network.OwnerUserID == device.OwnerID {
			membership = s.addNetworkDeviceLocked(networkID, deviceID, device.OwnerID, device.Alias, true, now)
		}
		if !membership.Enabled || membership.Status != "active" {
			return NetworkConfig{}, errNotFound
		}
	}
	s.ensureUniqueGlobalIPsLocked(now)
	device = s.devices[deviceID]
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
	globalIP := hostIP(device.GlobalIP)
	subnet, _ := s.globalIPSubnetLocked(globalIP)
	return NetworkConfig{
		NetworkID:       networkID,
		NetworkName:     network.Name,
		NetworkCode:     network.Code,
		ConfigVersion:   s.currentNetworkConfigVersionLocked(networkID),
		DeviceID:        deviceID,
		GlobalIP:        globalIP,
		PrefixLen:       subnet.PrefixLength,
		GlobalCIDR:      ipamGlobalCIDR,
		SubnetID:        subnet.SubnetID,
		SubnetCIDR:      subnet.CIDRBlock,
		SubnetPrefixLen: subnet.PrefixLength,
		GlobalName:      device.GlobalName,
		Peers:           s.devicesWithSubnetLocked(networkID, peers),
		SecurityGroups:  groups,
		Rules:           rules,
		DNSZones:        s.listDNSZonesLocked(networkID),
		DNSRecords:      s.listDNSRecordsLocked(networkID),
		RelayCandidates: relayCandidates,
	}, nil
}

func (s *Store) globalIPSubnetLocked(ip string) (IPAMSubnet, bool) {
	ip = hostIP(ip)
	if ip == "" {
		return IPAMSubnet{}, false
	}
	address, ok := s.globalIPs[ip]
	if !ok {
		return IPAMSubnet{}, false
	}
	subnet, ok := s.ipamSubnets[address.SubnetID]
	return subnet, ok
}

func (s *Store) devicesWithSubnetLocked(networkID string, devices []Device) []Device {
	out := make([]Device, 0, len(devices))
	for _, device := range devices {
		withSubnet := s.deviceWithSubnetLocked(device)
		withSubnet.Endpoints = s.deviceEndpointsForNetworkDeviceLocked(networkID, device.DeviceID, time.Now().Unix())
		out = append(out, withSubnet)
	}
	return out
}

func (s *Store) deviceEndpointsForNetworkDeviceLocked(networkID, deviceID string, now int64) []DeviceEndpoint {
	const endpointTTLSeconds = 5 * 60
	out := make([]DeviceEndpoint, 0)
	for _, endpoint := range s.deviceEndpoints[networkID+"|"+deviceID] {
		if endpoint.UpdatedAt > 0 && now-endpoint.UpdatedAt > endpointTTLSeconds {
			continue
		}
		out = append(out, endpoint)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type == out[j].Type {
			return out[i].Address < out[j].Address
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func hostIP(value string) string {
	value = strings.TrimSpace(value)
	if ip, _, ok := strings.Cut(value, "/"); ok {
		return strings.TrimSpace(ip)
	}
	return value
}

func (s *Store) peerRuntimeActiveLocked(deviceID string, now int64) bool {
	status := s.runtimeStatuses[deviceID]
	if !status.DeviceEnabled || !status.NetworkEnabled {
		return false
	}
	lastSeen := status.LastSeenAt
	if lastSeen == 0 {
		lastSeen = status.LastReportAt
	}
	return lastSeen > 0 && now-lastSeen <= int64(activePeerTTL.Seconds())
}

func (s *Store) createSessionLocked(userID string, now int64) UserSession {
	sessionSeq := s.nextUserSessionSeq
	s.nextUserSessionSeq++
	session := UserSession{
		SessionID: fmt.Sprintf("session-%06d", sessionSeq),
		UserID:    userID,
		Token:     fmt.Sprintf("token-%s-%d-%06d", userID, now, sessionSeq),
		CreatedAt: now,
		ExpiresAt: now + int64(userSessionTTL.Seconds()),
	}
	s.sessions[session.Token] = session
	return session
}

func (s *Store) ensureDefaultNetworkForUserLocked(userID string, now int64) Network {
	network, _, _ := s.ensureDefaultNetworkResourcesForUserLocked(userID, now)
	return network
}

func (s *Store) ensureDefaultNetworkResourcesForUserLocked(userID string, now int64) (Network, SecurityGroup, NetworkDNSZone) {
	id := "default-" + userID
	if network, ok := s.networks[id]; ok {
		var group SecurityGroup
		for _, item := range s.securityGroups {
			if item.NetworkID == network.NetworkID {
				group = item
				break
			}
		}
		var zone NetworkDNSZone
		for _, item := range s.dnsZones {
			if item.NetworkID == network.NetworkID {
				zone = item
				break
			}
		}
		return network, group, zone
	}
	network := Network{NetworkID: id, OwnerUserID: userID, Name: "默认网络", Code: "default", TemplateKey: "default", Status: "enabled", Default: true, CreatedAt: now, UpdatedAt: now}
	s.networks[id] = network
	group := s.addSecurityGroupLocked(id, "默认安全组", "默认网络安全组", "deny", now)
	zone := s.addDNSZoneLocked(id, networkZoneName(network), false, now)
	return network, group, zone
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

func (s *Store) bindExistingDeviceToUser(deviceID, userID string) error {
	deviceID = strings.TrimSpace(deviceID)
	userID = strings.TrimSpace(userID)
	if deviceID == "" || userID == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok {
		return errNotFound
	}
	if _, ok := s.users[userID]; !ok {
		return errNotFound
	}
	now := time.Now().Unix()
	previousOwnerID := ""
	if strings.TrimSpace(device.OwnerID) == "" {
		device.OwnerID = userID
		if strings.TrimSpace(device.GlobalIP) == "" {
			device.GlobalIP = s.allocateGlobalIPLocked(deviceID, now)
		}
		device.GlobalName = sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain()
		device.UpdatedAt = now
		s.devices[deviceID] = device
		s.addDeviceOwnerLocked(deviceID, userID, now)
	} else if device.OwnerID != userID {
		return errConflict
	}
	network := s.ensureDefaultNetworkForUserLocked(userID, now)
	membership := NetworkDevice{
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
	s.networkDevices[network.NetworkID+"|"+deviceID] = membership
	if err := s.persistPostgresDeviceRegisterTxLocked(ctx, postgresTx, device, membership, previousOwnerID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
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
	return s.deviceWithSubnetLocked(device)
}

func (s *Store) deviceWithSubnetLocked(device Device) Device {
	globalIP := hostIP(device.GlobalIP)
	if globalIP == "" {
		return device
	}
	device.GlobalIP = globalIP
	subnet, ok := s.globalIPSubnetLocked(globalIP)
	if !ok {
		return device
	}
	device.PrefixLen = subnet.PrefixLength
	device.GlobalCIDR = ipamGlobalCIDR
	device.SubnetID = subnet.SubnetID
	device.SubnetCIDR = subnet.CIDRBlock
	device.SubnetPrefixLen = subnet.PrefixLength
	return device
}

func (s *Store) allocateGlobalIPLocked(deviceID string, now int64) string {
	return s.allocateGlobalIPExcludingLocked(deviceID, now, nil)
}

func (s *Store) allocateGlobalIPExcludingLocked(deviceID string, now int64, reserved map[string]bool) string {
	s.ensureIPPoolLocked(now)
	var selected GlobalIPAddress
	for _, address := range s.globalIPs {
		if address.Status != "available" {
			continue
		}
		if reserved != nil && reserved[address.IP] {
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

func (s *Store) ensureUniqueGlobalIPsLocked(now int64) {
	deviceIDs := make([]string, 0, len(s.devices))
	for deviceID := range s.devices {
		deviceIDs = append(deviceIDs, deviceID)
	}
	sort.Strings(deviceIDs)

	used := make(map[string]bool, len(deviceIDs))
	reassign := make([]string, 0)
	for _, deviceID := range deviceIDs {
		device := s.devices[deviceID]
		ip := strings.TrimSpace(device.GlobalIP)
		if ip == "" || used[ip] {
			reassign = append(reassign, deviceID)
			continue
		}
		used[ip] = true
		if address, ok := s.globalIPs[ip]; ok {
			address.DeviceID = deviceID
			address.Status = "assigned"
			if address.AssignedAt == 0 {
				address.AssignedAt = now
			}
			address.ReleasedAt = 0
			s.globalIPs[ip] = address
		}
	}

	for _, deviceID := range reassign {
		device := s.devices[deviceID]
		oldIP := strings.TrimSpace(device.GlobalIP)
		if oldIP != "" {
			if address, ok := s.globalIPs[oldIP]; ok && address.DeviceID == deviceID {
				address.DeviceID = ""
				address.Status = "available"
				address.ReleasedAt = now
				s.globalIPs[oldIP] = address
			}
		}
		newIP := s.allocateGlobalIPExcludingLocked(deviceID, now, used)
		device.GlobalIP = newIP
		device.UpdatedAt = now
		s.devices[deviceID] = device
		if newIP != "" {
			used[newIP] = true
		}
	}

	for ip, address := range s.globalIPs {
		if address.Status != "assigned" || address.DeviceID == "" {
			continue
		}
		device, ok := s.devices[address.DeviceID]
		if ok && strings.TrimSpace(device.GlobalIP) == ip {
			continue
		}
		address.DeviceID = ""
		address.Status = "available"
		address.ReleasedAt = now
		s.globalIPs[ip] = address
	}
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
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err == nil {
		return string(hashed)
	}
	return legacyPasswordHash(password)
}

func verifyPasswordHash(storedHash, password string) bool {
	storedHash = strings.TrimSpace(storedHash)
	if strings.HasPrefix(storedHash, "$2a$") || strings.HasPrefix(storedHash, "$2b$") || strings.HasPrefix(storedHash, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}
	return storedHash == legacyPasswordHash(password)
}

func legacyPasswordHash(password string) string {
	sum := sha256.Sum256([]byte("slan:" + password))
	return hex.EncodeToString(sum[:])
}

func bootstrapKeyHash(key string) string {
	sum := sha256.Sum256([]byte("slan:device-bootstrap:" + strings.TrimSpace(key)))
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

func globalDeviceDomain() string {
	domain := strings.Trim(strings.TrimSpace(os.Getenv("SLAN_GLOBAL_DEVICE_DOMAIN")), ".")
	if domain == "" {
		return "staticlss.com"
	}
	return domain
}
