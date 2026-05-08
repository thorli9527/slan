export type NavItem = { id: string; label: string; desc: string };
export type NavGroup = { title: string; items: NavItem[] };

export type DeviceRow = {
  deviceId: string;
  platform: string;
  osVersion: string;
  alias: string;
  ip: string;
  owner: string;
  status: string;
};

export type WorkspaceRow = {
  workspaceId: string;
  name: string;
  code: string;
  template: string;
  status: string;
  members: number;
  devices: number;
  zone: string;
};

export type MemberRow = { user: string; alias: string; role: string; status: string };
export type UserAliasRow = { email: string; alias: string };
export type DNSZoneRow = { zoneId?: string; workspaceId: string; zone: string; recordType: string; value: string; expose: boolean; status: string };
export type DNSRow = { recordId?: string; zoneId?: string; workspaceId: string; name: string; fqdn: string; recordType: string; value: string; deviceId: string; port: string; expose: boolean };
export type PublicMappingRow = {
  mappingId?: string;
  workspaceId: string;
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
};
export type RuleSubjectType = 'device' | 'user' | 'workspace' | 'cidr' | 'domain' | 'all';
export type SecurityRuleRow = { ruleId?: string; direction: string; priority: number; action: string; protocol: string; port: string; subjectType: RuleSubjectType; subjectValue: string };
export type SecurityGroupRow = { securityGroupId: string; workspaceId: string; name: string; defaultPolicy: string; status: string };
export type DeviceExposureRow = { deviceId: string; user: string; alias: string; status: string };
export type WorkspaceDeviceInviteRow = {
  inviteId: string;
  workspaceId?: string;
  inviterUserId: string;
  inviteCode: string;
  status: string;
  createdAt: number;
  expiresAt: number;
  acceptedDeviceId?: string;
  acceptedUserId?: string;
  acceptedAt?: number;
};
export type WorkspacePanel = 'zones' | 'records' | 'publicMappings' | 'securityGroups' | 'securityRules';
export type WorkspacePreset = { name: string; code: string };
export type SecurityRuleTemplate = {
  name: string;
  direction: string;
  priority: number;
  action: string;
  protocol: string;
  port: string;
  subjectType: RuleSubjectType;
  subjectValue: string;
};

export type ApiDevice = {
  deviceId: string;
  platform: string;
  osVersion?: string;
  alias?: string;
  name?: string;
  globalIp: string;
  ownerId: string;
  status: string;
};

export type ApiWorkspace = {
  workspaceId: string;
  name: string;
  code?: string;
  templateKey?: string;
  status: string;
};

export type ApiWorkspaceDevice = {
  workspaceDeviceId: string;
  workspaceId: string;
  deviceId: string;
  ownerUserId: string;
  alias?: string;
  enabled: boolean;
  status: string;
};

export type ApiDNSZone = {
  zoneId: string;
  workspaceId: string;
  zoneName: string;
  exposeGlobal: boolean;
  status: string;
};

export type ApiDNSRecord = {
  recordId: string;
  zoneId: string;
  workspaceId: string;
  name: string;
  fqdn: string;
  recordType: string;
  targetDeviceId?: string;
  targetIp?: string;
  cname?: string;
  port?: string;
  status: string;
};

export type ApiPublicMapping = {
  mappingId: string;
  workspaceId: string;
  alias: string;
  publicDomain: string;
  sourceRecord: string;
  deviceId: string;
  protocol: string;
  port: string;
  externalPort: string;
  status: string;
};

export type ApiSecurityGroup = {
  securityGroupId: string;
  workspaceId: string;
  name: string;
  defaultPolicy: string;
  status: string;
};

export type ApiSecurityRule = {
  ruleId: string;
  direction: string;
  priority: number;
  action: string;
  protocol: string;
  portFrom: number;
  portTo: number;
  peerType: RuleSubjectType;
  peerValue: string;
  enabled: boolean;
};
