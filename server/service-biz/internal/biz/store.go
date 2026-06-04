package biz

import (
	"context"
	"database/sql"
	"errors"
	"log"
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
