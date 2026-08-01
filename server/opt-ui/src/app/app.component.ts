import { CommonModule } from '@angular/common';
import { ChangeDetectorRef, Component, OnInit, ViewEncapsulation } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DevicesPageComponent } from './features/devices/devices-page.component';
import { DeviceGroupsPageComponent } from './features/device-groups/device-groups-page.component';
import { NetworksPageComponent } from './features/networks/networks-page.component';
import { OperatorsPageComponent } from './features/operators/operators-page.component';
import { PunchNodesPageComponent } from './features/punch-nodes/punch-nodes-page.component';
import { RelayNodesPageComponent } from './features/relay-nodes/relay-nodes-page.component';
import { UsersPageComponent } from './features/users/users-page.component';
import { OPS_API } from './api-paths';

// 运营后台左侧导航的页面标识，必须和模板中的条件渲染保持一致。
type NavId = 'overview' | 'operators' | 'relayNodes' | 'punchNodes' | 'users' | 'devices' | 'deviceGroups' | 'networks';

// 后台操作员账号模型，用于登录后权限展示和账号维护。
type OperatorUser = {
  operatorId: string;
  name: string;
  email: string;
  role: 'super_admin' | 'owner' | 'admin' | 'ops' | 'finance';
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
  region: string;
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
  region: string;
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

// 用户资源模型，聚合地域、设备数量和状态。
type OpsUser = {
  userId: string;
  email: string;
  name: string;
  country: string;
  province: string;
  city: string;
  ipRegion: string;
  ownDevices: number;
  invitedDevices: number;
  relayUsedGb: number;
  status: 'active' | 'limited' | 'expired' | 'disabled';
  updatedAt: string;
};

// 运营视角设备模型，展示全局虚拟地址、在线状态和累计流量。
type OpsDevice = {
  deviceId: string;
  ownerId: string;
  ownerEmail?: string;
  name: string;
  alias?: string;
  platform: string;
  osName?: string;
  osVersion?: string;
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

type DeviceGroup = { groupId: string; userId: string; name: string; description: string; createdAt: string; updatedAt: string };
type DeviceGroupMember = { groupId: string; deviceId: string; addedAt: number };
type NetworkSummary = { network: { networkId: string; ownerId: string; name: string; cidr: string; intraGroupPolicy: string; status: string; default: boolean }; deviceCount: number; memberCount: number; zoneName: string };
type NetworkDevice = { networkId: string; deviceId: string; ownerUserId: string; alias: string; enabled: boolean; status: string };
type SecurityGroup = { securityGroupId: string; networkId: string; name: string; description: string };
type SecurityRule = { ruleId: string; direction: string; protocol: string; portRange: string; peerType: string; peerValue: string; action: string; priority: number; enabled: boolean };
type DNSZone = { zoneId: string; networkId: string; name: string; status: string };
type DNSRecord = { recordId: string; networkId: string; zoneId: string; name: string; type: string; value: string; ttl: number };

@Component({
  selector: 'ops-root',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    OperatorsPageComponent,
    RelayNodesPageComponent,
    PunchNodesPageComponent,
    UsersPageComponent,
    DevicesPageComponent,
    DeviceGroupsPageComponent,
    NetworksPageComponent,
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
  encapsulation: ViewEncapsulation.None,
})
// 运营后台根组件，集中持有页面状态、表单状态和对后端 ops API 的访问逻辑。
export class AppComponent implements OnInit {
  constructor(private readonly changeDetector: ChangeDetectorRef) {}

  // 导航配置同时驱动侧边栏文案和当前页面标题说明。
  readonly navItems: Array<{ id: NavId; label: string; desc: string }> = [
    { id: 'overview', label: '运营管理', desc: '平台核心指标' },
    { id: 'operators', label: '运营用户', desc: '后台账号与角色' },
    { id: 'relayNodes', label: '中继节点', desc: 'Relay/DERP 容量管理' },
    { id: 'punchNodes', label: '打洞节点', desc: 'P2P Punch 节点管理' },
    { id: 'users', label: '用户管理', desc: '用户资料与账户状态' },
    { id: 'devices', label: '设备管理', desc: '全局设备、在线与启用状态' },
    { id: 'deviceGroups', label: '设备分组', desc: '全局设备分组与归属' },
    { id: 'networks', label: '网络管理', desc: '网络、安全组与 DNS' },
  ];

  active: NavId = 'overview';
  readonly opsTokenKey = 'slan_ops_token';
  readonly opsEmailKey = 'slan_ops_email';
  operatorEmail = localStorage.getItem(this.opsEmailKey) || 'admin@slan.local';
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
  showUserDialog = false;
  showUserPasswordDialog = false;
  relayNodeMessage = '';
  punchNodeMessage = '';
  showDeviceDialog = false;
  showDeviceGroupDialog = false;
  showNetworkDialog = false;
  selectedOperator: OperatorUser | null = null;
  selectedRelayNode: RelayNode | null = null;
  selectedPunchNode: PunchNode | null = null;
  selectedUser: OpsUser | null = null;
  selectedDevice: OpsDevice | null = null;
  selectedDeviceGroup: DeviceGroup | null = null;
  managedDeviceGroup: DeviceGroup | null = null;
  selectedNetwork: NetworkSummary | null = null;
  selectedSecurityGroup: SecurityGroup | null = null;
  editingSecurityGroup: SecurityGroup | null = null;
  editingSecurityRule: SecurityRule | null = null;
  editingDNSZone: DNSZone | null = null;
  editingDNSRecord: DNSRecord | null = null;
  oldPassword = '';
  newPassword = '';
  confirmPassword = '';
  operatorNewPassword = '';
  operatorConfirmPassword = '';
  passwordMessage = '';
  operatorForm: OperatorForm = {};
  relayNodeForm: RelayNodeForm = {};
  punchNodeForm: Partial<PunchNode> = {};
  userForm: Partial<OpsUser> = {};
  userPassword = '';
  userConfirmPassword = '';
  userPasswordMessage = '';
  createUserPassword = '';
  createUserConfirmPassword = '';
  deviceForm: Partial<OpsDevice> = {};
  deviceGroupForm: Partial<DeviceGroup> = {};
  networkForm: any = {};
  securityGroupName = '';
  securityGroupDescription = '';
  ruleForm: any = { direction: 'ingress', protocol: 'all', portRange: 'all', action: 'allow', peerType: 'device_group', peerValue: '' };
  dnsZoneName = '';
  dnsRecordForm: any = { zoneId: '', name: '', type: 'A', value: '', ttl: 300 };
  deviceKeyword = '';

  // 传给 feature 子页面的视图模型，保持子页面只负责模板渲染。
  get vm(): this {
    return this;
  }

  get isLoggedIn(): boolean {
    return Boolean(localStorage.getItem(this.opsTokenKey));
  }

  get apiMessageIsSuccess(): boolean {
    return /(已保存|已启用|已停用|已删除|已创建|已更新|已指派|已加入|已离开)/.test(this.apiMessage);
  }

  ngOnInit(): void {
    if (this.isLoggedIn) {
      void this.loadOpsData();
    }
  }

  operators: OperatorUser[] = [];
  relayNodes: RelayNode[] = [];
  punchNodes: PunchNode[] = [];
  users: OpsUser[] = [];
  devices: OpsDevice[] = [];
  deviceGroups: DeviceGroup[] = [];
  deviceGroupMembers: DeviceGroupMember[] = [];
  networks: NetworkSummary[] = [];
  networkDevices: NetworkDevice[] = [];
  networkDeviceGroups: DeviceGroup[] = [];
  networkGroupToAdd = '';
  networkDeviceToAdd = '';
  securityGroups: SecurityGroup[] = [];
  securityRules: SecurityRule[] = [];
  dnsZones: DNSZone[] = [];
  dnsRecords: DNSRecord[] = [];

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
      this.operatorEmail = response.auth.operator.email;
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
    localStorage.removeItem(this.opsTokenKey);
    localStorage.removeItem(this.opsEmailKey);
    this.operatorEmail = 'admin@slan.local';
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
      const [operators, relayNodes, punchNodes, users, devices, deviceGroups, networks] = await Promise.all([
        this.request<{ items: OperatorUser[] }>('GET', OPS_API.operators),
        this.request<{ items: RelayNode[] }>('GET', OPS_API.relayNodes),
        this.request<{ items: PunchNode[] }>('GET', OPS_API.punchNodes),
        this.request<{ items: OpsUser[] }>('GET', OPS_API.users),
        this.request<{ items: OpsDevice[] }>('GET', OPS_API.devices),
        this.request<{ items: DeviceGroup[]; members: DeviceGroupMember[] }>('GET', OPS_API.deviceGroups),
        this.request<{ items: NetworkSummary[] }>('GET', OPS_API.networks),
      ]);
      this.operators = operators.items.map((item) => ({ ...item, lastLoginAt: this.formatDateTime(item.lastLoginAt) }));
      this.relayNodes = relayNodes.items;
      this.punchNodes = punchNodes.items.map((item) => ({
        ...item,
        createdAt: this.formatDateTime(item.createdAt),
        updatedAt: this.formatDateTime(item.updatedAt),
      }));
      this.users = users.items.map((item) => ({
        ...item,
        updatedAt: this.formatDateTime(item.updatedAt),
      }));
      this.devices = devices.items.map((item) => this.formatDevice(item));
      this.deviceGroups = deviceGroups.items.map((item) => ({ ...item, createdAt: this.formatDateTime(item.createdAt), updatedAt: this.formatDateTime(item.updatedAt) }));
      this.deviceGroupMembers = deviceGroups.members || [];
      this.networks = networks.items;
    } catch (error) {
      this.operators = [];
      this.relayNodes = [];
      this.punchNodes = [];
      this.users = [];
      this.devices = [];
      this.deviceGroups = [];
      this.deviceGroupMembers = [];
      this.networks = [];
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
    return this.navItems.find((item) => item.id === this.active) ?? this.navItems[0];
  }

  get totalUsers(): number {
    return this.users.length;
  }

  get filteredDevices(): OpsDevice[] {
    const keyword = this.deviceKeyword.trim().toLowerCase();
    if (!keyword) {
      return this.devices;
    }
    return this.devices.filter((device) => [
      device.deviceId,
      device.ownerEmail,
      device.alias,
      device.name,
      device.platform,
      device.osName,
      device.globalIp,
      device.globalName,
    ].some((value) => String(value ?? '').toLowerCase().includes(keyword)));
  }

  get onlineDeviceCount(): number {
    return this.devices.filter((device) => device.heartbeatOnline).length;
  }

  get totalRelayUsedGb(): number {
    return this.relayNodes.reduce((sum, node) => sum + node.usedTrafficGb, 0);
  }

  get limitedUsers(): number {
    return this.users.filter((user) => user.status === 'limited').length;
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

  setActive(id: NavId): void {
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
      : { name: '', email: '', role: 'ops', status: 'active', password: '', confirmPassword: '' };
    this.showOperatorDialog = true;
  }

  closeOperatorDialog(): void {
    this.showOperatorDialog = false;
    this.selectedOperator = null;
  }

  async saveOperatorDialog(): Promise<void> {
    if (!this.operatorForm.name?.trim() || !this.operatorForm.email?.trim()) {
      this.apiMessage = '请输入运营用户姓名和邮箱';
      return;
    }
    const isEdit = Boolean(this.selectedOperator);
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
      region: 'ap-east-1',
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
    try {
      const isEdit = Boolean(this.selectedRelayNode);
      const path = isEdit ? OPS_API.relayNode(this.selectedRelayNode!.nodeId) : OPS_API.relayNodes;
      const node = await this.request<RelayNode>(isEdit ? 'PATCH' : 'POST', path, {
        nodeId: this.selectedRelayNode?.nodeId,
        name: this.relayNodeForm.name,
        region: this.relayNodeForm.region,
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
      region: 'default',
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
    try {
      const isEdit = Boolean(this.selectedPunchNode);
      const path = isEdit ? OPS_API.punchNode(this.selectedPunchNode!.nodeId) : OPS_API.punchNodes;
      const node = await this.request<PunchNode>(isEdit ? 'PATCH' : 'POST', path, {
        nodeId: this.selectedPunchNode?.nodeId,
        name: this.punchNodeForm.name,
        region: this.punchNodeForm.region,
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

  openUserDialog(user?: OpsUser): void {
    this.selectedUser = user ?? null;
    this.userForm = user ? { ...user } : { status: 'active' };
    this.createUserPassword = '';
    this.createUserConfirmPassword = '';
    this.showUserDialog = true;
  }

  closeUserDialog(): void {
    this.showUserDialog = false;
    this.selectedUser = null;
  }

  async saveUserDialog(): Promise<void> {
    if (!this.userForm.email?.trim()) {
      this.apiMessage = '请输入用户邮箱';
      return;
    }
    if (!this.selectedUser && (!this.createUserPassword || this.createUserPassword !== this.createUserConfirmPassword)) {
      this.apiMessage = this.createUserPassword ? '两次输入的密码不一致' : '请输入用户密码';
      return;
    }
    try {
      const isEdit = Boolean(this.selectedUser);
      const user = await this.request<OpsUser>(
        isEdit ? 'PATCH' : 'POST',
        isEdit ? OPS_API.user(this.selectedUser!.userId) : OPS_API.users,
        {
          email: this.userForm.email,
          name: this.userForm.name,
          country: this.userForm.country,
          province: this.userForm.province,
          city: this.userForm.city,
          ipRegion: this.userForm.ipRegion,
          status: this.userForm.status,
          password: isEdit ? undefined : this.createUserPassword,
        },
      );
      const formatted = {
        ...user,
        updatedAt: this.formatDateTime(user.updatedAt),
      };
      this.users = [
        formatted,
        ...this.users.filter((item) => item.userId !== user.userId),
      ];
      this.closeUserDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openUserPasswordDialog(user: OpsUser): void {
    this.selectedUser = user;
    this.userPassword = '';
    this.userConfirmPassword = '';
    this.userPasswordMessage = '';
    this.showUserPasswordDialog = true;
  }

  closeUserPasswordDialog(): void {
    this.showUserPasswordDialog = false;
    this.selectedUser = null;
  }

  async saveUserPassword(): Promise<void> {
    if (!this.selectedUser || !this.userPassword) {
      this.userPasswordMessage = '请输入新密码';
      return;
    }
    if (this.userPassword !== this.userConfirmPassword) {
      this.userPasswordMessage = '两次输入的密码不一致';
      return;
    }
    try {
      await this.request<OpsUser>('PATCH', OPS_API.userPassword(this.selectedUser.userId), { password: this.userPassword });
      this.closeUserPasswordDialog();
      this.apiMessage = '用户密码已修改';
      this.notifyStateChanged();
    } catch (error) {
      this.userPasswordMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openDeviceDialog(device: OpsDevice): void {
    this.selectedDevice = device;
    this.deviceForm = { ...device };
    this.showDeviceDialog = true;
  }

  closeDeviceDialog(): void {
    this.showDeviceDialog = false;
    this.selectedDevice = null;
  }

  async saveDeviceDialog(): Promise<void> {
    if (!this.selectedDevice) {
      return;
    }
    try {
      const updated = await this.request<OpsDevice>('PATCH', OPS_API.device(this.selectedDevice.deviceId), {
        alias: this.deviceForm.alias,
        status: this.deviceForm.status,
        enabled: this.deviceForm.deviceEnabled,
      });
      const formatted = this.formatDevice(updated);
      this.devices = [formatted, ...this.devices.filter((item) => item.deviceId !== updated.deviceId)];
      this.closeDeviceDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async toggleOperator(operator: OperatorUser): Promise<void> {
    const nextStatus = operator.status === 'active' ? 'disabled' : 'active';
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
    if (password.length < 8) {
      return '新密码至少 8 位';
    }
    return '';
  }

  async toggleRelayNode(node: RelayNode): Promise<void> {
    const enabled = node.status !== 'active';
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
    try {
      const updated = await this.request<OpsDevice>('PATCH', OPS_API.device(device.deviceId), {
        alias: device.alias,
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
    if (!confirm(`确认删除设备 ${device.deviceId}？`)) {
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

  userEmail(userId: string): string {
    return this.users.find((item) => item.userId === userId)?.email || userId;
  }

  openDeviceGroupDialog(group?: DeviceGroup): void {
    this.selectedDeviceGroup = group ?? null;
    this.deviceGroupForm = group ? { ...group } : { userId: this.users[0]?.userId };
    this.showDeviceGroupDialog = true;
  }

  closeDeviceGroupDialog(): void { this.showDeviceGroupDialog = false; this.selectedDeviceGroup = null; }

  async saveDeviceGroup(): Promise<void> {
    const ownerId = this.deviceGroupForm.userId?.trim();
    if (!ownerId || !this.deviceGroupForm.name?.trim()) { this.apiMessage = '请选择所属用户并输入分组名称'; return; }
    try {
      const editing = Boolean(this.selectedDeviceGroup);
      const item = await this.request<DeviceGroup>(editing ? 'PATCH' : 'POST', editing ? OPS_API.deviceGroup(this.selectedDeviceGroup!.groupId) : OPS_API.deviceGroups, { ownerId, name: this.deviceGroupForm.name, description: this.deviceGroupForm.description });
      const formatted = { ...item, createdAt: this.formatDateTime(item.createdAt), updatedAt: this.formatDateTime(item.updatedAt) };
      this.deviceGroups = [formatted, ...this.deviceGroups.filter((group) => group.groupId !== item.groupId)];
      this.closeDeviceGroupDialog(); this.apiMessage = '设备分组已保存'; this.notifyStateChanged();
    } catch (error) { this.apiMessage = this.errorMessage(error); this.notifyStateChanged(); }
  }

  async deleteDeviceGroup(group: DeviceGroup): Promise<void> {
    if (!confirm(`确认删除设备分组 ${group.name}？相关网络引用也会被清理。`)) return;
    try { await this.request('DELETE', OPS_API.deviceGroup(group.groupId), { ownerId: group.userId }); this.deviceGroups = this.deviceGroups.filter((item) => item.groupId !== group.groupId); this.apiMessage = '设备分组已删除'; this.notifyStateChanged(); }
    catch (error) { this.apiMessage = this.errorMessage(error); this.notifyStateChanged(); }
  }

  manageDeviceGroup(group: DeviceGroup): void { this.managedDeviceGroup = group; }
  devicesForManagedGroup(): OpsDevice[] { return this.managedDeviceGroup ? this.devices.filter((device) => device.ownerId === this.managedDeviceGroup!.userId) : []; }
  deviceInManagedGroup(deviceId: string): boolean { return Boolean(this.managedDeviceGroup && this.deviceGroupMembers.some((member) => member.groupId === this.managedDeviceGroup!.groupId && member.deviceId === deviceId)); }
  async toggleManagedGroupDevice(device: OpsDevice): Promise<void> {
    if (!this.managedDeviceGroup) return;
    const current = this.deviceGroupMembers.filter((member) => member.deviceId === device.deviceId).map((member) => member.groupId);
    const groupId = this.managedDeviceGroup.groupId;
    const groupIds = current.includes(groupId) ? current.filter((id) => id !== groupId) : [...current, groupId];
    try {
      await this.request('PUT', OPS_API.deviceGroupsForDevice(device.deviceId), { ownerId: this.managedDeviceGroup.userId, groupIds });
      this.deviceGroupMembers = this.deviceGroupMembers.filter((member) => member.deviceId !== device.deviceId);
      this.deviceGroupMembers.push(...groupIds.map((id) => ({ groupId: id, deviceId: device.deviceId, addedAt: Math.floor(Date.now() / 1000) })));
      this.apiMessage = '设备分组成员已更新'; this.notifyStateChanged();
    } catch (error) { this.apiMessage = this.errorMessage(error); this.notifyStateChanged(); }
  }

  openNetworkDialog(item?: NetworkSummary): void {
    this.networkForm = item ? { ...item.network } : { ownerId: this.users[0]?.userId, intraGroupPolicy: 'allow', status: 'active' };
    this.showNetworkDialog = true;
  }
  closeNetworkDialog(): void { this.showNetworkDialog = false; }
  async saveNetwork(): Promise<void> {
    if (!this.networkForm.ownerId || !this.networkForm.name?.trim()) { this.apiMessage = '请选择所属用户并输入网络名称'; return; }
    try {
      const editing = Boolean(this.networkForm.networkId);
      const item = await this.request<NetworkSummary>(editing ? 'PATCH' : 'POST', editing ? OPS_API.network(this.networkForm.networkId) : OPS_API.networks, this.networkForm);
      this.networks = [item, ...this.networks.filter((network) => network.network.networkId !== item.network.networkId)];
      this.showNetworkDialog = false; this.apiMessage = '网络已保存'; this.notifyStateChanged();
    } catch (error) { this.apiMessage = this.errorMessage(error); this.notifyStateChanged(); }
  }
  async deleteNetwork(item: NetworkSummary): Promise<void> {
    if (!confirm(`确认删除网络 ${item.network.name}？网络内安全组和 DNS 数据将一并删除。`)) return;
    try { await this.request('DELETE', OPS_API.network(item.network.networkId), { ownerId: item.network.ownerId }); this.networks = this.networks.filter((network) => network.network.networkId !== item.network.networkId); if (this.selectedNetwork?.network.networkId === item.network.networkId) this.selectedNetwork = null; this.apiMessage = '网络已删除'; this.notifyStateChanged(); }
    catch (error) { this.apiMessage = this.errorMessage(error); this.notifyStateChanged(); }
  }
  async selectNetwork(item: NetworkSummary): Promise<void> {
    this.selectedNetwork = item; this.selectedSecurityGroup = null; this.securityRules = [];
    this.ruleForm = { direction: 'ingress', protocol: 'all', portRange: 'all', action: 'allow', peerType: 'device_group', peerValue: this.deviceGroups.find((group) => group.userId === item.network.ownerId)?.groupId || '' };
    try {
      const [groups, zones, records, networkDevices, networkGroups] = await Promise.all([
        this.request<{items: SecurityGroup[]}>('GET', OPS_API.securityGroups(item.network.networkId)),
        this.request<{items: DNSZone[]}>('GET', OPS_API.dnsZones(item.network.networkId)),
        this.request<{items: DNSRecord[]}>('GET', OPS_API.dnsRecords(item.network.networkId)),
        this.request<{items: NetworkDevice[]}>('GET', OPS_API.networkDevices(item.network.networkId)),
        this.request<{items: DeviceGroup[]}>('GET', OPS_API.networkDeviceGroups(item.network.networkId)),
      ]);
      this.securityGroups = groups.items; this.dnsZones = zones.items; this.dnsRecords = records.items; this.networkDevices = networkDevices.items; this.networkDeviceGroups = networkGroups.items;
      this.networkDeviceToAdd = this.availableNetworkDevices()[0]?.deviceId || '';
      this.networkGroupToAdd = this.deviceGroups.find((group) => group.userId === item.network.ownerId && !this.networkDeviceGroups.some((current) => current.groupId === group.groupId))?.groupId || '';
      this.notifyStateChanged();
    } catch (error) { this.apiMessage = this.errorMessage(error); this.notifyStateChanged(); }
  }
  async addNetworkDeviceGroup(): Promise<void> { if (!this.selectedNetwork || !this.networkGroupToAdd) return; try { const response = await this.request<{items: DeviceGroup[]}>('POST', OPS_API.networkDeviceGroups(this.selectedNetwork.network.networkId), { ownerId: this.selectedNetwork.network.ownerId, groupId: this.networkGroupToAdd }); this.networkDeviceGroups = response.items; this.networkGroupToAdd = ''; this.apiMessage = '网络引用分组已更新'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }
  availableNetworkDevices(): OpsDevice[] { return this.selectedNetwork ? this.devices.filter((device) => device.ownerId === this.selectedNetwork!.network.ownerId && !this.networkDevices.some((member) => member.deviceId === device.deviceId)) : []; }
  async addNetworkDevice(): Promise<void> { if (!this.selectedNetwork || !this.networkDeviceToAdd) return; try { const item = await this.request<NetworkDevice>('POST', OPS_API.networkDevice(this.selectedNetwork.network.networkId, this.networkDeviceToAdd), { ownerId: this.selectedNetwork.network.ownerId }); this.networkDevices = [...this.networkDevices.filter((member) => member.deviceId !== item.deviceId), item]; this.networkDeviceToAdd = this.availableNetworkDevices()[0]?.deviceId || ''; this.apiMessage = '设备已加入网络并通知客户端'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }
  async removeNetworkDevice(device: NetworkDevice): Promise<void> { if (!this.selectedNetwork || !confirm(`确认将设备 ${device.alias || device.deviceId} 移出网络？`)) return; try { await this.request('DELETE', OPS_API.networkDevice(this.selectedNetwork.network.networkId, device.deviceId), { ownerId: this.selectedNetwork.network.ownerId }); this.networkDevices = this.networkDevices.filter((member) => member.deviceId !== device.deviceId); this.networkDeviceToAdd = this.availableNetworkDevices()[0]?.deviceId || ''; this.apiMessage = '设备已离开网络并通知客户端'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }
  availableNetworkDeviceGroups(): DeviceGroup[] { return this.selectedNetwork ? this.deviceGroups.filter((group) => group.userId === this.selectedNetwork!.network.ownerId && !this.networkDeviceGroups.some((current) => current.groupId === group.groupId)) : []; }
  async removeNetworkDeviceGroup(group: DeviceGroup): Promise<void> { if (!this.selectedNetwork) return; try { const response = await this.request<{items: DeviceGroup[]}>('DELETE', OPS_API.networkDeviceGroup(this.selectedNetwork.network.networkId, group.groupId), { ownerId: this.selectedNetwork.network.ownerId }); this.networkDeviceGroups = response.items; this.apiMessage = '网络引用分组已移除'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }
  editSecurityGroup(group: SecurityGroup): void { this.editingSecurityGroup = group; this.securityGroupName = group.name; this.securityGroupDescription = group.description; }
  async createSecurityGroup(): Promise<void> {
    if (!this.selectedNetwork || !this.securityGroupName.trim()) return;
    try { const editing = Boolean(this.editingSecurityGroup); const item = await this.request<SecurityGroup>(editing ? 'PATCH' : 'POST', editing ? OPS_API.securityGroup(this.editingSecurityGroup!.securityGroupId) : OPS_API.securityGroups(this.selectedNetwork.network.networkId), { ownerId: this.selectedNetwork.network.ownerId, name: this.securityGroupName, description: this.securityGroupDescription }); this.securityGroups = [...this.securityGroups.filter((group) => group.securityGroupId !== item.securityGroupId), item]; this.securityGroupName = ''; this.securityGroupDescription = ''; this.editingSecurityGroup = null; this.apiMessage = '安全组已保存'; this.notifyStateChanged(); }
    catch (error) { this.apiMessage = this.errorMessage(error); this.notifyStateChanged(); }
  }
  async selectSecurityGroup(group: SecurityGroup): Promise<void> { this.selectedSecurityGroup = group; try { this.securityRules = (await this.request<{items: SecurityRule[]}>('GET', OPS_API.securityRules(group.securityGroupId))).items; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }
  async deleteSecurityGroup(group: SecurityGroup): Promise<void> { if (!this.selectedNetwork || !confirm(`确认删除安全组 ${group.name}？`)) return; try { await this.request('DELETE', OPS_API.securityGroup(group.securityGroupId), { ownerId: this.selectedNetwork.network.ownerId }); this.securityGroups = this.securityGroups.filter((item) => item.securityGroupId !== group.securityGroupId); if (this.selectedSecurityGroup?.securityGroupId === group.securityGroupId) this.selectedSecurityGroup = null; this.apiMessage = '安全组已删除'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }
  editSecurityRule(rule: SecurityRule): void { this.editingSecurityRule = rule; this.ruleForm = { ...rule }; }
  async createSecurityRule(): Promise<void> { if (!this.selectedNetwork || !this.selectedSecurityGroup || !this.ruleForm.peerValue) { this.apiMessage = '请选择规则来源设备或设备分组'; return; } try { const editing = Boolean(this.editingSecurityRule); const item = await this.request<SecurityRule>(editing ? 'PATCH' : 'POST', editing ? OPS_API.securityRule(this.editingSecurityRule!.ruleId) : OPS_API.securityRules(this.selectedSecurityGroup.securityGroupId), { ownerId: this.selectedNetwork.network.ownerId, ...this.ruleForm, priority: Number(this.ruleForm.priority || 100), enabled: this.ruleForm.enabled !== false }); this.securityRules = [...this.securityRules.filter((rule) => rule.ruleId !== item.ruleId), item]; this.editingSecurityRule = null; this.apiMessage = '安全规则已保存'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); this.notifyStateChanged(); } }
  async deleteSecurityRule(rule: SecurityRule): Promise<void> { if (!this.selectedNetwork) return; try { await this.request('DELETE', OPS_API.securityRule(rule.ruleId), { ownerId: this.selectedNetwork.network.ownerId }); this.securityRules = this.securityRules.filter((item) => item.ruleId !== rule.ruleId); this.apiMessage = '安全规则已删除'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }

  editDNSZone(zone: DNSZone): void { this.editingDNSZone = zone; this.dnsZoneName = zone.name; }
  async createDNSZone(): Promise<void> { if (!this.selectedNetwork || !this.dnsZoneName.trim()) return; try { const editing = Boolean(this.editingDNSZone); const item = await this.request<DNSZone>(editing ? 'PATCH' : 'POST', editing ? OPS_API.dnsZone(this.editingDNSZone!.zoneId) : OPS_API.dnsZones(this.selectedNetwork.network.networkId), { ownerId: this.selectedNetwork.network.ownerId, name: this.dnsZoneName, status: this.editingDNSZone?.status || 'active' }); this.dnsZones = [...this.dnsZones.filter((zone) => zone.zoneId !== item.zoneId), item]; this.dnsZoneName = ''; this.editingDNSZone = null; this.apiMessage = 'DNS 区域已保存'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }
  async deleteDNSZone(zone: DNSZone): Promise<void> { if (!this.selectedNetwork || !confirm(`确认删除 DNS 区域 ${zone.name}？`)) return; try { await this.request('DELETE', OPS_API.dnsZone(zone.zoneId), { ownerId: this.selectedNetwork.network.ownerId }); this.dnsZones = this.dnsZones.filter((item) => item.zoneId !== zone.zoneId); this.apiMessage = 'DNS 区域已删除'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }
  editDNSRecord(record: DNSRecord): void { this.editingDNSRecord = record; this.dnsRecordForm = { ...record }; }
  async createDNSRecord(): Promise<void> { if (!this.selectedNetwork || !this.dnsRecordForm.name?.trim() || !this.dnsRecordForm.value?.trim()) return; try { const editing = Boolean(this.editingDNSRecord); const item = await this.request<DNSRecord>(editing ? 'PATCH' : 'POST', editing ? OPS_API.dnsRecord(this.editingDNSRecord!.recordId) : OPS_API.dnsRecords(this.selectedNetwork.network.networkId), { ownerId: this.selectedNetwork.network.ownerId, ...this.dnsRecordForm, ttl: Number(this.dnsRecordForm.ttl || 300) }); this.dnsRecords = [...this.dnsRecords.filter((record) => record.recordId !== item.recordId), item]; this.dnsRecordForm = { zoneId: '', name: '', type: 'A', value: '', ttl: 300 }; this.editingDNSRecord = null; this.apiMessage = 'DNS 记录已保存'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }
  async deleteDNSRecord(record: DNSRecord): Promise<void> { if (!this.selectedNetwork) return; try { await this.request('DELETE', OPS_API.dnsRecord(record.recordId), { ownerId: this.selectedNetwork.network.ownerId }); this.dnsRecords = this.dnsRecords.filter((item) => item.recordId !== record.recordId); this.apiMessage = 'DNS 记录已删除'; this.notifyStateChanged(); } catch (error) { this.apiMessage = this.errorMessage(error); } }

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
