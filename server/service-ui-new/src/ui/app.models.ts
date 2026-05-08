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
export type DNSZoneRow = { workspaceId: string; zone: string; recordType: string; value: string; expose: boolean; status: string };
export type DNSRow = { workspaceId: string; name: string; fqdn: string; recordType: string; value: string; deviceId: string; port: string; expose: boolean };
export type PublicMappingRow = {
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
export type SecurityRuleRow = { direction: string; priority: number; action: string; protocol: string; port: string; subjectType: RuleSubjectType; subjectValue: string };
export type DeviceExposureRow = { deviceId: string; user: string; alias: string; status: string };
export type WorkspaceDeviceInviteRow = {
  inviteId: string;
  workspaceId: string;
  inviterUserId: string;
  inviteCode: string;
  status: string;
  createdAt: number;
  expiresAt: number;
  acceptedDeviceId?: string;
  acceptedUserId?: string;
  acceptedAt?: number;
};
export type WorkspacePanel = 'devices' | 'zones' | 'records' | 'publicMappings' | 'securityGroups' | 'securityRules';
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
