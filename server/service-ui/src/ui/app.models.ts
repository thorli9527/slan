export type NavItem = { id: string; label: string; desc: string };
export type NavGroup = { title: string; items: NavItem[] };

// Server API models. Keep these field names aligned with service-biz/internal/biz/models.go.
export type ApiUser = {
  userId: string;
  email: string;
  name?: string;
  status: string;
  createdAt: number;
  updatedAt: number;
};

export type ApiUserSession = {
  sessionId: string;
  userId: string;
  token: string;
  createdAt: number;
  expiresAt: number;
};

export type ApiAuthResponse = {
  user: ApiUser;
  session: ApiUserSession;
};

export type ApiDevice = {
  deviceId: string;
  ownerId: string;
  ownerEmail?: string;
  name: string;
  platform: string;
  osName?: string;
  osVersion?: string;
  alias?: string;
  publicKey?: string;
  globalIp: string;
  globalName: string;
  status: string;
  createdAt: number;
  updatedAt: number;
};

export type ClientDownload = {
  downloadId: string;
  platform: string;
  platformName: string;
  version: string;
  arch?: string;
  channel: string;
  fileName: string;
  fileSize: number;
  sha256?: string;
  downloadUrl: string;
  releaseNotes?: string;
  status: string;
  createdAt: number;
  updatedAt: number;
};

export type ApiUserAlias = {
  ownerUserId: string;
  email: string;
  alias: string;
  updatedAt: number;
};

export type ApiDeviceOwner = {
  ownerRecordId: string;
  deviceId: string;
  userId: string;
  status: string;
  boundAt: number;
  unboundAt?: number;
};

export type ApiNetwork = {
  networkId: string;
  ownerUserId: string;
  name: string;
  code: string;
  templateKey?: string;
  status: string;
  default: boolean;
  createdAt: number;
  updatedAt: number;
};

export type ApiDeviceInvite = {
  inviteId: string;
  inviterUserId?: string;
  inviteCode: string;
  status: string;
  createdAt: number;
  expiresAt: number;
  acceptedDeviceId?: string;
  acceptedUserId?: string;
  acceptedAt?: number;
};

export type ApiDeviceBootstrapKey = {
  id: string;
  key?: string;
  createdByUserId: string;
  networkId: string;
  deviceAlias?: string;
  expiresAt: number;
  usedAt?: number;
  usedByDeviceId?: string;
  revokedAt?: number;
  status: string;
  createdAt: number;
};

export type ApiDeviceAccessGrant = {
  grantId: string;
  deviceId: string;
  userId: string;
  grantedBy?: string;
  inviteCode?: string;
  status: string;
  createdAt: number;
};

export type ApiNetworkDevice = {
  networkDeviceId: string;
  networkId: string;
  deviceId: string;
  ownerUserId: string;
  alias?: string;
  enabled: boolean;
  status: string;
  createdAt: number;
  updatedAt: number;
};

export type ApiNetworkDNSZone = {
  zoneId: string;
  networkId: string;
  zoneName: string;
  exposeGlobal: boolean;
  status: string;
  createdAt: number;
};

export type ApiNetworkDNSRecord = {
  recordId: string;
  zoneId: string;
  networkId: string;
  name: string;
  fqdn: string;
  recordType: string;
  targetDeviceId?: string;
  targetIp?: string;
  cname?: string;
  port?: string;
  ttl: number;
  status: string;
  createdAt: number;
};

export type ApiPublicDomainMapping = {
  mappingId: string;
  networkId: string;
  alias: string;
  publicDomain: string;
  sourceRecord: string;
  deviceId: string;
  protocol: string;
  port: string;
  externalPort: string;
  status: string;
  createdAt: number;
  updatedAt: number;
};

export type ApiSecurityGroup = {
  securityGroupId: string;
  networkId: string;
  name: string;
  description?: string;
  status: string;
  createdAt: number;
};

export type ApiSecurityGroupRule = {
  ruleId: string;
  securityGroupId: string;
  direction: string;
  priority: number;
  action: string;
  protocol: string;
  portFrom: number;
  portTo: number;
  peerType: RuleSubjectType;
  peerValue: string;
  description?: string;
  enabled: boolean;
  createdAt: number;
};

export type ApiDeviceRuntimeStatus = {
  deviceId: string;
  heartbeatOnline: boolean;
  networkEnabled: boolean;
  deviceEnabled: boolean;
  rxBytesTotal: number;
  txBytesTotal: number;
  lastSeenAt: number;
  lastReportAt: number;
};

export type ApiDeviceQuota = {
  userId: string;
  planCode: string;
  planName: string;
  ownDeviceLimit: number;
  invitedDeviceLimit: number;
  totalDeviceLimit: number;
  ownDevices: number;
  invitedDevices: number;
  totalDevices: number;
  remainingDevices: number;
  planExpiresAt?: number;
  status: string;
};

export type ApiNetworkConfig = {
  networkId: string;
  networkName?: string;
  networkCode?: string;
  configVersion?: number;
  deviceId: string;
  globalIp: string;
  globalName: string;
  peers: ApiDevice[];
  securityGroups: ApiSecurityGroup[];
  rules: ApiSecurityGroupRule[];
  dnsZones: ApiNetworkDNSZone[];
  dnsRecords: ApiNetworkDNSRecord[];
};

export type ApiRelayCandidate = {
  endpointId: string;
  transport: string;
  address: string;
  countryCode?: string;
  regionId?: string;
  clusterId?: string;
};

export type ApiRelayTicket = {
  ticketId: string;
  networkId: string;
  sessionId: string;
  srcNodeId: string;
  dstNodeId: string;
  derpClusterId?: string;
  countryCode?: string;
  cityCode?: string;
  allowedDerpNodeIds: string[];
  relayUrl: string;
  expiresAt: string;
  sessionKey: string;
  signature: string;
};

// UI view models. These are derived from server API models for display/editing.
export type DeviceRow = {
  deviceId: string;
  platform: string;
  osVersion: string;
  alias: string;
  ip: string;
  owner: string;
  status: string;
};

export type NetworkRow = {
  networkId: string;
  name: string;
  code: string;
  template: string;
  status: string;
  devices: number;
  zone: string;
  // Compatibility aliases used by existing templates while the UI is being renamed from workspace to network.
  workspaceId: string;
  members: number;
};

export type MemberRow = { user: string; alias: string; role: string; status: string };
export type UserAliasRow = { email: string; alias: string };
export type DNSZoneRow = { zoneId?: string; networkId: string; zone: string; recordType: string; value: string; expose: boolean; status: string; workspaceId: string };
export type DNSRow = { recordId?: string; zoneId?: string; networkId: string; name: string; fqdn: string; recordType: string; value: string; deviceId: string; port: string; expose: boolean; workspaceId: string };
export type PublicMappingRow = {
  mappingId?: string;
  networkId: string;
  alias: string;
  publicDomain: string;
  sourceRecord: string;
  deviceId: string;
  protocol: string;
  port: string;
  externalPort: string;
  accessMode: string;
  tlsMode: string;
  status: string;
  workspaceId: string;
};
export type RuleSubjectType = 'device' | 'user' | 'network' | 'workspace' | 'cidr' | 'domain' | 'all';
export type SecurityRuleRow = { ruleId?: string; securityGroupId?: string; direction: string; priority: number; action: string; protocol: string; port: string; subjectType: RuleSubjectType; subjectValue: string };
export type SecurityGroupRow = { securityGroupId: string; networkId: string; name: string; status: string; workspaceId: string };
export type DeviceExposureRow = { deviceId: string; user: string; alias: string; status: string };
export type NetworkDeviceInviteRow = ApiDeviceInvite & { networkId?: string; workspaceId?: string };
export type NetworkPanel = 'devices' | 'zones' | 'records' | 'publicMappings' | 'securityGroups' | 'securityRules';
export type NetworkPreset = { name: string; code: string };
export type SecurityRuleTemplate = {
  name: string;
  description?: string;
  direction: string;
  priority: number;
  action: string;
  protocol: string;
  port: string;
  subjectType: RuleSubjectType;
  subjectValue: string;
};

// Compatibility aliases. Prefer Network* and ApiNetwork* in new code.
export type WorkspaceRow = NetworkRow;
export type WorkspaceDeviceInviteRow = NetworkDeviceInviteRow;
export type WorkspacePanel = NetworkPanel;
export type WorkspacePreset = NetworkPreset;
export type ApiWorkspace = ApiNetwork & { workspaceId?: string };
export type ApiWorkspaceDevice = ApiNetworkDevice & { workspaceDeviceId?: string; workspaceId?: string };
export type ApiDNSZone = ApiNetworkDNSZone & { workspaceId?: string };
export type ApiDNSRecord = ApiNetworkDNSRecord & { workspaceId?: string };
export type ApiPublicMapping = ApiPublicDomainMapping & { workspaceId?: string };
export type ApiSecurityRule = ApiSecurityGroupRule;
