import {
  DeviceBootstrapKeyRow,
  DeviceExposureRow,
  DeviceGroupRow,
  DeviceRow,
  DNSRow,
  DNSZoneRow,
  MemberRow,
  NavGroup,
  SecurityGroupRow,
  SecurityRuleRow,
  SecurityRuleTemplate,
  UserAliasRow,
  WorkspaceDeviceInviteRow,
  WorkspacePreset,
  WorkspaceRow,
} from './app.models';

export const ROOT_DOMAIN = 'staticlss.com';
export const DEFAULT_USER_ID = '00000000000040008000000000000001';
export const DEFAULT_NETWORK_ID = '00000000000040008000000000000002';
export const DEV_NETWORK_ID = '00000000000040008000000000000003';
export const DEFAULT_MAC_DEVICE_ID = '00000000000040008000000000000004';
export const DEFAULT_IPHONE_DEVICE_ID = '00000000000040008000000000000005';
export const DEFAULT_BOB_DEVICE_ID = '00000000000040008000000000000006';
export const DEFAULT_ZONE_ID = '00000000000040008000000000000007';
export const DEV_ZONE_ID = '00000000000040008000000000000008';
export const DEFAULT_RECORD_ID = '00000000000040008000000000000009';
export const DEV_RECORD_ID = '0000000000004000800000000000000a';
export const DEFAULT_SECURITY_GROUP_ID = '0000000000004000800000000000000c';
export const DEV_SECURITY_GROUP_ID = '0000000000004000800000000000000d';

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
  { name: '全部出站', direction: 'egress', priority: 100, action: 'allow', protocol: 'all', port: 'all', subjectType: 'user', subjectValue: DEFAULT_USER_ID },
  { name: '拒绝全部入站', description: '兜底拒绝所有来源', direction: 'ingress', priority: 900, action: 'deny', protocol: 'all', port: 'all', subjectType: 'user', subjectValue: DEFAULT_USER_ID },
];

export const NAV_GROUPS: NavGroup[] = [
  {
    title: '账号与设备',
    items: [
      { id: 'overview', label: '控制台概览', desc: '账号资源与入口' },
      {
        id: 'deviceManagement',
        label: '设备管理',
        desc: '设备与分组',
        children: [
          { id: 'devices', label: '设备管理', desc: '设备列表与接入' },
          { id: 'deviceGroups', label: '分组管理', desc: '部门和团队分组' },
        ],
      },
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
  { deviceId: DEFAULT_MAC_DEVICE_ID, platform: 'macOS', osVersion: '15.3', alias: '办公 Mac', ip: '10.0.0.1', owner: 'alice@staticlss.com', status: 'active' },
  { deviceId: DEFAULT_IPHONE_DEVICE_ID, platform: 'iOS', osVersion: '18.2', alias: 'Alice iPhone', ip: '10.0.0.2', owner: 'alice@staticlss.com', status: 'active' },
  { deviceId: DEFAULT_BOB_DEVICE_ID, platform: 'Windows', osVersion: '11', alias: 'Bob Laptop', ip: '10.0.0.3', owner: 'bob@staticlss.com', status: 'active' },
];

export const INITIAL_DEVICE_GROUPS: DeviceGroupRow[] = [
  { groupId: 'device-group-dev', name: '开发部', description: '研发、测试和工程设备', createdAt: 1781579000 },
  { groupId: 'device-group-marketing', name: '市场部', description: '市场活动和内容运营设备', createdAt: 1781579000 },
  { groupId: 'device-group-sales', name: '销售部', description: '销售团队和客户现场设备', createdAt: 1781579000 },
  { groupId: 'device-group-ops', name: '运营部', description: '运营支持和平台值守设备', createdAt: 1781579000 },
];

export const DEVICE_GROUP_PRESETS: Pick<DeviceGroupRow, 'name' | 'description'>[] = [
  { name: '开发部', description: '研发、测试和工程设备' },
  { name: '市场部', description: '市场活动和内容运营设备' },
  { name: '销售部', description: '销售团队和客户现场设备' },
  { name: '运营部', description: '运营支持和平台值守设备' },
];

export const INITIAL_DEVICE_GROUP_IDS: Record<string, string> = {
  [DEFAULT_MAC_DEVICE_ID]: 'device-group-dev',
  [DEFAULT_IPHONE_DEVICE_ID]: 'device-group-marketing',
};

export const INITIAL_WORKSPACE_DEVICE_IDS: Record<string, string[]> = {
  [DEFAULT_NETWORK_ID]: [DEFAULT_MAC_DEVICE_ID, DEFAULT_IPHONE_DEVICE_ID],
  [DEV_NETWORK_ID]: [DEFAULT_MAC_DEVICE_ID],
};

export const INITIAL_WORKSPACES: WorkspaceRow[] = [
  { networkId: DEFAULT_NETWORK_ID, workspaceId: DEFAULT_NETWORK_ID, name: '默认网络', code: 'default', template: 'default', intraGroupPolicy: 'allow', default: true, members: 1, devices: 2, zone: `default.${DEFAULT_NETWORK_ID}.${DEFAULT_USER_ID}.sub.staticlss.com` },
  { networkId: DEV_NETWORK_ID, workspaceId: DEV_NETWORK_ID, name: '开发组', code: 'dev', template: 'dev', intraGroupPolicy: 'allow', default: false, members: 2, devices: 1, zone: `dev.${DEV_NETWORK_ID}.${DEFAULT_USER_ID}.sub.staticlss.com` },
];

export const INITIAL_MEMBERS: MemberRow[] = [
  { user: 'alice@staticlss.com', alias: 'Alice', role: 'owner', status: 'active' },
  { user: 'bob@staticlss.com', alias: 'Bob', role: 'member', status: 'pending' },
];

export const INITIAL_USER_ALIASES: UserAliasRow[] = [
  { email: 'bob@staticlss.com', alias: 'Bob' },
  { email: 'ops@staticlss.com', alias: 'Ops' },
  { email: 'dev@staticlss.com', alias: 'Dev' },
];

export const INITIAL_DNS_ZONES: DNSZoneRow[] = [
  { zoneId: DEFAULT_ZONE_ID, networkId: DEFAULT_NETWORK_ID, workspaceId: DEFAULT_NETWORK_ID, zone: 'default.lan', recordType: 'A', value: '10.0.0.1', expose: false, status: 'active' },
  { zoneId: DEV_ZONE_ID, networkId: DEV_NETWORK_ID, workspaceId: DEV_NETWORK_ID, zone: 'dev.internal', recordType: 'A', value: '10.0.0.2', expose: true, status: 'active' },
];

export const INITIAL_DNS_RECORDS: DNSRow[] = [
  { recordId: DEFAULT_RECORD_ID, zoneId: DEFAULT_ZONE_ID, networkId: DEFAULT_NETWORK_ID, workspaceId: DEFAULT_NETWORK_ID, name: 'mac', fqdn: 'mac.default.lan', recordType: 'A', value: 'alice@staticlss.com / 办公 Mac / 443', deviceId: DEFAULT_MAC_DEVICE_ID, port: '443', ttl: 60, targetType: 'device', expose: false },
  { recordId: DEV_RECORD_ID, zoneId: DEV_ZONE_ID, networkId: DEV_NETWORK_ID, workspaceId: DEV_NETWORK_ID, name: 'api', fqdn: 'api.dev.internal', recordType: 'A', value: 'alice@staticlss.com / Alice iPhone / 8443', deviceId: DEFAULT_IPHONE_DEVICE_ID, port: '8443', ttl: 60, targetType: 'device', expose: true },
];

export const INITIAL_SECURITY_GROUPS: SecurityGroupRow[] = [
  { securityGroupId: DEFAULT_SECURITY_GROUP_ID, networkId: DEFAULT_NETWORK_ID, workspaceId: DEFAULT_NETWORK_ID, name: '', createdAt: 1767225600 },
  { securityGroupId: DEV_SECURITY_GROUP_ID, networkId: DEV_NETWORK_ID, workspaceId: DEV_NETWORK_ID, name: '', createdAt: 1767225600 },
];

export const INITIAL_SECURITY_RULES: SecurityRuleRow[] = [
  { ruleId: '0000000000004000800000000000000e', securityGroupId: DEFAULT_SECURITY_GROUP_ID, direction: 'ingress', priority: 100, action: 'allow', protocol: 'tcp', port: '22', subjectType: 'workspace', subjectValue: 'self', enabled: true },
  { ruleId: '0000000000004000800000000000000f', securityGroupId: DEFAULT_SECURITY_GROUP_ID, direction: 'egress', priority: 100, action: 'allow', protocol: 'all', port: 'all', subjectType: 'user', subjectValue: DEFAULT_USER_ID, enabled: true },
  { ruleId: '00000000000040008000000000000010', securityGroupId: DEV_SECURITY_GROUP_ID, direction: 'ingress', priority: 100, action: 'allow', protocol: 'tcp', port: '22', subjectType: 'workspace', subjectValue: 'self', enabled: true },
  { ruleId: '00000000000040008000000000000011', securityGroupId: DEV_SECURITY_GROUP_ID, direction: 'egress', priority: 100, action: 'allow', protocol: 'all', port: 'all', subjectType: 'user', subjectValue: DEFAULT_USER_ID, enabled: true },
];

export const INITIAL_DEVICE_EXPOSURES: DeviceExposureRow[] = [
  { deviceId: DEFAULT_MAC_DEVICE_ID, user: 'bob@staticlss.com', alias: 'Bob', status: 'active' },
  { deviceId: DEFAULT_MAC_DEVICE_ID, user: 'ops@staticlss.com', alias: 'Ops', status: 'pending' },
  { deviceId: DEFAULT_IPHONE_DEVICE_ID, user: 'dev@staticlss.com', alias: 'Dev', status: 'active' },
];

export const INITIAL_WORKSPACE_DEVICE_INVITES: WorkspaceDeviceInviteRow[] = [
  {
    inviteId: '00000000000040008000000000000012',
    workspaceId: DEFAULT_NETWORK_ID,
    inviterUserId: DEFAULT_USER_ID,
    inviteCode: 'DEFAULT-000001',
    status: 'pending',
    createdAt: 1767225600,
    expiresAt: 1767312000,
  },
  {
    inviteId: '00000000000040008000000000000013',
    workspaceId: DEV_NETWORK_ID,
    inviterUserId: DEFAULT_USER_ID,
    inviteCode: 'DEV-000002',
    status: 'accepted',
    createdAt: 1767225600,
    expiresAt: 1767312000,
    acceptedDeviceId: DEFAULT_MAC_DEVICE_ID,
    acceptedUserId: 'alice@staticlss.com',
    acceptedAt: 1767232800,
  },
];

export const INITIAL_DEVICE_BOOTSTRAP_KEYS: DeviceBootstrapKeyRow[] = [
  {
    id: 'dbkdefault000000000000000000000001',
    key: 'sk_default_preview_0001',
    createdByUserId: DEFAULT_USER_ID,
    networkId: DEFAULT_NETWORK_ID,
    deviceAlias: '办公 Mac',
    expiresAt: 1767312000,
    status: 'active',
    createdAt: 1767225600,
  },
  {
    id: 'dbkdev0000000000000000000000000002',
    key: 'sk_dev_preview_0002',
    createdByUserId: DEFAULT_USER_ID,
    networkId: DEV_NETWORK_ID,
    deviceAlias: 'Android 测试机',
    expiresAt: 1767312000,
    revokedAt: 1767230000,
    status: 'revoked',
    createdAt: 1767226600,
  },
];
