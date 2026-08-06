import { CommonModule } from '@angular/common';
import { ChangeDetectorRef, Component, OnDestroy, OnInit, ViewEncapsulation } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { AuditEventsPageComponent } from './features/audit-events/audit-events-page.component';
import { DevicesPageComponent } from './features/devices/devices-page.component';
import { DeviceGroupsPageComponent } from './features/device-groups/device-groups-page.component';
import { NetworksPageComponent } from './features/networks/networks-page.component';
import { NetworkDetailPageComponent } from './features/networks/network-detail-page.component';
import { OperatorsPageComponent } from './features/operators/operators-page.component';
import { OverviewPageComponent } from './features/overview/overview-page.component';
import { PunchNodesPageComponent } from './features/punch-nodes/punch-nodes-page.component';
import { RelayNodesPageComponent } from './features/relay-nodes/relay-nodes-page.component';
import { OPS_API } from './api-paths';

// 运营后台左侧导航的页面标识，必须和模板中的条件渲染保持一致。
type NavId = 'overview' | 'operators' | 'auditEvents' | 'relayNodes' | 'punchNodes' | 'networks' | 'devices' | 'deviceGroups';
type NavItem = { id: NavId; label: string; desc: string };
type NavMenu = { id: string; label: string; defaultId: NavId; items: NavItem[] };
type QuickRenameKind = 'device' | 'network' | 'deviceGroup' | 'securityGroup';
type ConfirmationDialog = { title: string; message: string; confirmLabel: string };

// 后台操作员账号模型，用于登录后权限展示和账号维护。
type OperatorUser = {
  operatorId: string;
  name: string;
  email: string;
  role: 'admin' | 'operator';
  status: 'active' | 'disabled';
  lastLoginAt: string;
};

type OperatorForm = Partial<OperatorUser> & {
  password?: string;
  confirmPassword?: string;
};

// Relay/DERP 中继节点模型，描述中继容量、协议入口和健康状态。
type RelayNode = {
  nodeId: string;
  name: string;
  transport: 'relay_udp' | 'derp_tcp_tls_443';
  publicAddr: string;
  maxBandwidthMbps: number;
  monthlyTrafficGb: number;
  usedTrafficGb: number;
  maxSessions: number;
  activeSessions: number;
  status: 'active' | 'maintenance' | 'disabled';
  health: 'healthy' | 'warning' | 'down';
};

type RelayNodeForm = Partial<Omit<RelayNode, 'publicAddr'>> & {
  publicIp?: string;
  publicPort?: number;
};

// P2P 打洞节点模型，biz 以公网 UDP IP 和端口直接管理 punch-service。
type PunchNode = {
  nodeId: string;
  name: string;
  publicUdpIp: string;
  publicUdpPort: number;
  maxSessions: number;
  activeSessions: number;
  status: 'active' | 'maintenance' | 'disabled';
  health: 'healthy' | 'warning' | 'down';
  priority?: number;
  createdAt: string;
  updatedAt: string;
};

// 客户资源模型，聚合地域、Relay 用量和状态。
type Customer = {
  customerId: string;
  email: string;
  name: string;
  country: string;
  province: string;
  city: string;
  ipRegion: string;
  relayUsedGb: number;
  status: 'active' | 'limited' | 'expired' | 'disabled';
};

// 运营视角设备模型，展示全局虚拟地址、在线状态和累计流量。
type OpsDevice = {
  deviceId: string;
  name: string;
  alias?: string;
  platform: string;
  osName?: string;
  osVersion?: string;
  deviceVersion?: string;
  globalIp: string;
  globalName: string;
  status: 'active' | 'disabled';
  heartbeatOnline: boolean;
  networkEnabled: boolean;
  deviceEnabled: boolean;
  rxBytesTotal: number;
  txBytesTotal: number;
  networkCount: number;
  lastSeenAt?: string;
  lastReportAt?: string;
  createdAt: string;
  updatedAt: string;
};

type DeviceCredential = {
  credentialId: string;
  keyId: string;
  deviceId: string;
  name: string;
  scopes: string;
  status: 'active' | 'revoked';
  lastUsedAt: string;
  lastUsedIp: string;
  createdAt: string;
  updatedAt: string;
};

type AuditEvent = {
  eventId: string;
  actorType: string;
  actorId: string;
  action: string;
  resourceType: string;
  resourceId: string;
  status: string;
  remoteIp?: string;
  detail?: string;
  createdAt: string | number;
};

type OpsNetwork = { networkId: string; name: string; intraGroupPolicy: string; status: string; deviceIds: string[]; deviceGroupIds: string[]; createdAt: number; updatedAt: number };
type OpsDeviceGroup = { groupId: string; name: string; description: string; createdAt: number; updatedAt: number };
type OpsDeviceGroupMember = { groupId: string; deviceId: string; addedAt: number };
type DNSZone = { zoneId: string; networkId: string; name: string; status: string; createdAt: number; updatedAt: number };
type DNSRecord = { recordId: string; networkId: string; zoneId: string; name: string; type: string; value: string; port: string; ttl: number; status: string };
type SecurityGroup = { securityGroupId: string; networkId: string; name: string; description: string };
type SecurityRule = { ruleId: string; securityGroupId: string; direction: string; protocol: string; portRange: string; peerType: string; peerValue: string; action: string; priority: number; description: string; enabled: boolean };
type NetworkPolicyDetail = {
  networkId: string;
  dns: { zones: DNSZone[]; records: DNSRecord[] };
  security: { groups: SecurityGroup[]; rules: SecurityRule[] };
  summary: { dnsZoneCount: number; dnsRecordCount: number; securityGroupCount: number; securityRuleCount: number };
};


@Component({
  selector: 'ops-root',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    AuditEventsPageComponent,
    OverviewPageComponent,
    OperatorsPageComponent,
    RelayNodesPageComponent,
    PunchNodesPageComponent,
    DevicesPageComponent,
    NetworksPageComponent,
    NetworkDetailPageComponent,
    DeviceGroupsPageComponent,
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
  encapsulation: ViewEncapsulation.None,
})
// 运营后台根组件，集中持有页面状态、表单状态和对后端 ops API 的访问逻辑。
export class AppComponent implements OnInit, OnDestroy {
  constructor(private readonly changeDetector: ChangeDetectorRef) {}

  // 一级页面、一级资源菜单与分组菜单分开定义。
  readonly navItems: NavItem[] = [
    { id: 'overview', label: '运营概览', desc: '平台指标与待处理事项' },
    { id: 'auditEvents', label: '安全审计', desc: '登录、凭据与异常来源' },
    { id: 'operators', label: '运营账号', desc: '后台账号与角色' },
  ];
  readonly resourceNavItems: NavItem[] = [
    { id: 'devices', label: '设备管理', desc: '全局设备、在线与启用状态' },
    { id: 'deviceGroups', label: '设备组管理', desc: '全局设备分组与成员' },
    { id: 'networks', label: '网络管理', desc: '网络、成员与策略' },
  ];
  readonly navMenus: NavMenu[] = [
    {
      id: 'nodes',
      label: '节点管理',
      defaultId: 'punchNodes',
      items: [
        { id: 'punchNodes', label: '打洞节点', desc: 'P2P Punch 节点管理' },
        { id: 'relayNodes', label: '中继节点', desc: 'Relay/DERP 容量管理' },
      ],
    },
  ];

  active: NavId = 'overview';
  readonly opsTokenKey = 'slan_ops_token';
  readonly opsEmailKey = 'slan_ops_email';
	readonly opsRoleKey = 'slan_ops_role';
  operatorEmail = localStorage.getItem(this.opsEmailKey) || 'admin1';
	operatorRole: OperatorUser['role'] = localStorage.getItem(this.opsRoleKey) === 'admin' ? 'admin' : 'operator';
  loginEmail = this.operatorEmail;
  loginPassword = '';
  loginMessage = '';
  loading = false;
  apiMessage = '';
  showCurrentPasswordDialog = false;
  showOperatorPasswordDialog = false;
  showOperatorDialog = false;
  showRelayNodeDialog = false;
  showPunchNodeDialog = false;
  relayNodeMessage = '';
  punchNodeMessage = '';
  showCustomerDialog = false;
  showDeviceDialog = false;
  showDeviceGroupAssignmentDialog = false;
  showCredentialDialog = false;
  credentialCreating = false;
  showNetworkDialog = false;
  showNetworkBindingDialog = false;
  showDeviceGroupDialog = false;
  showDeviceGroupBindingDialog = false;
  showQuickRenameDialog = false;
  selectedCustomer: Customer | null = null;
  selectedOperator: OperatorUser | null = null;
  selectedRelayNode: RelayNode | null = null;
  selectedPunchNode: PunchNode | null = null;
  selectedDevice: OpsDevice | null = null;
  deviceGroupAssignmentDevice: OpsDevice | null = null;
  oldPassword = '';
  newPassword = '';
  confirmPassword = '';
  operatorNewPassword = '';
  operatorConfirmPassword = '';
  passwordMessage = '';
  operatorForm: OperatorForm = {};
  relayNodeForm: RelayNodeForm = {};
  punchNodeForm: Partial<PunchNode> = {};
  customerForm: Partial<Customer> = {};
  deviceForm: Partial<OpsDevice> = {};
  deviceKeyword = '';
  auditStatusFilter = '';
  auditActionFilter = '';
  auditResourceTypeFilter = '';
  auditKeyword = '';
  createdCredentialKey = '';
  selectedNetwork: OpsNetwork | null = null;
  networkDetailId: string | null = null;
  dnsZoneDetailId: string | null = null;
  securityGroupDetailId: string | null = null;
  networkForm = { name: '', intraGroupPolicy: 'allow', status: 'active' };
  networkBindingIds: string[] = [];
  networkBindingKeyword = '';
  networkBindingSaving = false;
  networkBindingError = '';
  networkPolicyTab: 'dns' | 'security' = 'dns';
  dnsZones: DNSZone[] = [];
  dnsRecords: DNSRecord[] = [];
  securityGroups: SecurityGroup[] = [];
  securityRules: SecurityRule[] = [];
  selectedDNSZone: DNSZone | null = null;
  selectedDNSRecord: DNSRecord | null = null;
  selectedSecurityRule: SecurityRule | null = null;
  dnsZoneForm = { name: '', status: 'active' };
  dnsRecordForm = { zoneId: '', name: '', type: 'A', value: '', port: '', ttl: 300 };
  securityGroupForm = { name: '', description: '' };
  securityRuleForm = { securityGroupId: '', direction: 'ingress', protocol: 'any', portRange: '', peerType: 'device_group', peerValue: '', action: 'allow', priority: 100, description: '', enabled: true };
  selectedDeviceGroup: OpsDeviceGroup | null = null;
  deviceGroupForm = { name: '', description: '' };
  deviceGroupBindingDeviceIds: string[] = [];
  deviceGroupBindingKeyword = '';
  deviceGroupBindingSaving = false;
  deviceGroupBindingError = '';
  deviceGroupAssignmentIds: string[] = [];
  deviceGroupAssignmentKeyword = '';
  deviceGroupAssignmentSaving = false;
  deviceGroupAssignmentError = '';
  quickRenameKind: QuickRenameKind | null = null;
  quickRenameTarget: OpsDevice | OpsNetwork | OpsDeviceGroup | SecurityGroup | null = null;
  quickRenameValue = '';
  quickRenameError = '';
  quickRenameSaving = false;
  confirmationDialog: ConfirmationDialog | null = null;
  private confirmationResolver: ((confirmed: boolean) => void) | null = null;

  // 传给 feature 子页面的视图模型，保持子页面只负责模板渲染。
  get vm(): this {
    return this;
  }

  get isLoggedIn(): boolean {
    return Boolean(localStorage.getItem(this.opsTokenKey));
  }

  get apiMessageIsSuccess(): boolean {
    return /(已保存|已启用|已停用|已删除|已创建|已更新|已指派)/.test(this.apiMessage);
  }

  get quickRenameResourceLabel(): string {
    switch (this.quickRenameKind) {
      case 'device': return '设备';
      case 'network': return '网络';
      case 'deviceGroup': return '设备组';
      case 'securityGroup': return '安全组';
      default: return '资源';
    }
  }

  private requestConfirmation(title: string, message: string, confirmLabel: string): Promise<boolean> {
    this.confirmationResolver?.(false);
    return new Promise((resolve) => {
      this.confirmationResolver = resolve;
      this.confirmationDialog = { title, message, confirmLabel };
      this.notifyStateChanged();
    });
  }

  resolveConfirmation(confirmed: boolean): void {
    const resolve = this.confirmationResolver;
    this.confirmationResolver = null;
    this.confirmationDialog = null;
    resolve?.(confirmed);
    this.notifyStateChanged();
  }

  ngOnInit(): void {
    window.addEventListener('popstate', this.handlePopState);
    if (this.isLoggedIn) {
      void this.loadOpsData();
    }
  }

  ngOnDestroy(): void {
    window.removeEventListener('popstate', this.handlePopState);
  }

  private readonly handlePopState = (): void => {
    void this.syncNetworkRouteFromLocation();
  };

  operators: OperatorUser[] = [];
  relayNodes: RelayNode[] = [];
  punchNodes: PunchNode[] = [];
  customers: Customer[] = [];
  devices: OpsDevice[] = [];
  deviceCredentials: DeviceCredential[] = [];
  auditEvents: AuditEvent[] = [];
  networks: OpsNetwork[] = [];
  deviceGroups: OpsDeviceGroup[] = [];
  deviceGroupMembers: OpsDeviceGroupMember[] = [];

  async login(): Promise<void> {
    this.loginMessage = '';
    if (!this.loginEmail.trim() || !this.loginPassword.trim()) {
      this.loginMessage = '请输入邮箱和密码';
      return;
    }
    try {
      const response = await this.request<{ auth: { operator: OperatorUser; session: { token: string } } }>('POST', OPS_API.authLogin, {
        email: this.loginEmail.trim(),
        password: this.loginPassword,
      }, false);
      localStorage.setItem(this.opsTokenKey, response.auth.session.token);
      localStorage.setItem(this.opsEmailKey, response.auth.operator.email);
	  localStorage.setItem(this.opsRoleKey, response.auth.operator.role);
      this.operatorEmail = response.auth.operator.email;
	  this.operatorRole = response.auth.operator.role;
      this.loginEmail = response.auth.operator.email;
      this.loginPassword = '';
      await this.loadOpsData();
      this.notifyStateChanged();
    } catch (error) {
      this.loginMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  logout(message = ''): void {
	const token = localStorage.getItem(this.opsTokenKey);
	if (token) {
		void fetch(OPS_API.authLogout, {
			method: 'POST',
			headers: { Authorization: `Bearer ${token}` },
		}).catch(() => undefined);
	}
    localStorage.removeItem(this.opsTokenKey);
    localStorage.removeItem(this.opsEmailKey);
	localStorage.removeItem(this.opsRoleKey);
    this.operatorEmail = 'admin1';
	this.operatorRole = 'operator';
    this.loginEmail = this.operatorEmail;
    this.loginPassword = '';
    this.loginMessage = message;
    this.apiMessage = '';
    this.notifyStateChanged();
  }

  // 登录后批量拉取所有运营页面数据，确保概览和各管理页使用同一份状态。
  async loadOpsData(): Promise<void> {
    this.loading = true;
    this.apiMessage = '';
    try {
      const [operators, auditEvents, relayNodes, punchNodes, customers, devices, credentials, networks, groups] = await Promise.all([
		this.operatorRole === 'admin'
		  ? this.request<{ items: OperatorUser[] }>('GET', OPS_API.operators)
		  : Promise.resolve({ items: [] as OperatorUser[] }),
        this.request<{ items: AuditEvent[] }>('GET', `${OPS_API.auditEvents}?limit=200`),
        this.request<{ items: RelayNode[] }>('GET', OPS_API.relayNodes),
        this.request<{ items: PunchNode[] }>('GET', OPS_API.punchNodes),
        this.request<{ items: Customer[] }>('GET', OPS_API.customers),
        this.request<{ items: OpsDevice[] }>('GET', OPS_API.devices),
        this.request<{ items: DeviceCredential[] }>('GET', OPS_API.deviceCredentials),
        this.request<{ items: OpsNetwork[] }>('GET', OPS_API.networks),
        this.request<{ items: OpsDeviceGroup[]; members: OpsDeviceGroupMember[] }>('GET', OPS_API.deviceGroups),
      ]);
      this.operators = operators.items.map((item) => ({ ...item, lastLoginAt: this.formatDateTime(item.lastLoginAt) }));
      this.auditEvents = auditEvents.items.map((item) => ({ ...item, createdAt: this.formatDateTime(item.createdAt) }));
      this.relayNodes = relayNodes.items;
      this.punchNodes = punchNodes.items.map((item) => ({
        ...item,
        createdAt: this.formatDateTime(item.createdAt),
        updatedAt: this.formatDateTime(item.updatedAt),
      }));
      this.customers = customers.items;
      this.devices = devices.items.map((item) => this.formatDevice(item));
      this.deviceCredentials = credentials.items.map((item) => this.formatDeviceCredential(item));
      this.networks = networks.items;
      this.deviceGroups = groups.items;
      this.deviceGroupMembers = groups.members;
      await this.syncNetworkRouteFromLocation();
    } catch (error) {
      this.operators = [];
      this.auditEvents = [];
      this.relayNodes = [];
      this.punchNodes = [];
      this.customers = [];
      this.devices = [];
      this.deviceCredentials = [];
      this.networks = [];
      this.deviceGroups = [];
      this.deviceGroupMembers = [];
      this.apiMessage = this.errorMessage(error);
    } finally {
      this.loading = false;
      this.notifyStateChanged();
    }
  }

  private notifyStateChanged(): void {
    this.changeDetector.detectChanges();
  }

  // 统一封装 ops HTTP 请求，附加 Bearer token 并集中处理 401 过期登录。
  private async request<T>(method: string, path: string, body?: unknown, requireAuth = true): Promise<T> {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    const token = localStorage.getItem(this.opsTokenKey);
    if (requireAuth && token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    const response = await fetch(path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!response.ok) {
      const text = await response.text();
      if (response.status === 401 && requireAuth) {
        this.logout('登录已过期，请重新登录');
        throw new Error('登录已过期，请重新登录');
      }
      let message = text || `HTTP ${response.status}`;
      try {
        const payload = JSON.parse(text) as { error?: string };
        if (payload?.error?.trim()) {
          message = payload.error.trim();
        }
      } catch {
      }
      throw new Error(message);
    }
    if (response.status === 204) {
      return undefined as T;
    }
    return response.json() as Promise<T>;
  }

  private formatDate(value: string | number | undefined): string {
    if (!value) {
      return '';
    }
    if (typeof value === 'string' && value.includes('-')) {
      return value.slice(0, 10);
    }
    return new Date(Number(value) * 1000).toISOString().slice(0, 10);
  }

  formatDateTime(value: string | number | undefined): string {
    if (!value) {
      return '-';
    }
    if (typeof value === 'string' && value.includes('-')) {
      return value;
    }
    return new Date(Number(value) * 1000).toISOString().replace('T', ' ').slice(0, 16);
  }

  private dateToUnix(value: string): number {
    return Math.floor(new Date(`${value}T00:00:00`).getTime() / 1000);
  }

  private errorMessage(error: unknown): string {
    const message = error instanceof Error ? error.message : String(error);
    const normalized = message.trim().toLowerCase();
    if (normalized === 'conflict') {
      return '数据已存在或唯一字段重复，请检查后再保存';
    }
    if (normalized === 'bad request') {
      return '请求参数不完整或格式不正确';
    }
    if (normalized === 'not found') {
      return '数据不存在或已被删除，请刷新后重试';
    }
    if (normalized === 'unauthorized') {
      this.logout('登录已过期，请重新登录');
      return '登录已过期，请重新登录';
    }
    return message;
  }

  get activeNav() {
	return this.visibleNavItems.find((item) => item.id === this.active) ?? this.visibleNavItems[0];
  }

	get visibleNavItems(): NavItem[] {
	  const standalone = this.operatorRole === 'admin'
      ? this.navItems
      : this.navItems.filter((item) => item.id !== 'operators');
    return [...standalone, ...this.resourceNavItems, ...this.navMenus.flatMap((menu) => menu.items)];
	}

  get activeNavMenu(): NavMenu | undefined {
    return this.navMenus.find((menu) => menu.items.some((item) => item.id === this.active));
  }

  isNavMenuActive(menu: NavMenu): boolean {
    return menu.items.some((item) => item.id === this.active);
  }

  selectNavMenu(menu: NavMenu): void {
    this.setActive(menu.defaultId);
  }

  get filteredDevices(): OpsDevice[] {
    const keyword = this.deviceKeyword.trim().toLowerCase();
    if (!keyword) {
      return this.devices;
    }
    return this.devices.filter((device) => [
      device.deviceId,
      device.name,
      device.platform,
      device.osName,
      device.globalIp,
      device.globalName,
    ].some((value) => String(value ?? '').toLowerCase().includes(keyword)));
  }

  get auditActions(): string[] {
    return [...new Set(this.auditEvents.map((item) => item.action).filter(Boolean))].sort();
  }

  get auditResourceTypes(): string[] {
    return [...new Set(this.auditEvents.map((item) => item.resourceType).filter(Boolean))].sort();
  }

  get filteredAuditEvents(): AuditEvent[] {
    const keyword = this.auditKeyword.trim().toLowerCase();
    return this.auditEvents.filter((item) => {
      if (this.auditStatusFilter && item.status !== this.auditStatusFilter) return false;
      if (this.auditActionFilter && item.action !== this.auditActionFilter) return false;
      if (this.auditResourceTypeFilter && item.resourceType !== this.auditResourceTypeFilter) return false;
      if (!keyword) return true;
      return [item.actorId, item.resourceId, item.remoteIp, item.detail, item.eventId]
        .some((value) => String(value ?? '').toLowerCase().includes(keyword));
    });
  }

  get warningAuditEventCount(): number {
    return this.auditEvents.filter((item) => item.status === 'warning').length;
  }

  get limitedCustomers(): number {
    return this.customers.filter((customer) => customer.status === 'limited').length;
  }

  get activeRelayNodes(): number {
    return this.relayNodes.filter((node) => node.status === 'active').length;
  }

  get udpRelayNodes(): RelayNode[] {
    return this.relayNodes.filter((node) => node.transport === 'relay_udp');
  }

  get tcpRelayNodes(): RelayNode[] {
    return this.relayNodes.filter((node) => node.transport === 'derp_tcp_tls_443');
  }

  get activeUdpRelayNodes(): number {
    return this.udpRelayNodes.filter((node) => node.status === 'active').length;
  }

  get activeTcpRelayNodes(): number {
    return this.tcpRelayNodes.filter((node) => node.status === 'active').length;
  }

  get activePunchNodes(): number {
    return this.punchNodes.filter((node) => node.status === 'active').length;
  }

  get healthyPunchNodes(): number {
    return this.punchNodes.filter((node) => node.health === 'healthy').length;
  }

  get totalPunchSessions(): number {
    return this.punchNodes.reduce((sum, node) => sum + node.activeSessions, 0);
  }

  get addressStats(): Array<{ label: string; count: number; percent: number }> {
    const total = Math.max(1, this.customers.length);
    const counts = new Map<string, number>();
    for (const customer of this.customers) {
      const label = `${customer.country} / ${customer.city}`;
      counts.set(label, (counts.get(label) ?? 0) + 1);
    }
    return [...counts.entries()]
      .map(([label, count]) => ({ label, count, percent: Math.round((count / total) * 100) }))
      .sort((a, b) => b.count - a.count);
  }

  get regionStats(): Array<{ label: string; count: number; percent: number }> {
    const total = Math.max(1, this.customers.length);
    const counts = new Map<string, number>();
    for (const customer of this.customers) {
      counts.set(customer.ipRegion, (counts.get(customer.ipRegion) ?? 0) + 1);
    }
    return [...counts.entries()]
      .map(([label, count]) => ({ label, count, percent: Math.round((count / total) * 100) }))
      .sort((a, b) => b.count - a.count);
  }

  setActive(id: NavId): void {
	if (id === 'operators' && this.operatorRole !== 'admin') {
	  this.active = 'overview';
	  return;
    }
    if (this.networkDetailId) {
      this.clearNetworkDetail();
      window.history.pushState({}, '', '/');
    }
    this.active = id;
  }

  relayNodePercent(node: RelayNode): number {
    return Math.min(100, Math.round((node.usedTrafficGb / node.monthlyTrafficGb) * 100));
  }

  relayTransportLabel(transport: RelayNode['transport']): string {
    return transport === 'derp_tcp_tls_443' ? 'TCP 中继' : 'UDP 中继';
  }

  punchNodePercent(node: PunchNode): number {
    return Math.min(100, Math.round((node.activeSessions / Math.max(1, node.maxSessions)) * 100));
  }

  formatBytes(value: number | undefined): string {
    const bytes = Number(value ?? 0);
    if (bytes >= 1024 * 1024 * 1024) {
      return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
    }
    if (bytes >= 1024 * 1024) {
      return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
    }
    if (bytes >= 1024) {
      return `${(bytes / 1024).toFixed(1)} KB`;
    }
    return `${bytes} B`;
  }

  openOperatorDialog(operator?: OperatorUser): void {
    this.selectedOperator = operator ?? null;
    this.operatorForm = operator
      ? { ...operator, password: '', confirmPassword: '' }
      : { name: '', email: '', role: 'operator', status: 'active', password: '', confirmPassword: '' };
    this.showOperatorDialog = true;
  }

  closeOperatorDialog(): void {
    this.showOperatorDialog = false;
    this.selectedOperator = null;
  }

  async saveOperatorDialog(): Promise<void> {
    if (!this.operatorForm.name?.trim() || !this.operatorForm.email?.trim()) {
      this.apiMessage = '请输入运营账号姓名和邮箱';
      return;
    }
    const isEdit = Boolean(this.selectedOperator);
    if (this.selectedOperator?.status === 'active' && this.operatorForm.status === 'disabled' &&
        !await this.requestConfirmation('停用运营账号', `停用后，${this.selectedOperator.email} 将无法登录运营后台。`, '确认停用')) {
      return;
    }
    if (!isEdit) {
      const password = this.operatorForm.password?.trim() ?? '';
      const confirmPassword = this.operatorForm.confirmPassword?.trim() ?? '';
      if (!password) {
        this.apiMessage = '请输入初始密码';
        this.notifyStateChanged();
        return;
      }
      if (password !== confirmPassword) {
        this.apiMessage = '两次输入的密码不一致';
        this.notifyStateChanged();
        return;
      }
	  const passwordMessage = this.validatePassword(password, confirmPassword, false);
	  if (passwordMessage) {
		this.apiMessage = passwordMessage;
		this.notifyStateChanged();
		return;
	  }
    }
    try {
      const path = isEdit ? OPS_API.operator(this.selectedOperator!.operatorId) : OPS_API.operators;
      const payload = isEdit
        ? {
            name: this.operatorForm.name,
            role: this.operatorForm.role,
            status: this.operatorForm.status,
          }
        : {
            name: this.operatorForm.name,
            email: this.operatorForm.email,
            role: this.operatorForm.role,
            password: this.operatorForm.password,
          };
      const operator = await this.request<OperatorUser>(isEdit ? 'PATCH' : 'POST', path, payload);
      const formatted = { ...operator, lastLoginAt: this.formatDateTime(operator.lastLoginAt) };
      this.operators = [formatted, ...this.operators.filter((item) => item.operatorId !== operator.operatorId)];
      this.closeOperatorDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openRelayNodeDialog(node?: RelayNode): void {
    this.selectedRelayNode = node ?? null;
    this.relayNodeMessage = '';
    this.relayNodeForm = node ? {
      ...node,
      ...this.parseRelayPublicAddress(node.publicAddr, node.transport),
    } : {
      name: '',
      transport: 'relay_udp',
      publicIp: '',
      publicPort: 29110,
      maxBandwidthMbps: 1000,
      monthlyTrafficGb: 10240,
      usedTrafficGb: 0,
      maxSessions: 5000,
      activeSessions: 0,
      status: 'active',
      health: 'healthy',
    };
    this.showRelayNodeDialog = true;
  }

  closeRelayNodeDialog(): void {
    this.showRelayNodeDialog = false;
    this.selectedRelayNode = null;
    this.relayNodeMessage = '';
  }

  async saveRelayNodeDialog(): Promise<void> {
    this.relayNodeMessage = '';
    const transport = this.relayNodeForm.transport ?? 'relay_udp';
    const publicIp = this.relayNodeForm.publicIp?.trim() ?? '';
    const publicPort = Number(this.relayNodeForm.publicPort ?? 0);
    if (!this.relayNodeForm.name?.trim() || !publicIp) {
      this.relayNodeMessage = '请输入节点名称和公网 IP';
      return;
    }
    if (!this.isIPv4Address(publicIp)) {
      this.relayNodeMessage = '公网 IP 必须使用 IPv4 地址，不能使用域名';
      return;
    }
    if (publicPort <= 0 || publicPort > 65535) {
      this.relayNodeMessage = '公网端口必须在 1-65535 范围内';
      return;
    }
    const publicAddr = this.relayPublicAddress(transport, publicIp, publicPort);
    const duplicated = this.relayNodes.some((node) => node.publicAddr === publicAddr && node.nodeId !== this.selectedRelayNode?.nodeId);
    if (duplicated) {
      this.relayNodeMessage = '公网地址已存在，不能重复配置到多个中继节点';
      return;
    }
    if (this.selectedRelayNode?.status === 'active' && this.relayNodeForm.status !== 'active' &&
        !await this.requestConfirmation('变更中继节点状态', `节点 ${this.selectedRelayNode.name} 将停止承载新的中继连接。`, '确认变更')) {
      return;
    }
    try {
      const isEdit = Boolean(this.selectedRelayNode);
      const path = isEdit ? OPS_API.relayNode(this.selectedRelayNode!.nodeId) : OPS_API.relayNodes;
      const node = await this.request<RelayNode>(isEdit ? 'PATCH' : 'POST', path, {
        nodeId: this.selectedRelayNode?.nodeId,
        name: this.relayNodeForm.name,
        transport,
        publicAddr,
        maxBandwidthMbps: this.relayNodeForm.maxBandwidthMbps,
        monthlyTrafficGb: this.relayNodeForm.monthlyTrafficGb,
        maxSessions: this.relayNodeForm.maxSessions,
        status: this.relayNodeForm.status,
      });
      this.relayNodes = [node, ...this.relayNodes.filter((item) => item.nodeId !== node.nodeId)];
      this.apiMessage = `${this.relayTransportLabel(node.transport)} ${node.name} 已保存`;
      this.closeRelayNodeDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.relayNodeMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  private relayPublicAddress(transport: RelayNode['transport'], publicIp: string, publicPort: number): string {
    const scheme = transport === 'derp_tcp_tls_443' ? 'derp' : 'udp';
    return `${scheme}://${publicIp}:${publicPort}`;
  }

  private parseRelayPublicAddress(publicAddr: string, transport: RelayNode['transport']): Pick<RelayNodeForm, 'publicIp' | 'publicPort'> {
    const fallbackPort = transport === 'derp_tcp_tls_443' ? 29120 : 29110;
    const value = publicAddr.trim();
    const match = value.match(/^(?:[a-zA-Z][a-zA-Z0-9+.-]*:\/\/)?([^:/]+):(\d+)$/);
    if (!match) {
      return { publicIp: value, publicPort: fallbackPort };
    }
    return { publicIp: match[1], publicPort: Number(match[2]) || fallbackPort };
  }

  private isIPv4Address(value: string): boolean {
    const parts = value.split('.');
    return parts.length === 4 && parts.every((part) => {
      if (!/^\d+$/.test(part)) {
        return false;
      }
      const number = Number(part);
      return number >= 0 && number <= 255 && String(number) === part;
    });
  }

  openPunchNodeDialog(node?: PunchNode): void {
    this.selectedPunchNode = node ?? null;
    this.punchNodeMessage = '';
    this.punchNodeForm = node ? { ...node } : {
      name: '',
      publicUdpIp: '',
      publicUdpPort: 29130,
      maxSessions: 10000,
      activeSessions: 0,
      status: 'active',
      health: 'healthy',
      priority: this.punchNodes.length + 1,
    };
    this.showPunchNodeDialog = true;
  }

  closePunchNodeDialog(): void {
    this.showPunchNodeDialog = false;
    this.selectedPunchNode = null;
    this.punchNodeMessage = '';
  }

  async savePunchNodeDialog(): Promise<void> {
    this.punchNodeMessage = '';
    if (!this.punchNodeForm.name?.trim() || !this.punchNodeForm.publicUdpIp?.trim()) {
      this.punchNodeMessage = '请输入打洞节点名称和公网 UDP IP';
      return;
    }
    const publicUdpIp = this.punchNodeForm.publicUdpIp.trim();
    if (!this.isIPv4Address(publicUdpIp)) {
      this.punchNodeMessage = '公网 UDP IP 必须使用 IPv4 地址，不能使用域名';
      return;
    }
    const publicUdpPort = Number(this.punchNodeForm.publicUdpPort ?? 0);
    if (publicUdpPort <= 0 || publicUdpPort > 65534) {
      this.punchNodeMessage = '公网 UDP 端口必须在 1-65534 范围内';
      return;
    }
    const duplicated = this.punchNodes.some((node) =>
      node.publicUdpIp === publicUdpIp &&
      node.publicUdpPort === publicUdpPort &&
      node.nodeId !== this.selectedPunchNode?.nodeId
    );
    if (duplicated) {
      this.punchNodeMessage = '公网 UDP IP 和端口已存在，不能重复配置到多个打洞节点';
      return;
    }
    if (this.selectedPunchNode?.status === 'active' && this.punchNodeForm.status !== 'active' &&
        !await this.requestConfirmation('变更打洞节点状态', `节点 ${this.selectedPunchNode.name} 将停止承载新的打洞会话。`, '确认变更')) {
      return;
    }
    try {
      const isEdit = Boolean(this.selectedPunchNode);
      const path = isEdit ? OPS_API.punchNode(this.selectedPunchNode!.nodeId) : OPS_API.punchNodes;
      const node = await this.request<PunchNode>(isEdit ? 'PATCH' : 'POST', path, {
        nodeId: this.selectedPunchNode?.nodeId,
        name: this.punchNodeForm.name,
        publicUdpIp,
        publicUdpPort,
        maxSessions: Number(this.punchNodeForm.maxSessions ?? 0),
        status: this.punchNodeForm.status,
        health: this.punchNodeForm.health,
        priority: Number(this.punchNodeForm.priority ?? 0),
      });
      const formatted = {
        ...node,
        createdAt: this.formatDateTime(node.createdAt),
        updatedAt: this.formatDateTime(node.updatedAt),
      };
      this.punchNodes = [formatted, ...this.punchNodes.filter((item) => item.nodeId !== node.nodeId)]
        .sort((a, b) => Number(a.priority ?? 0) - Number(b.priority ?? 0));
      this.apiMessage = `UDP 打洞 ${formatted.name} 已保存`;
      this.closePunchNodeDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.punchNodeMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openCustomerDialog(customer?: Customer): void {
    this.selectedCustomer = customer ?? null;
    this.customerForm = customer ? { ...customer } : { email: '', name: '', status: 'active' };
    this.showCustomerDialog = true;
  }

  closeCustomerDialog(): void {
    this.showCustomerDialog = false;
    this.selectedCustomer = null;
  }

  async saveCustomerDialog(): Promise<void> {
    if (!this.customerForm.email?.trim()) {
      this.apiMessage = '请输入客户邮箱';
      return;
    }
    if (this.selectedCustomer?.status === 'active' && this.customerForm.status !== 'active' &&
        !await this.requestConfirmation('限制客户账号', `客户 ${this.selectedCustomer.email} 的服务状态将被限制或停用。`, '确认变更')) {
      return;
    }
    try {
      const editing = Boolean(this.selectedCustomer);
      const path = editing ? OPS_API.customer(this.selectedCustomer!.customerId) : OPS_API.customers;
      const customer = await this.request<Customer>(editing ? 'PATCH' : 'POST', path, this.customerForm);
      this.customers = [customer, ...this.customers.filter((item) => item.customerId !== customer.customerId)];
      this.closeCustomerDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openDeviceDialog(device?: OpsDevice): void {
    this.selectedDevice = device ?? null;
    this.deviceForm = device ? { ...device } : { name: '', platform: '', osName: '', osVersion: '', status: 'active' };
    this.showDeviceDialog = true;
  }

  closeDeviceDialog(): void {
    this.showDeviceDialog = false;
    this.selectedDevice = null;
  }

  async saveDeviceDialog(): Promise<void> {
    if (this.selectedDevice?.status === 'active' &&
        (this.deviceForm.status === 'disabled' || this.deviceForm.deviceEnabled === false) &&
        !await this.requestConfirmation('停用设备', `停用后，设备 ${this.selectedDevice.name || this.selectedDevice.deviceId} 将无法接入网络。`, '确认停用')) {
      return;
    }
    try {
      const editing = Boolean(this.selectedDevice);
      const updated = await this.request<OpsDevice>(editing ? 'PATCH' : 'POST', editing ? OPS_API.device(this.selectedDevice!.deviceId) : OPS_API.devices, editing ? {
        name: this.deviceForm.name, virtualIp: this.deviceForm.globalIp, status: this.deviceForm.status, enabled: this.deviceForm.deviceEnabled,
      } : {
        name: this.deviceForm.name, platform: this.deviceForm.platform, osName: this.deviceForm.osName, osVersion: this.deviceForm.osVersion,
      });
      const formatted = this.formatDevice(updated);
      this.devices = [formatted, ...this.devices.filter((item) => item.deviceId !== updated.deviceId)];
      this.closeDeviceDialog();
      this.apiMessage = editing ? '设备已更新' : '设备已创建';
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openQuickRename(kind: QuickRenameKind, target: OpsDevice | OpsNetwork | OpsDeviceGroup | SecurityGroup): void {
    this.quickRenameKind = kind;
    this.quickRenameTarget = target;
    this.quickRenameValue = target.name;
    this.quickRenameError = '';
    this.showQuickRenameDialog = true;
  }

  closeQuickRename(): void {
    if (this.quickRenameSaving) return;
    this.showQuickRenameDialog = false;
    this.quickRenameKind = null;
    this.quickRenameTarget = null;
    this.quickRenameError = '';
  }

  async saveQuickRename(): Promise<void> {
    const kind = this.quickRenameKind;
    const target = this.quickRenameTarget;
    const name = this.quickRenameValue.trim();
    if (!kind || !target || !name || this.quickRenameSaving) {
      this.quickRenameError = name ? '' : '请输入名称';
      return;
    }
    this.quickRenameSaving = true;
    this.quickRenameError = '';
    try {
      if (kind === 'device') {
        const device = target as OpsDevice;
        const updated = this.formatDevice(await this.request<OpsDevice>('PATCH', OPS_API.device(device.deviceId), { name }));
        this.devices = this.devices.map((item) => item.deviceId === updated.deviceId ? updated : item);
      } else if (kind === 'network') {
        const network = target as OpsNetwork;
        const updated = await this.request<OpsNetwork>('PATCH', OPS_API.network(network.networkId), { name });
        this.networks = this.networks.map((item) => item.networkId === updated.networkId ? updated : item);
        if (this.selectedNetwork?.networkId === updated.networkId) this.selectedNetwork = updated;
      } else if (kind === 'deviceGroup') {
        const group = target as OpsDeviceGroup;
        const updated = await this.request<OpsDeviceGroup>('PATCH', OPS_API.deviceGroup(group.groupId), { name, description: group.description });
        this.deviceGroups = this.deviceGroups.map((item) => item.groupId === updated.groupId ? updated : item);
      } else {
        const group = target as SecurityGroup;
        const updated = await this.request<SecurityGroup>('PATCH', OPS_API.securityGroup(group.securityGroupId), { name, description: group.description });
        this.securityGroups = this.securityGroups.map((item) => item.securityGroupId === updated.securityGroupId ? updated : item);
      }
      this.apiMessage = '名称已更新';
      this.closeQuickRename();
    } catch (error) {
      this.quickRenameError = this.errorMessage(error);
    } finally {
      this.quickRenameSaving = false;
      if (!this.quickRenameError) this.closeQuickRename();
      this.notifyStateChanged();
    }
  }

  async toggleOperator(operator: OperatorUser): Promise<void> {
    const nextStatus = operator.status === 'active' ? 'disabled' : 'active';
    if (nextStatus === 'disabled' &&
        !await this.requestConfirmation('停用运营账号', `停用后，${operator.email} 将无法登录运营后台。`, '确认停用')) {
      return;
    }
    try {
      const updated = await this.request<OperatorUser>('PATCH', OPS_API.operator(operator.operatorId), {
        name: operator.name,
        email: operator.email,
        role: operator.role,
        status: nextStatus,
      });
      Object.assign(operator, { ...updated, lastLoginAt: this.formatDateTime(updated.lastLoginAt) });
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openCurrentPasswordDialog(): void {
    this.oldPassword = '';
    this.newPassword = '';
    this.confirmPassword = '';
    this.passwordMessage = '';
    this.showCurrentPasswordDialog = true;
  }

  closeCurrentPasswordDialog(): void {
    this.showCurrentPasswordDialog = false;
  }

  async saveCurrentPassword(): Promise<void> {
    this.passwordMessage = this.validatePassword(this.newPassword, this.confirmPassword, true);
    if (this.passwordMessage) {
      return;
    }
    try {
      await this.request('PATCH', OPS_API.authPassword, {
        oldPassword: this.oldPassword,
        newPassword: this.newPassword,
      });
      this.closeCurrentPasswordDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.passwordMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openOperatorPasswordDialog(operator: OperatorUser): void {
    this.selectedOperator = operator;
    this.operatorNewPassword = '';
    this.operatorConfirmPassword = '';
    this.passwordMessage = '';
    this.showOperatorPasswordDialog = true;
  }

  closeOperatorPasswordDialog(): void {
    this.showOperatorPasswordDialog = false;
    this.selectedOperator = null;
  }

  async saveOperatorPassword(): Promise<void> {
    this.passwordMessage = this.validatePassword(this.operatorNewPassword, this.operatorConfirmPassword, false);
    if (this.passwordMessage || !this.selectedOperator) {
      return;
    }
    try {
      await this.request('POST', OPS_API.operatorPassword(this.selectedOperator.operatorId), {
        password: this.operatorNewPassword,
      });
      this.closeOperatorPasswordDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.passwordMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  private validatePassword(password: string, confirmPassword: string, requireOldPassword: boolean): string {
    if (requireOldPassword && !this.oldPassword.trim()) {
      return '请输入旧密码';
    }
    if (!password.trim() || !confirmPassword.trim()) {
      return '请输入新密码并确认';
    }
    if (password !== confirmPassword) {
      return '两次输入的新密码不一致';
    }
	if (password.length < 12 || password.length > 72) {
	  return '新密码长度需为 12 至 72 位';
    }
    return '';
  }

  async toggleRelayNode(node: RelayNode): Promise<void> {
    const enabled = node.status !== 'active';
    if (!enabled &&
        !await this.requestConfirmation('停用中继节点', `停用 ${node.name} 后，该节点将不再承载新的中继连接。`, '确认停用')) {
      return;
    }
    try {
      const updated = await this.request<RelayNode>('PATCH', OPS_API.relayNodeStatus(node.nodeId), {
        enabled,
      });
      Object.assign(node, updated);
      this.apiMessage = `${this.relayTransportLabel(node.transport)} ${node.name} 已${enabled ? '启用' : '停用'}`;
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async deleteRelayNode(node: RelayNode): Promise<void> {
    if (!await this.requestConfirmation('删除中继节点', `删除 ${node.name} 后无法恢复，请确认该节点已不再承载业务。`, '确认删除')) {
      return;
    }
    try {
      await this.request('DELETE', OPS_API.relayNode(node.nodeId));
      this.relayNodes = this.relayNodes.filter((item) => item.nodeId !== node.nodeId);
      this.apiMessage = `${this.relayTransportLabel(node.transport)} ${node.name} 已删除`;
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async togglePunchNode(node: PunchNode): Promise<void> {
    const enabled = node.status !== 'active';
    if (!enabled &&
        !await this.requestConfirmation('停用打洞节点', `停用 ${node.name} 后，该节点将不再承载新的打洞会话。`, '确认停用')) {
      return;
    }
    try {
      const updated = await this.request<PunchNode>('PATCH', OPS_API.punchNodeStatus(node.nodeId), {
        enabled,
      });
      Object.assign(node, {
        ...updated,
        createdAt: this.formatDateTime(updated.createdAt),
        updatedAt: this.formatDateTime(updated.updatedAt),
      });
      this.apiMessage = `UDP 打洞 ${node.name} 已${enabled ? '启用' : '停用'}`;
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async deletePunchNode(node: PunchNode): Promise<void> {
    if (!await this.requestConfirmation('删除打洞节点', `删除 ${node.name} 后无法恢复，请确认该节点已不再承载业务。`, '确认删除')) {
      return;
    }
    try {
      await this.request('DELETE', OPS_API.punchNode(node.nodeId));
      this.punchNodes = this.punchNodes.filter((item) => item.nodeId !== node.nodeId);
      this.apiMessage = `UDP 打洞 ${node.name} 已删除`;
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async toggleDevice(device: OpsDevice): Promise<void> {
    const enabled = !device.deviceEnabled;
    if (!enabled &&
        !await this.requestConfirmation('停用设备', `停用后，设备 ${device.name || device.deviceId} 将无法接入网络。`, '确认停用')) {
      return;
    }
    try {
      const updated = await this.request<OpsDevice>('PATCH', OPS_API.device(device.deviceId), {
        status: enabled ? 'active' : 'disabled',
        enabled,
      });
      Object.assign(device, this.formatDevice(updated));
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async deleteDevice(device: OpsDevice): Promise<void> {
    if (!await this.requestConfirmation('删除设备', `删除 ${device.name || device.deviceId} 后，相关授权和网络关系将同时失效。`, '确认删除')) {
      return;
    }
    try {
      await this.request('DELETE', OPS_API.device(device.deviceId));
      this.devices = this.devices.filter((item) => item.deviceId !== device.deviceId);
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  groupIdsForDevice(deviceId: string): string[] {
    return this.deviceGroupMembers
      .filter((member) => member.deviceId === deviceId)
      .map((member) => member.groupId);
  }

  get filteredDeviceGroupAssignments(): OpsDeviceGroup[] {
    const keyword = this.deviceGroupAssignmentKeyword.trim().toLowerCase();
    if (!keyword) return this.deviceGroups;
    return this.deviceGroups.filter((group) => [group.name, group.description]
      .some((value) => String(value ?? '').toLowerCase().includes(keyword)));
  }

  openDeviceGroupAssignmentDialog(device: OpsDevice): void {
    this.deviceGroupAssignmentDevice = device;
    this.deviceGroupAssignmentIds = this.groupIdsForDevice(device.deviceId);
    this.deviceGroupAssignmentKeyword = '';
    this.deviceGroupAssignmentError = '';
    this.showDeviceGroupAssignmentDialog = true;
  }

  closeDeviceGroupAssignmentDialog(): void {
    if (this.deviceGroupAssignmentSaving) return;
    this.showDeviceGroupAssignmentDialog = false;
    this.deviceGroupAssignmentDevice = null;
    this.deviceGroupAssignmentError = '';
  }

  isDeviceGroupAssignmentSelected(groupId: string): boolean {
    return this.deviceGroupAssignmentIds.includes(groupId);
  }

  setDeviceGroupAssignmentSelected(groupId: string, selected: boolean): void {
    const ids = new Set(this.deviceGroupAssignmentIds);
    if (selected) ids.add(groupId); else ids.delete(groupId);
    this.deviceGroupAssignmentIds = [...ids];
    this.deviceGroupAssignmentError = '';
  }

  selectFilteredDeviceGroupAssignments(): void {
    this.deviceGroupAssignmentIds = [...new Set([
      ...this.deviceGroupAssignmentIds,
      ...this.filteredDeviceGroupAssignments.map((group) => group.groupId),
    ])];
  }

  clearDeviceGroupAssignments(): void { this.deviceGroupAssignmentIds = []; }

  async saveDeviceGroupAssignments(): Promise<void> {
    const device = this.deviceGroupAssignmentDevice;
    if (!device || this.deviceGroupAssignmentSaving) return;
    const current = new Set(this.groupIdsForDevice(device.deviceId));
    const desired = new Set(this.deviceGroupAssignmentIds);
    const removals = [...current].filter((groupId) => !desired.has(groupId));
    const additions = [...desired].filter((groupId) => !current.has(groupId));
    if (removals.length > 0 &&
        !await this.requestConfirmation('解除设备组绑定', `将从设备 ${device.name || device.deviceId} 解除 ${removals.length} 个设备组，相关网络权限可能立即变化。`, '确认解除')) {
      return;
    }
    this.deviceGroupAssignmentSaving = true;
    this.deviceGroupAssignmentError = '';
    try {
      for (const groupId of removals) {
        await this.request('DELETE', OPS_API.deviceGroupDevice(groupId, device.deviceId));
      }
      for (const groupId of additions) {
        await this.request('POST', OPS_API.deviceGroupDevices(groupId), { deviceId: device.deviceId });
      }
      await this.reloadOpsResources();
      this.showDeviceGroupAssignmentDialog = false;
      this.deviceGroupAssignmentDevice = null;
      this.apiMessage = '设备组绑定已更新';
    } catch (error) {
      this.deviceGroupAssignmentError = this.errorMessage(error);
      await this.reloadOpsResources().catch(() => undefined);
      this.deviceGroupAssignmentIds = this.groupIdsForDevice(device.deviceId);
    } finally {
      this.deviceGroupAssignmentSaving = false;
      this.notifyStateChanged();
    }
  }

  credentialsForDevice(deviceId: string): DeviceCredential[] {
    return this.deviceCredentials.filter((credential) => credential.deviceId === deviceId);
  }

  hasActiveCredential(deviceId: string): boolean {
    return this.credentialsForDevice(deviceId).some((credential) => credential.status === 'active');
  }

  closeCredentialDialog(): void {
    this.showCredentialDialog = false;
    this.createdCredentialKey = '';
  }

  async createDeviceCredential(): Promise<void> {
    if (this.credentialCreating) {
      return;
    }
    this.credentialCreating = true;
    this.createdCredentialKey = '';
    try {
      const created = await this.request<DeviceCredential & { key: string }>('POST', OPS_API.deviceCredentials, {
        scopes: 'standard_device',
      });
      this.createdCredentialKey = created.key;
      this.deviceCredentials = [this.formatDeviceCredential(created), ...this.deviceCredentials];
      this.showCredentialDialog = true;
      this.apiMessage = '授权 Key 已创建';
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
    } finally {
      this.credentialCreating = false;
      this.notifyStateChanged();
    }
  }


  openNetworkDialog(network?: OpsNetwork): void {
    this.selectedNetwork = network ?? null;
    this.networkForm = network ? { name: network.name, intraGroupPolicy: network.intraGroupPolicy, status: network.status } : { name: '', intraGroupPolicy: 'allow', status: 'active' };
    this.showNetworkDialog = true;
  }
  closeNetworkDialog(): void { this.showNetworkDialog = false; this.selectedNetwork = null; }
  async saveNetwork(): Promise<void> {
    if (this.selectedNetwork?.status === 'active' && this.networkForm.status === 'disabled' &&
        !await this.requestConfirmation('停用网络', `停用 ${this.selectedNetwork.name} 后，网络内设备将无法继续通信。`, '确认停用')) {
      return;
    }
    try {
	  const editing = Boolean(this.selectedNetwork);
      const path = editing ? OPS_API.network(this.selectedNetwork!.networkId) : OPS_API.networks;
      const method = editing ? 'PATCH' : 'POST';
      const saved = await this.request<OpsNetwork>(method, path, this.networkForm);
      this.networks = [saved, ...this.networks.filter((item) => item.networkId !== saved.networkId)];
      this.closeNetworkDialog(); this.apiMessage = editing ? '网络已更新' : '网络已创建';
    } catch (error) { this.apiMessage = this.errorMessage(error); } finally { this.notifyStateChanged(); }
  }
  async deleteNetwork(network: OpsNetwork): Promise<void> {
    if (!await this.requestConfirmation('删除网络', `删除 ${network.name} 后，设备组、域名和安全策略关系将失效且无法恢复。`, '确认删除')) return;
    try { await this.request('DELETE', OPS_API.network(network.networkId)); this.networks = this.networks.filter((item) => item.networkId !== network.networkId); this.apiMessage = '网络已删除'; }
    catch (error) { this.apiMessage = this.errorMessage(error); } finally { this.notifyStateChanged(); }
  }
  openNetworkBindingDialog(network: OpsNetwork): void {
    this.selectedNetwork = network;
    this.networkBindingIds = [...network.deviceGroupIds];
    this.networkBindingKeyword = '';
    this.networkBindingError = '';
    this.showNetworkBindingDialog = true;
  }
  closeNetworkBindingDialog(): void {
    if (this.networkBindingSaving) return;
    this.showNetworkBindingDialog = false;
    this.selectedNetwork = null;
    this.networkBindingError = '';
  }
  get filteredNetworkBindingGroups(): OpsDeviceGroup[] {
    const keyword = this.networkBindingKeyword.trim().toLowerCase();
    if (!keyword) return this.deviceGroups;
    return this.deviceGroups.filter((group) => [group.name, group.description]
      .some((value) => String(value ?? '').toLowerCase().includes(keyword)));
  }
  isNetworkBindingSelected(groupId: string): boolean { return this.networkBindingIds.includes(groupId); }
  setNetworkBindingSelected(groupId: string, selected: boolean): void {
    const ids = new Set(this.networkBindingIds);
    if (selected) ids.add(groupId); else ids.delete(groupId);
    this.networkBindingIds = [...ids];
    this.networkBindingError = '';
  }
  selectFilteredNetworkBindings(): void {
    this.networkBindingIds = [...new Set([
      ...this.networkBindingIds,
      ...this.filteredNetworkBindingGroups.map((group) => group.groupId),
    ])];
  }
  clearNetworkBindings(): void { this.networkBindingIds = []; }
  async saveNetworkBinding(): Promise<void> {
    const network = this.selectedNetwork;
    if (!network || this.networkBindingSaving) return;
    const current = new Set(network.deviceGroupIds);
    const desired = new Set(this.networkBindingIds);
    const removals = [...current].filter((groupId) => !desired.has(groupId));
    const additions = [...desired].filter((groupId) => !current.has(groupId));
    if (removals.length > 0 &&
        !await this.requestConfirmation('移除网络设备组', `将从网络 ${network.name} 移除 ${removals.length} 个设备组，组内设备会失去该网络访问权限。`, '确认移除')) {
      return;
    }
    this.networkBindingSaving = true;
    this.networkBindingError = '';
    try {
      for (const groupId of removals) {
        await this.request('DELETE', OPS_API.networkDeviceGroup(network.networkId, groupId));
      }
      for (const groupId of additions) {
        await this.request('POST', OPS_API.networkDeviceGroups(network.networkId), { groupId });
      }
      await this.reloadOpsResources();
      this.showNetworkBindingDialog = false;
      this.selectedNetwork = null;
      this.apiMessage = '网络设备组已更新';
    } catch (error) {
      this.networkBindingError = this.errorMessage(error);
      await this.reloadOpsResources().catch(() => undefined);
      const refreshed = this.networks.find((item) => item.networkId === network.networkId);
      this.networkBindingIds = [...(refreshed?.deviceGroupIds ?? network.deviceGroupIds)];
    } finally {
      this.networkBindingSaving = false;
      this.notifyStateChanged();
    }
  }

  networkDeviceLabel(deviceId: string): string {
    const device = this.devices.find((item) => item.deviceId === deviceId);
    return device?.name || deviceId;
  }

  get networkDetailNetwork(): OpsNetwork | null {
    if (!this.networkDetailId) return null;
    return this.networks.find((network) => network.networkId === this.networkDetailId) ?? null;
  }

  get securityGroupDetail(): SecurityGroup | null {
    if (!this.securityGroupDetailId) return null;
    return this.securityGroups.find((group) => group.securityGroupId === this.securityGroupDetailId) ?? null;
  }

  get dnsZoneDetail(): DNSZone | null {
    if (!this.dnsZoneDetailId) return null;
    return this.dnsZones.find((zone) => zone.zoneId === this.dnsZoneDetailId) ?? null;
  }

  networkDetailPath(network: OpsNetwork): string {
    return this.domainManagementPath(network);
  }

  domainManagementPath(network: OpsNetwork): string {
    return `/networks/${encodeURIComponent(network.networkId)}/domains`;
  }

  domainRecordsPath(network: OpsNetwork, zone: DNSZone): string {
    return `${this.domainManagementPath(network)}/${encodeURIComponent(zone.zoneId)}/records`;
  }

  securityGroupRulesPath(network: OpsNetwork, group: SecurityGroup): string {
    return `${this.securityGroupsPath(network)}/${encodeURIComponent(group.securityGroupId)}/rules`;
  }

  securityGroupsPath(network: OpsNetwork): string {
    return `/networks/${encodeURIComponent(network.networkId)}/security-groups`;
  }

  openNetworkPolicyTab(tab: 'dns' | 'security'): void {
    const network = this.networkDetailNetwork;
    if (!network) return;
    this.networkPolicyTab = tab;
    this.dnsZoneDetailId = null;
    this.securityGroupDetailId = null;
    const path = tab === 'security' ? this.securityGroupsPath(network) : this.networkDetailPath(network);
    if (window.location.pathname !== path) window.history.pushState({}, '', path);
    this.notifyStateChanged();
  }

  openNetworkDetail(event: MouseEvent, network: OpsNetwork): void {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    window.history.pushState({}, '', this.networkDetailPath(network));
    void this.activateNetworkDetail(network);
  }

  closeNetworkDetail(): void {
    this.clearNetworkDetail();
    window.history.pushState({}, '', '/');
    this.notifyStateChanged();
  }

  openSecurityGroupRules(group: SecurityGroup): void {
    const network = this.networkDetailNetwork;
    if (!network) return;
    this.securityGroupDetailId = group.securityGroupId;
    this.dnsZoneDetailId = null;
    this.networkPolicyTab = 'security';
    this.securityRuleForm.securityGroupId = group.securityGroupId;
    window.history.pushState({}, '', this.securityGroupRulesPath(network, group));
    this.notifyStateChanged();
  }

  closeSecurityGroupRules(): void {
    const network = this.networkDetailNetwork;
    this.securityGroupDetailId = null;
    this.selectedSecurityRule = null;
    if (network) window.history.pushState({}, '', this.securityGroupsPath(network));
    this.notifyStateChanged();
  }

  openDNSZoneRecords(zone: DNSZone): void {
    const network = this.networkDetailNetwork;
    if (!network) return;
    this.dnsZoneDetailId = zone.zoneId;
    this.securityGroupDetailId = null;
    this.networkPolicyTab = 'dns';
    this.dnsRecordForm.zoneId = zone.zoneId;
    window.history.pushState({}, '', this.domainRecordsPath(network, zone));
    this.notifyStateChanged();
  }

  closeDNSZoneRecords(): void {
    const network = this.networkDetailNetwork;
    this.dnsZoneDetailId = null;
    this.selectedDNSRecord = null;
    if (network) window.history.pushState({}, '', this.domainManagementPath(network));
    this.notifyStateChanged();
  }

  private clearNetworkDetail(): void {
    this.networkDetailId = null;
    this.dnsZoneDetailId = null;
    this.securityGroupDetailId = null;
    this.selectedNetwork = null;
    this.networkPolicyTab = 'dns';
  }

  private async syncNetworkRouteFromLocation(): Promise<void> {
    const match = window.location.pathname.match(
      /^\/networks\/([^/]+)\/(domains|security-groups)(?:\/([^/]+)\/(records|rules))?\/?$/,
    );
    if (!match) {
      if (this.networkDetailId) this.clearNetworkDetail();
      return;
    }
    const networkId = decodeURIComponent(match[1]);
    const network = this.networks.find((item) => item.networkId === networkId);
    if (!network) {
      this.clearNetworkDetail();
      window.history.replaceState({}, '', '/');
      this.apiMessage = '网络不存在或已被删除';
      return;
    }
    const resource = match[2];
    const resourceId = match[3] ? decodeURIComponent(match[3]) : null;
    const child = match[4] ?? null;
    const validChild = (!resourceId && !child)
      || (resource === 'domains' && child === 'records')
      || (resource === 'security-groups' && child === 'rules');
    if (!validChild) {
      window.history.replaceState({}, '', this.domainManagementPath(network));
      await this.activateNetworkDetail(network);
      return;
    }
    const policyTab = resource === 'security-groups' ? 'security' : 'dns';
    const dnsZoneId = resource === 'domains' ? resourceId : null;
    const securityGroupId = resource === 'security-groups' ? resourceId : null;
    await this.activateNetworkDetail(network, policyTab, securityGroupId, dnsZoneId);
  }

  private async activateNetworkDetail(
    network: OpsNetwork,
    policyTab: 'dns' | 'security' = 'dns',
    securityGroupId: string | null = null,
    dnsZoneId: string | null = null,
  ): Promise<void> {
    this.active = 'networks';
    this.selectedNetwork = network;
    this.networkDetailId = network.networkId;
    this.securityGroupDetailId = securityGroupId;
    this.dnsZoneDetailId = dnsZoneId;
    this.networkPolicyTab = policyTab;
    this.resetNetworkPolicyForms();
    await this.loadNetworkPolicies();
    if (securityGroupId && !this.securityGroupDetail) {
      this.securityGroupDetailId = null;
      window.history.replaceState({}, '', this.securityGroupsPath(network));
      this.apiMessage = '安全组不存在或已被删除';
    }
    if (dnsZoneId && !this.dnsZoneDetail) {
      this.dnsZoneDetailId = null;
      window.history.replaceState({}, '', this.domainManagementPath(network));
      this.apiMessage = '域名不存在或已被删除';
    }
  }
  async loadNetworkPolicies(): Promise<void> {
    if (!this.selectedNetwork) return;
    try {
      const id = this.selectedNetwork.networkId;
      const detail = await this.request<NetworkPolicyDetail>('GET', OPS_API.networkPolicy(id));
      this.dnsZones = detail.dns.zones;
      this.dnsRecords = detail.dns.records;
      this.securityGroups = detail.security.groups;
      this.securityRules = detail.security.rules;
      if (!this.dnsRecordForm.zoneId) this.dnsRecordForm.zoneId = this.dnsZones[0]?.zoneId ?? '';
      if (!this.securityRuleForm.securityGroupId) this.securityRuleForm.securityGroupId = this.securityGroups[0]?.securityGroupId ?? '';
    } catch (error) { this.apiMessage = this.errorMessage(error); } finally { this.notifyStateChanged(); }
  }
  resetNetworkPolicyForms(): void {
    this.selectedDNSZone = null; this.selectedDNSRecord = null; this.selectedSecurityRule = null;
    this.dnsZoneForm = { name: '', status: 'active' };
    this.dnsRecordForm = { zoneId: this.dnsZones[0]?.zoneId ?? '', name: '', type: 'A', value: '', port: '', ttl: 300 };
    this.securityGroupForm = { name: '', description: '' };
    this.securityRuleForm = { securityGroupId: this.securityGroups[0]?.securityGroupId ?? '', direction: 'ingress', protocol: 'any', portRange: '', peerType: 'device_group', peerValue: '', action: 'allow', priority: 100, description: '', enabled: true };
  }
  editDNSZone(item: DNSZone): void { this.selectedDNSZone = item; this.dnsZoneForm = { name: item.name, status: item.status }; }
  editDNSRecord(item: DNSRecord): void { this.selectedDNSRecord = item; this.dnsRecordForm = { zoneId: item.zoneId, name: item.name, type: item.type, value: item.value, port: item.port, ttl: item.ttl }; }
  editSecurityRule(item: SecurityRule): void { this.selectedSecurityRule = item; this.securityRuleForm = { securityGroupId: item.securityGroupId, direction: item.direction, protocol: item.protocol, portRange: item.portRange, peerType: item.peerType, peerValue: item.peerValue, action: item.action, priority: item.priority, description: item.description, enabled: item.enabled }; }
  async saveDNSZone(): Promise<void> { if (!this.selectedNetwork || !this.dnsZoneForm.name.trim()) return; const editing = this.selectedDNSZone; if (editing?.status === 'active' && this.dnsZoneForm.status === 'disabled' && !await this.requestConfirmation('停用域名', `停用 ${editing.name} 后，该域名的解析记录将不再生效。`, '确认停用')) return; try { await this.request(editing ? 'PATCH' : 'POST', editing ? OPS_API.dnsZone(editing.zoneId) : OPS_API.networkDNSZones(this.selectedNetwork.networkId), this.dnsZoneForm); this.resetNetworkPolicyForms(); await this.loadNetworkPolicies(); this.apiMessage = 'DNS 区域已保存'; } catch (error) { this.apiMessage = this.errorMessage(error); } }
  async saveDNSRecord(): Promise<void> { if (!this.selectedNetwork || !this.dnsRecordForm.name.trim() || !this.dnsRecordForm.value.trim()) return; const editing = this.selectedDNSRecord; try { await this.request(editing ? 'PATCH' : 'POST', editing ? OPS_API.dnsRecord(editing.recordId) : OPS_API.networkDNSRecords(this.selectedNetwork.networkId), this.dnsRecordForm); this.resetNetworkPolicyForms(); await this.loadNetworkPolicies(); this.apiMessage = 'DNS 记录已保存'; } catch (error) { this.apiMessage = this.errorMessage(error); } }
  async saveSecurityGroup(): Promise<void> { if (!this.selectedNetwork || !this.securityGroupForm.name.trim()) return; try { await this.request('POST', OPS_API.networkSecurityGroups(this.selectedNetwork.networkId), this.securityGroupForm); this.resetNetworkPolicyForms(); await this.loadNetworkPolicies(); this.apiMessage = '安全组已保存'; } catch (error) { this.apiMessage = this.errorMessage(error); } }
  async saveSecurityRule(): Promise<void> { if (!this.securityRuleForm.securityGroupId) return; const editing = this.selectedSecurityRule; if (editing?.enabled && !this.securityRuleForm.enabled && !await this.requestConfirmation('停用访问规则', '停用后，该规则将不再参与网络访问控制。', '确认停用')) return; try { const body = { ...this.securityRuleForm }; delete (body as Partial<typeof body>).securityGroupId; await this.request(editing ? 'PATCH' : 'POST', editing ? OPS_API.securityRule(editing.ruleId) : OPS_API.securityGroupRules(this.securityRuleForm.securityGroupId), body); this.resetNetworkPolicyForms(); await this.loadNetworkPolicies(); this.apiMessage = '访问规则已保存'; } catch (error) { this.apiMessage = this.errorMessage(error); } }
  async deleteDNSZone(item: DNSZone): Promise<void> { if (!await this.requestConfirmation('删除域名', `删除 ${item.name} 后，其全部解析记录也将失效且无法恢复。`, '确认删除')) return; await this.deleteNetworkPolicy(OPS_API.dnsZone(item.zoneId), 'DNS 区域已删除'); }
  async deleteDNSRecord(item: DNSRecord): Promise<void> { if (!await this.requestConfirmation('删除解析记录', `删除主机记录 ${item.name} 后，客户端将无法再通过该记录解析目标。`, '确认删除')) return; await this.deleteNetworkPolicy(OPS_API.dnsRecord(item.recordId), 'DNS 记录已删除'); }
  async deleteSecurityGroup(item: SecurityGroup): Promise<void> { if (!await this.requestConfirmation('删除安全组', `删除 ${item.name} 后，其访问规则将一并失效且无法恢复。`, '确认删除')) return; const wasOpen = this.securityGroupDetailId === item.securityGroupId; await this.deleteNetworkPolicy(OPS_API.securityGroup(item.securityGroupId), '安全组已删除'); if (wasOpen) this.closeSecurityGroupRules(); }
  async deleteSecurityRule(item: SecurityRule): Promise<void> { if (!await this.requestConfirmation('删除访问规则', `删除该${item.direction === 'ingress' ? '入方向' : '出方向'}规则后无法恢复。`, '确认删除')) return; await this.deleteNetworkPolicy(OPS_API.securityRule(item.ruleId), '访问规则已删除'); }
  private async deleteNetworkPolicy(path: string, message: string): Promise<void> { try { await this.request('DELETE', path); this.resetNetworkPolicyForms(); await this.loadNetworkPolicies(); this.apiMessage = message; } catch (error) { this.apiMessage = this.errorMessage(error); } }

  membersForGroup(groupId: string): OpsDeviceGroupMember[] { return this.deviceGroupMembers.filter((item) => item.groupId === groupId); }
  memberCountForGroup(groupId: string): number { return this.membersForGroup(groupId).length; }
  get filteredDeviceGroupBindingDevices(): OpsDevice[] {
    const keyword = this.deviceGroupBindingKeyword.trim().toLowerCase();
    if (!keyword) return this.devices;
    return this.devices.filter((device) => [device.name, device.deviceId, device.globalIp]
      .some((value) => String(value ?? '').toLowerCase().includes(keyword)));
  }
  isDeviceGroupBindingSelected(deviceId: string): boolean { return this.deviceGroupBindingDeviceIds.includes(deviceId); }
  setDeviceGroupBindingSelected(deviceId: string, selected: boolean): void {
    const ids = new Set(this.deviceGroupBindingDeviceIds);
    if (selected) ids.add(deviceId); else ids.delete(deviceId);
    this.deviceGroupBindingDeviceIds = [...ids];
    this.deviceGroupBindingError = '';
  }
  selectFilteredDeviceGroupBindings(): void {
    this.deviceGroupBindingDeviceIds = [...new Set([
      ...this.deviceGroupBindingDeviceIds,
      ...this.filteredDeviceGroupBindingDevices.map((device) => device.deviceId),
    ])];
  }
  clearDeviceGroupBindings(): void { this.deviceGroupBindingDeviceIds = []; }
  openDeviceGroupDialog(group?: OpsDeviceGroup): void { this.selectedDeviceGroup = group ?? null; this.deviceGroupForm = group ? { name: group.name, description: group.description } : { name: '', description: '' }; this.showDeviceGroupDialog = true; }
  async saveDeviceGroup(): Promise<void> {
    try {
      const path = this.selectedDeviceGroup ? OPS_API.deviceGroup(this.selectedDeviceGroup.groupId) : OPS_API.deviceGroups;
      const method = this.selectedDeviceGroup ? 'PATCH' : 'POST';
      await this.request(method, path, this.deviceGroupForm); await this.reloadOpsResources(); this.showDeviceGroupDialog = false; this.apiMessage = '设备组已保存';
    } catch (error) { this.apiMessage = this.errorMessage(error); } finally { this.notifyStateChanged(); }
  }
  async deleteDeviceGroup(group: OpsDeviceGroup): Promise<void> {
    if (!await this.requestConfirmation('删除设备组', `删除 ${group.name} 后，设备成员关系和网络引用将失效且无法恢复。`, '确认删除')) return;
    try { await this.request('DELETE', OPS_API.deviceGroup(group.groupId)); await this.reloadOpsResources(); this.apiMessage = '设备组已删除'; }
    catch (error) { this.apiMessage = this.errorMessage(error); } finally { this.notifyStateChanged(); }
  }
  openDeviceGroupBindingDialog(group: OpsDeviceGroup): void {
    this.selectedDeviceGroup = group;
    this.deviceGroupBindingDeviceIds = this.membersForGroup(group.groupId).map((member) => member.deviceId);
    this.deviceGroupBindingKeyword = '';
    this.deviceGroupBindingError = '';
    this.showDeviceGroupBindingDialog = true;
  }
  closeDeviceGroupBindingDialog(): void {
    if (this.deviceGroupBindingSaving) return;
    this.showDeviceGroupBindingDialog = false;
    this.selectedDeviceGroup = null;
    this.deviceGroupBindingError = '';
  }
  async saveDeviceGroupBinding(): Promise<void> {
    const group = this.selectedDeviceGroup;
    if (!group || this.deviceGroupBindingSaving) return;
    const current = new Set(this.membersForGroup(group.groupId).map((member) => member.deviceId));
    const desired = new Set(this.deviceGroupBindingDeviceIds);
    const removals = [...current].filter((deviceId) => !desired.has(deviceId));
    const additions = [...desired].filter((deviceId) => !current.has(deviceId));
    if (removals.length > 0 &&
        !await this.requestConfirmation('移除设备组成员', `将从设备组 ${group.name} 移除 ${removals.length} 台设备，相关网络权限可能立即变化。`, '确认移除')) {
      return;
    }
    this.deviceGroupBindingSaving = true;
    this.deviceGroupBindingError = '';
    try {
      for (const deviceId of removals) {
        await this.request('DELETE', OPS_API.deviceGroupDevice(group.groupId, deviceId));
      }
      for (const deviceId of additions) {
        await this.request('POST', OPS_API.deviceGroupDevices(group.groupId), { deviceId });
      }
      await this.reloadOpsResources();
      this.showDeviceGroupBindingDialog = false;
      this.selectedDeviceGroup = null;
      this.apiMessage = '设备组成员已更新';
    } catch (error) {
      this.deviceGroupBindingError = this.errorMessage(error);
      await this.reloadOpsResources().catch(() => undefined);
      this.deviceGroupBindingDeviceIds = this.membersForGroup(group.groupId).map((member) => member.deviceId);
    } finally {
      this.deviceGroupBindingSaving = false;
      this.notifyStateChanged();
    }
  }
  private async reloadOpsResources(): Promise<void> {
    const [networks, groups] = await Promise.all([this.request<{ items: OpsNetwork[] }>('GET', OPS_API.networks), this.request<{ items: OpsDeviceGroup[]; members: OpsDeviceGroupMember[] }>('GET', OPS_API.deviceGroups)]);
    this.networks = networks.items; this.deviceGroups = groups.items; this.deviceGroupMembers = groups.members;
  }

  private formatDeviceCredential(item: DeviceCredential): DeviceCredential {
    return {
      ...item,
      lastUsedAt: item.lastUsedAt ? this.formatDateTime(item.lastUsedAt) : '',
      createdAt: this.formatDateTime(item.createdAt),
      updatedAt: this.formatDateTime(item.updatedAt),
    };
  }

  private formatDevice(device: OpsDevice): OpsDevice {
    return {
      ...device,
      lastSeenAt: device.lastSeenAt ? this.formatDateTime(device.lastSeenAt) : '-',
      lastReportAt: device.lastReportAt ? this.formatDateTime(device.lastReportAt) : '-',
      createdAt: this.formatDateTime(device.createdAt),
      updatedAt: this.formatDateTime(device.updatedAt),
    };
  }
}
