import {
  DeviceExposureRow,
  DeviceRow,
  DNSRow,
  DNSZoneRow,
  MemberRow,
  NavGroup,
  PublicMappingRow,
  SecurityRuleRow,
  SecurityRuleTemplate,
  UserAliasRow,
  WorkspaceDeviceInviteRow,
  WorkspacePreset,
  WorkspaceRow,
} from './app.models';

export const ROOT_DOMAIN = 'vlan.com';

export const WORKSPACE_PRESETS: WorkspacePreset[] = [
  { name: '默认网络', code: 'default' },
  { name: '开发组', code: 'dev' },
  { name: '工作部', code: 'eng' },
  { name: '运营部', code: 'ops' },
  { name: '测试组', code: 'test' },
  { name: '财务部', code: 'finance' },
  { name: '销售部', code: 'sales' },
];

export const SECURITY_RULE_TEMPLATES: SecurityRuleTemplate[] = [
  { name: 'SSH 登录', description: 'Linux 运维登录，TCP 22', direction: 'ingress', priority: 100, action: 'allow', protocol: 'tcp', port: '22', subjectType: 'workspace', subjectValue: 'self' },
  { name: 'RDP 远程桌面', description: 'Windows 远程桌面，TCP 3389', direction: 'ingress', priority: 100, action: 'allow', protocol: 'tcp', port: '3389', subjectType: 'workspace', subjectValue: 'self' },
  { name: 'HTTP 服务', description: 'Web 明文访问，TCP 80', direction: 'ingress', priority: 100, action: 'allow', protocol: 'tcp', port: '80', subjectType: 'workspace', subjectValue: 'self' },
  { name: 'HTTPS 服务', description: 'Web 加密访问，TCP 443', direction: 'ingress', priority: 100, action: 'allow', protocol: 'tcp', port: '443', subjectType: 'workspace', subjectValue: 'self' },
  { name: 'Web 服务', description: '同时开放 HTTP/HTTPS', direction: 'ingress', priority: 100, action: 'allow', protocol: 'tcp', port: '80,443', subjectType: 'workspace', subjectValue: 'self' },
  { name: 'MySQL', description: '数据库访问，TCP 3306', direction: 'ingress', priority: 120, action: 'allow', protocol: 'tcp', port: '3306', subjectType: 'workspace', subjectValue: 'self' },
  { name: 'Redis', description: '缓存访问，TCP 6379', direction: 'ingress', priority: 120, action: 'allow', protocol: 'tcp', port: '6379', subjectType: 'workspace', subjectValue: 'self' },
  { name: 'ICMP Ping', description: '允许网络探测', direction: 'ingress', priority: 110, action: 'allow', protocol: 'icmp', port: 'all', subjectType: 'workspace', subjectValue: 'self' },
  { name: '内网互通', description: '当前网络内全部协议互通', direction: 'ingress', priority: 100, action: 'allow', protocol: 'all', port: 'all', subjectType: 'workspace', subjectValue: 'self' },
  { name: '全部出站', direction: 'egress', priority: 100, action: 'allow', protocol: 'all', port: 'all', subjectType: 'all', subjectValue: 'all' },
  { name: '拒绝全部入站', description: '兜底拒绝所有来源', direction: 'ingress', priority: 900, action: 'deny', protocol: 'all', port: 'all', subjectType: 'cidr', subjectValue: '0.0.0.0/0' },
];

export const NAV_GROUPS: NavGroup[] = [
  {
    title: '账号与设备',
    items: [
      { id: 'overview', label: '控制台概览', desc: '账号资源与入口' },
      { id: 'devices', label: '设备', desc: '我能管理或访问的设备' },
      { id: 'userAliases', label: '用户别名', desc: '邮箱显示别名' },
    ],
  },
  {
    title: '网络',
    items: [
      { id: 'workspaces', label: '网络管理', desc: '设备和访问策略' },
    ],
  },
];

export const INITIAL_DEVICES: DeviceRow[] = [
  { deviceId: 'mac-001', platform: 'macOS', osVersion: '15.3', alias: '办公 Mac', ip: '10.0.0.1', owner: 'alice@vlan.com', status: 'active' },
  { deviceId: 'iphone-001', platform: 'iOS', osVersion: '18.2', alias: 'Alice iPhone', ip: '10.0.0.2', owner: 'alice@vlan.com', status: 'active' },
  { deviceId: 'bob-laptop-001', platform: 'Windows', osVersion: '11', alias: 'Bob Laptop', ip: '10.0.0.3', owner: 'bob@vlan.com', status: 'active' },
];

export const INITIAL_WORKSPACE_DEVICE_IDS: Record<string, string[]> = {
  'default-user-000001': ['mac-001', 'iphone-001'],
  'workspace-000001': ['mac-001'],
};

export const INITIAL_WORKSPACES: WorkspaceRow[] = [
  { networkId: 'default-user-000001', workspaceId: 'default-user-000001', name: '默认网络', code: 'default', template: 'default', status: 'enabled', members: 1, devices: 2, zone: 'default.default-user-000001.user-000001.sub.slan.com' },
  { networkId: 'workspace-000001', workspaceId: 'workspace-000001', name: '开发组', code: 'dev', template: 'dev', status: 'enabled', members: 2, devices: 1, zone: 'dev.workspace-000001.user-000001.sub.slan.com' },
];

export const INITIAL_MEMBERS: MemberRow[] = [
  { user: 'alice@vlan.com', alias: 'Alice', role: 'owner', status: 'active' },
  { user: 'bob@vlan.com', alias: 'Bob', role: 'member', status: 'pending' },
];

export const INITIAL_USER_ALIASES: UserAliasRow[] = [
  { email: 'bob@vlan.com', alias: 'Bob' },
  { email: 'ops@vlan.com', alias: 'Ops' },
  { email: 'dev@vlan.com', alias: 'Dev' },
];

export const INITIAL_DNS_ZONES: DNSZoneRow[] = [
  { networkId: 'default-user-000001', workspaceId: 'default-user-000001', zone: 'default.lan', recordType: 'A', value: '10.0.0.1', expose: false, status: 'active' },
  { networkId: 'workspace-000001', workspaceId: 'workspace-000001', zone: 'dev.internal', recordType: 'A', value: '10.0.0.2', expose: true, status: 'active' },
];

export const INITIAL_DNS_RECORDS: DNSRow[] = [
  { networkId: 'default-user-000001', workspaceId: 'default-user-000001', name: 'mac', fqdn: 'mac.default.lan', recordType: 'A', value: 'alice@vlan.com / 办公 Mac / 443', deviceId: 'mac-001', port: '443', expose: false },
  { networkId: 'workspace-000001', workspaceId: 'workspace-000001', name: 'api', fqdn: 'api.dev.internal', recordType: 'A', value: 'alice@vlan.com / Alice iPhone / 8443', deviceId: 'iphone-001', port: '8443', expose: true },
];

export const INITIAL_PUBLIC_MAPPINGS: PublicMappingRow[] = [
  { networkId: 'default-user-000001', workspaceId: 'default-user-000001', alias: 'api', publicDomain: 'api.default.alice.pub.slan.com', sourceRecord: 'api', deviceId: 'iphone-001', protocol: 'HTTP', port: '8443', externalPort: '443', accessMode: 'public', tlsMode: 'auto', status: 'enabled' },
];

export const INITIAL_SECURITY_RULES: SecurityRuleRow[] = [
  { direction: 'ingress', priority: 100, action: 'allow', protocol: 'tcp', port: '22', subjectType: 'device', subjectValue: 'mac-001' },
  { direction: 'egress', priority: 100, action: 'allow', protocol: 'all', port: 'all', subjectType: 'all', subjectValue: 'all' },
];

export const INITIAL_DEVICE_EXPOSURES: DeviceExposureRow[] = [
  { deviceId: 'mac-001', user: 'bob@vlan.com', alias: 'Bob', status: 'active' },
  { deviceId: 'mac-001', user: 'ops@vlan.com', alias: 'Ops', status: 'pending' },
  { deviceId: 'iphone-001', user: 'dev@vlan.com', alias: 'Dev', status: 'active' },
];

export const INITIAL_WORKSPACE_DEVICE_INVITES: WorkspaceDeviceInviteRow[] = [
  {
    inviteId: 'device-invite-000001',
    workspaceId: 'default-user-000001',
    inviterUserId: 'user-000001',
    inviteCode: 'DEFAULT-000001',
    status: 'pending',
    createdAt: 1767225600,
    expiresAt: 1767312000,
  },
  {
    inviteId: 'device-invite-000002',
    workspaceId: 'workspace-000001',
    inviterUserId: 'user-000001',
    inviteCode: 'DEV-000002',
    status: 'accepted',
    createdAt: 1767225600,
    expiresAt: 1767312000,
    acceptedDeviceId: 'mac-001',
    acceptedUserId: 'alice@vlan.com',
    acceptedAt: 1767232800,
  },
];
