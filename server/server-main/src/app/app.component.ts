import { CommonModule } from '@angular/common';
import { ChangeDetectorRef, Component, OnInit, ViewEncapsulation } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ClientDownloadsPageComponent } from './features/client-downloads/client-downloads-page.component';
import { CustomersPageComponent } from './features/customers/customers-page.component';
import { DevicesPageComponent } from './features/devices/devices-page.component';
import { OperatorsPageComponent } from './features/operators/operators-page.component';
import { OrdersPageComponent } from './features/orders/orders-page.component';
import { OverviewPageComponent } from './features/overview/overview-page.component';
import { ProductsPageComponent } from './features/products/products-page.component';
import { PunchNodesPageComponent } from './features/punch-nodes/punch-nodes-page.component';
import { RelayNodesPageComponent } from './features/relay-nodes/relay-nodes-page.component';
import { RenewalsPageComponent } from './features/renewals/renewals-page.component';

// 运营后台左侧导航的页面标识，必须和模板中的条件渲染保持一致。
type NavId = 'overview' | 'operators' | 'relayNodes' | 'punchNodes' | 'customers' | 'devices' | 'clientDownloads' | 'products' | 'orders' | 'renewals';

// 后台操作员账号模型，用于登录后权限展示和账号维护。
type OperatorUser = {
  operatorId: string;
  name: string;
  email: string;
  role: 'super_admin' | 'owner' | 'admin' | 'ops' | 'finance';
  status: 'active' | 'disabled';
  lastLoginAt: string;
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

// 客户套餐模型，定义设备数、Relay 配额、P2P 能力和高级功能开关。
type CustomerPlan = {
  code: string;
  name: string;
  ownDeviceLimit: number;
  invitedDeviceLimit: number;
  totalDeviceLimit: number;
  relayMonthlyGb: number;
  relayBandwidthMbps: number;
  relayThrottleMbps: number;
  p2pUnlimited: boolean;
  customDomain: boolean;
  acl: boolean;
  dedicatedRelay: boolean;
  auditLog: boolean;
  apiAccess: boolean;
  monthlyPrice: number;
  yearlyPrice: number;
  status?: 'active' | 'offline';
};

// 可售商品模型，覆盖套餐、流量包和企业合同包。
type Product = {
  productId: string;
  name: string;
  type: 'plan' | 'traffic_pack' | 'enterprise';
  planCode?: CustomerPlan['code'];
  period: 'monthly' | 'yearly' | 'one_time' | 'contract';
  validDays: number;
  relayTrafficGb: number;
  relayBandwidthMbps: number;
  listPrice: number;
  salePrice: number;
  currency: 'CNY';
  autoRenew: boolean;
  status: 'active' | 'offline';
  description: string;
};

// 客户资源模型，聚合地域、套餐、设备数量和限流状态。
type Customer = {
  customerId: string;
  email: string;
  name: string;
  country: string;
  province: string;
  city: string;
  ipRegion: string;
  planCode: CustomerPlan['code'];
  planExpiresAt: string;
  ownDevices: number;
  invitedDevices: number;
  relayUsedGb: number;
  status: 'active' | 'limited' | 'expired' | 'disabled';
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

// 客户端发布包模型，用于维护各平台安装包、渠道和校验信息。
type ClientDownload = {
  downloadId: string;
  platform: 'macos' | 'windows' | 'ios' | 'linux' | 'android';
  platformName: string;
  version: string;
  arch?: string;
  channel: 'stable' | 'beta' | 'dev';
  fileName: string;
  fileSize: number;
  sha256?: string;
  downloadUrl: string;
  releaseNotes?: string;
  status: 'active' | 'offline';
  createdAt: string;
  updatedAt: string;
};

// 续费记录模型，记录人工或支付渠道完成的有效期延长操作。
type Renewal = {
  renewalId: string;
  customerId?: string;
  customerEmail: string;
  planCode: CustomerPlan['code'];
  period: 'monthly' | 'yearly' | 'custom';
  amount: number;
  paidAt: string;
  validUntil: string;
  source: 'manual' | 'wechat' | 'alipay' | 'bank';
  operator: string;
};

// 订单模型，跟踪购买、支付和权益开通状态。
type Order = {
  orderId: string;
  customerId?: string;
  customerEmail: string;
  productId?: string;
  productName: string;
  productType: Product['type'];
  amount: number;
  currency?: 'CNY';
  payStatus: 'pending' | 'paid' | 'refunded' | 'closed';
  provisionStatus: 'pending' | 'provisioned' | 'failed';
  createdAt: string;
  paidAt?: string;
  validUntil?: string;
  channel: 'manual' | 'wechat' | 'alipay' | 'bank';
};

@Component({
  selector: 'ops-root',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    OverviewPageComponent,
    OperatorsPageComponent,
    RelayNodesPageComponent,
    PunchNodesPageComponent,
    CustomersPageComponent,
    DevicesPageComponent,
    ClientDownloadsPageComponent,
    ProductsPageComponent,
    OrdersPageComponent,
    RenewalsPageComponent,
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
    { id: 'overview', label: '运营管理', desc: '平台指标与待处理事项' },
    { id: 'operators', label: '运营用户', desc: '后台账号与角色' },
    { id: 'relayNodes', label: '中继节点', desc: 'Relay/DERP 容量管理' },
    { id: 'punchNodes', label: '打洞节点', desc: 'P2P Punch 节点管理' },
    { id: 'customers', label: '客户管理', desc: '客户资源与限流状态' },
    { id: 'devices', label: '设备管理', desc: '全局设备、在线与启用状态' },
    { id: 'clientDownloads', label: '客户端发布', desc: '安装包上传与下载' },
    { id: 'products', label: '商品管理', desc: '客户级别、套餐商品与流量包' },
    { id: 'orders', label: '订单管理', desc: '购买、支付与开通状态' },
    { id: 'renewals', label: '续费管理', desc: '有效期和手动续费' },
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
  showAssignPlanDialog = false;
  showCurrentPasswordDialog = false;
  showOperatorPasswordDialog = false;
  showOperatorDialog = false;
  showRelayNodeDialog = false;
  showPunchNodeDialog = false;
  showPlanDialog = false;
  showProductDialog = false;
  showOrderDialog = false;
  showCustomerDialog = false;
  showDeviceDialog = false;
  showRenewalDialog = false;
  selectedCustomer: Customer | null = null;
  selectedOperator: OperatorUser | null = null;
  selectedRelayNode: RelayNode | null = null;
  selectedPunchNode: PunchNode | null = null;
  selectedPlan: CustomerPlan | null = null;
  selectedProduct: Product | null = null;
  selectedOrder: Order | null = null;
  selectedDevice: OpsDevice | null = null;
  selectedRenewal: Renewal | null = null;
  assignPlanCode: CustomerPlan['code'] = 'pro';
  assignExpiresAt = '2027-05-09';
  renewalAmount = 299;
  oldPassword = '';
  newPassword = '';
  confirmPassword = '';
  operatorNewPassword = '';
  operatorConfirmPassword = '';
  passwordMessage = '';
  operatorForm: Partial<OperatorUser> = {};
  relayNodeForm: Partial<RelayNode> = {};
  punchNodeForm: Partial<PunchNode> = {};
  planForm: Partial<CustomerPlan> = {};
  productForm: Partial<Product> = {};
  orderForm: Partial<Order> = {};
  customerForm: Partial<Customer> = {};
  deviceForm: Partial<OpsDevice> = {};
  renewalForm: Partial<Renewal> = {};
  downloadForm: Partial<ClientDownload> = {
    platform: 'macos',
    version: '2.0.0',
    arch: 'universal',
    channel: 'stable',
    status: 'active',
    releaseNotes: '',
  };
  selectedDownloadFile: File | null = null;
  selectedDownloadFileName = '';
  deviceKeyword = '';

  // 传给 feature 子页面的视图模型，保持子页面只负责模板渲染。
  get vm(): this {
    return this;
  }

  get isLoggedIn(): boolean {
    return Boolean(localStorage.getItem(this.opsTokenKey));
  }

  ngOnInit(): void {
    if (this.isLoggedIn) {
      void this.loadOpsData();
    }
  }

  operators: OperatorUser[] = [
    { operatorId: 'op-000001', name: '平台管理员', email: 'admin@slan.com', role: 'owner', status: 'active', lastLoginAt: '2026-05-09 09:20' },
    { operatorId: 'op-000002', name: '运营值班', email: 'ops@slan.com', role: 'ops', status: 'active', lastLoginAt: '2026-05-08 22:10' },
    { operatorId: 'op-000003', name: '财务', email: 'finance@slan.com', role: 'finance', status: 'active', lastLoginAt: '2026-05-07 18:42' },
  ];

  relayNodes: RelayNode[] = [
    { nodeId: 'relay-hk-001', name: '香港 Relay 1', region: 'ap-east-1', transport: 'relay_udp', publicAddr: 'udp://hk1.relay.slan.com:3478', maxBandwidthMbps: 1000, monthlyTrafficGb: 20480, usedTrafficGb: 6830, maxSessions: 8000, activeSessions: 2310, status: 'active', health: 'healthy' },
    { nodeId: 'derp-tokyo-001', name: '东京 DERP 1', region: 'ap-northeast-1', transport: 'derp_tcp_tls_443', publicAddr: 'https://tyo1.derp.slan.com', maxBandwidthMbps: 500, monthlyTrafficGb: 10240, usedTrafficGb: 9120, maxSessions: 4000, activeSessions: 3380, status: 'maintenance', health: 'warning' },
    { nodeId: 'relay-sg-001', name: '新加坡 Relay 1', region: 'ap-southeast-1', transport: 'relay_udp', publicAddr: 'udp://sg1.relay.slan.com:3478', maxBandwidthMbps: 800, monthlyTrafficGb: 15360, usedTrafficGb: 4210, maxSessions: 6000, activeSessions: 1740, status: 'active', health: 'healthy' },
  ];

  punchNodes: PunchNode[] = [
    { nodeId: 'punch-hk-001', name: '香港 Punch 1', region: 'ap-east-1', publicUdpIp: '47.245.40.231', publicUdpPort: 29130, maxSessions: 10000, activeSessions: 1280, status: 'active', health: 'healthy', priority: 10, createdAt: '2026-05-09 09:00', updatedAt: '2026-05-09 09:20' },
  ];

  plans: CustomerPlan[] = [
    { code: 'free', name: '免费版', ownDeviceLimit: 5, invitedDeviceLimit: 5, totalDeviceLimit: 10, relayMonthlyGb: 50, relayBandwidthMbps: 5, relayThrottleMbps: 1, p2pUnlimited: true, customDomain: false, acl: false, dedicatedRelay: false, auditLog: false, apiAccess: false, monthlyPrice: 0, yearlyPrice: 0 },
    { code: 'pro', name: '专业版', ownDeviceLimit: 30, invitedDeviceLimit: 100, totalDeviceLimit: 130, relayMonthlyGb: 1024, relayBandwidthMbps: 100, relayThrottleMbps: 20, p2pUnlimited: true, customDomain: true, acl: true, dedicatedRelay: false, auditLog: false, apiAccess: false, monthlyPrice: 39, yearlyPrice: 299 },
    { code: 'enterprise', name: '企业版', ownDeviceLimit: 500, invitedDeviceLimit: 5000, totalDeviceLimit: 5500, relayMonthlyGb: 10240, relayBandwidthMbps: 1000, relayThrottleMbps: 200, p2pUnlimited: true, customDomain: true, acl: true, dedicatedRelay: true, auditLog: true, apiAccess: true, monthlyPrice: 0, yearlyPrice: 2999 },
  ];

  products: Product[] = [
    { productId: 'prod-free', name: '免费版', type: 'plan', planCode: 'free', period: 'monthly', validDays: 30, relayTrafficGb: 50, relayBandwidthMbps: 5, listPrice: 0, salePrice: 0, currency: 'CNY', autoRenew: false, status: 'active', description: '默认开通，P2P 不限，Relay 超限降速' },
    { productId: 'prod-pro-month', name: '专业版月付', type: 'plan', planCode: 'pro', period: 'monthly', validDays: 31, relayTrafficGb: 1024, relayBandwidthMbps: 100, listPrice: 39, salePrice: 39, currency: 'CNY', autoRenew: true, status: 'active', description: '适合个人和小团队，支持自定义域名和 ACL' },
    { productId: 'prod-pro-year', name: '专业版年付', type: 'plan', planCode: 'pro', period: 'yearly', validDays: 365, relayTrafficGb: 1024, relayBandwidthMbps: 100, listPrice: 468, salePrice: 299, currency: 'CNY', autoRenew: true, status: 'active', description: '专业版年付优惠' },
    { productId: 'prod-relay-100g', name: 'Relay 加油包 100GB', type: 'traffic_pack', period: 'one_time', validDays: 31, relayTrafficGb: 100, relayBandwidthMbps: 0, listPrice: 19, salePrice: 19, currency: 'CNY', autoRenew: false, status: 'active', description: '当月 Relay 流量加购，不改变客户级别' },
    { productId: 'prod-enterprise', name: '企业定制包', type: 'enterprise', planCode: 'enterprise', period: 'contract', validDays: 365, relayTrafficGb: 10240, relayBandwidthMbps: 1000, listPrice: 29999, salePrice: 2999, currency: 'CNY', autoRenew: false, status: 'active', description: '专属 Relay、审计日志、API 接入，按合同报价' },
  ];

  customers: Customer[] = [
    { customerId: 'cust-000001', email: 'alice@staticlss.com', name: 'Alice', country: '中国', province: '广东', city: '深圳', ipRegion: '华南', planCode: 'pro', planExpiresAt: '2027-05-09', ownDevices: 3, invitedDevices: 2, relayUsedGb: 318, status: 'active' },
    { customerId: 'cust-000002', email: 'bob@staticlss.com', name: 'Bob', country: '中国', province: '上海', city: '上海', ipRegion: '华东', planCode: 'free', planExpiresAt: '2026-06-01', ownDevices: 5, invitedDevices: 4, relayUsedGb: 48, status: 'limited' },
    { customerId: 'cust-000003', email: 'corp@example.com', name: '企业客户', country: '新加坡', province: '-', city: 'Singapore', ipRegion: '亚太', planCode: 'enterprise', planExpiresAt: '2028-01-01', ownDevices: 560, invitedDevices: 820, relayUsedGb: 6230, status: 'active' },
  ];

  renewals: Renewal[] = [
    { renewalId: 'renew-000001', customerEmail: 'alice@staticlss.com', planCode: 'pro', period: 'yearly', amount: 299, paidAt: '2026-05-09', validUntil: '2027-05-09', source: 'alipay', operator: 'admin@slan.com' },
    { renewalId: 'renew-000002', customerEmail: 'corp@example.com', planCode: 'enterprise', period: 'yearly', amount: 12999, paidAt: '2026-01-01', validUntil: '2028-01-01', source: 'bank', operator: 'finance@slan.com' },
  ];

  orders: Order[] = [
    { orderId: 'ord-202605090001', customerEmail: 'alice@staticlss.com', productName: '专业版年付', productType: 'plan', amount: 299, payStatus: 'paid', provisionStatus: 'provisioned', createdAt: '2026-05-09 09:10', paidAt: '2026-05-09 09:12', validUntil: '2027-05-09', channel: 'alipay' },
    { orderId: 'ord-202605090002', customerEmail: 'bob@staticlss.com', productName: 'Relay 加油包 100GB', productType: 'traffic_pack', amount: 19, payStatus: 'pending', provisionStatus: 'pending', createdAt: '2026-05-09 10:40', channel: 'wechat' },
    { orderId: 'ord-202601010001', customerEmail: 'corp@example.com', productName: '企业定制包', productType: 'enterprise', amount: 12999, payStatus: 'paid', provisionStatus: 'provisioned', createdAt: '2026-01-01 13:20', paidAt: '2026-01-01 14:02', validUntil: '2028-01-01', channel: 'bank' },
  ];

  devices: OpsDevice[] = [
    { deviceId: 'mac-001', ownerId: 'user-000001', ownerEmail: 'alice@staticlss.com', name: '办公 Mac', alias: '办公 Mac', platform: 'macos', osName: 'macOS', osVersion: '15.3', globalIp: '10.0.0.1', globalName: 'mac-001.staticlss.com', status: 'active', heartbeatOnline: true, networkEnabled: true, deviceEnabled: true, rxBytesTotal: 0, txBytesTotal: 0, networkCount: 1, lastSeenAt: '2026-05-09 09:20', lastReportAt: '2026-05-09 09:20', createdAt: '2026-05-09 09:00', updatedAt: '2026-05-09 09:20' },
  ];
  clientDownloads: ClientDownload[] = [];

  async login(): Promise<void> {
    this.loginMessage = '';
    if (!this.loginEmail.trim() || !this.loginPassword.trim()) {
      this.loginMessage = '请输入邮箱和密码';
      return;
    }
    try {
      const response = await this.request<{ auth: { operator: OperatorUser; session: { token: string } } }>('POST', '/api/ops/auth/login', {
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
      const [operators, relayNodes, punchNodes, customers, devices, downloads, plans, products, orders, renewals] = await Promise.all([
        this.request<{ items: OperatorUser[] }>('GET', '/api/ops/operators'),
        this.request<{ items: RelayNode[] }>('GET', '/api/ops/relay-nodes'),
        this.request<{ items: PunchNode[] }>('GET', '/api/ops/punch-nodes'),
        this.request<{ items: Customer[] }>('GET', '/api/ops/customers'),
        this.request<{ items: OpsDevice[] }>('GET', '/api/ops/devices'),
        this.request<{ items: ClientDownload[] }>('GET', '/api/ops/client-downloads'),
        this.request<{ items: CustomerPlan[] }>('GET', '/api/ops/plans'),
        this.request<{ items: Product[] }>('GET', '/api/ops/products'),
        this.request<{ items: Order[] }>('GET', '/api/ops/orders'),
        this.request<{ items: Renewal[] }>('GET', '/api/ops/renewals'),
      ]);
      this.operators = operators.items.map((item) => ({ ...item, lastLoginAt: this.formatDateTime(item.lastLoginAt) }));
      this.relayNodes = relayNodes.items;
      this.punchNodes = punchNodes.items.map((item) => ({
        ...item,
        createdAt: this.formatDateTime(item.createdAt),
        updatedAt: this.formatDateTime(item.updatedAt),
      }));
      this.customers = customers.items.map((item) => ({ ...item, planExpiresAt: this.formatDate(item.planExpiresAt) }));
      this.devices = devices.items.map((item) => this.formatDevice(item));
      this.clientDownloads = downloads.items.map((item) => ({
        ...item,
        createdAt: this.formatDateTime(item.createdAt),
        updatedAt: this.formatDateTime(item.updatedAt),
      }));
      this.plans = plans.items;
      this.products = products.items;
      this.orders = orders.items.map((item) => ({
        ...item,
        createdAt: this.formatDateTime(item.createdAt),
        paidAt: item.paidAt ? this.formatDateTime(item.paidAt) : undefined,
        validUntil: item.validUntil ? this.formatDate(item.validUntil) : undefined,
      }));
      this.renewals = renewals.items.map((item) => ({
        ...item,
        paidAt: this.formatDate(item.paidAt),
        validUntil: this.formatDate(item.validUntil),
      }));
    } catch (error) {
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
      throw new Error(text || `HTTP ${response.status}`);
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
    return this.navItems.find((item) => item.id === this.active) ?? this.navItems[0];
  }

  get totalCustomers(): number {
    return this.customers.length;
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

  get paidOrders(): Order[] {
    return this.orders.filter((order) => order.payStatus === 'paid');
  }

  get todayRevenue(): number {
    return this.paidOrders
      .filter((order) => order.paidAt?.startsWith('2026-05-09'))
      .reduce((sum, order) => sum + order.amount, 0);
  }

  get monthlyRevenue(): number {
    return this.paidOrders
      .filter((order) => order.paidAt?.startsWith('2026-05'))
      .reduce((sum, order) => sum + order.amount, 0);
  }

  get yearlyRevenue(): number {
    return this.paidOrders
      .filter((order) => order.paidAt?.startsWith('2026'))
      .reduce((sum, order) => sum + order.amount, 0);
  }

  get pendingRevenue(): number {
    return this.orders
      .filter((order) => order.payStatus === 'pending')
      .reduce((sum, order) => sum + order.amount, 0);
  }

  get paidOrderCount(): number {
    return this.orders.filter((order) => order.payStatus === 'paid').length;
  }

  get pendingOrderCount(): number {
    return this.orders.filter((order) => order.payStatus === 'pending').length;
  }

  get provisionedOrderCount(): number {
    return this.orders.filter((order) => order.provisionStatus === 'provisioned').length;
  }

  get closedOrderCount(): number {
    return this.orders.filter((order) => order.payStatus === 'closed' || order.payStatus === 'refunded').length;
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
    this.active = id;
  }

  planName(code: CustomerPlan['code']): string {
    return this.plans.find((plan) => plan.code === code)?.name ?? code;
  }

  planOf(code: CustomerPlan['code']): CustomerPlan {
    return this.plans.find((plan) => plan.code === code) ?? this.plans[0];
  }

  customerRelayPercent(customer: Customer): number {
    const plan = this.planOf(customer.planCode);
    return Math.min(100, Math.round((customer.relayUsedGb / plan.relayMonthlyGb) * 100));
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

  openAssignPlan(customer: Customer): void {
    this.selectedCustomer = customer;
    this.assignPlanCode = customer.planCode;
    this.assignExpiresAt = customer.planExpiresAt;
    this.renewalAmount = this.planOf(customer.planCode).yearlyPrice;
    this.showAssignPlanDialog = true;
  }

  closeAssignPlan(): void {
    this.showAssignPlanDialog = false;
    this.selectedCustomer = null;
  }

  openOperatorDialog(operator?: OperatorUser): void {
    this.selectedOperator = operator ?? null;
    this.operatorForm = operator ? { ...operator } : { name: '', email: '', role: 'ops', status: 'active' };
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
    try {
      const isEdit = Boolean(this.selectedOperator);
      const path = isEdit ? `/api/ops/operators/${encodeURIComponent(this.selectedOperator!.operatorId)}` : '/api/ops/operators';
      const operator = await this.request<OperatorUser>(isEdit ? 'PATCH' : 'POST', path, this.operatorForm);
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
    this.relayNodeForm = node ? { ...node } : {
      name: '',
      region: 'ap-east-1',
      transport: 'relay_udp',
      publicAddr: '',
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
  }

  async saveRelayNodeDialog(): Promise<void> {
    if (!this.relayNodeForm.name?.trim() || !this.relayNodeForm.publicAddr?.trim()) {
      this.apiMessage = '请输入节点名称和公网地址';
      return;
    }
    const publicAddr = this.relayNodeForm.publicAddr.trim();
    const duplicated = this.relayNodes.some((node) => node.publicAddr === publicAddr && node.nodeId !== this.selectedRelayNode?.nodeId);
    if (duplicated) {
      this.apiMessage = '公网地址已存在，不能重复配置到多个中继节点';
      return;
    }
    try {
      const isEdit = Boolean(this.selectedRelayNode);
      const path = isEdit ? `/api/ops/relay-nodes/${encodeURIComponent(this.selectedRelayNode!.nodeId)}` : '/api/ops/relay-nodes';
      const node = await this.request<RelayNode>(isEdit ? 'PATCH' : 'POST', path, {
        nodeId: this.selectedRelayNode?.nodeId,
        name: this.relayNodeForm.name,
        region: this.relayNodeForm.region,
        transport: this.relayNodeForm.transport,
        publicAddr,
        maxBandwidthMbps: this.relayNodeForm.maxBandwidthMbps,
        monthlyTrafficGb: this.relayNodeForm.monthlyTrafficGb,
        maxSessions: this.relayNodeForm.maxSessions,
        status: this.relayNodeForm.status,
      });
      this.relayNodes = [node, ...this.relayNodes.filter((item) => item.nodeId !== node.nodeId)];
      this.closeRelayNodeDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openPunchNodeDialog(node?: PunchNode): void {
    this.selectedPunchNode = node ?? null;
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
  }

  async savePunchNodeDialog(): Promise<void> {
    if (!this.punchNodeForm.name?.trim() || !this.punchNodeForm.publicUdpIp?.trim()) {
      this.apiMessage = '请输入打洞节点名称和公网 UDP IP';
      return;
    }
    const publicUdpIp = this.punchNodeForm.publicUdpIp.trim();
    const publicUdpPort = Number(this.punchNodeForm.publicUdpPort ?? 0);
    if (publicUdpPort <= 0 || publicUdpPort > 65534) {
      this.apiMessage = '公网 UDP 端口必须在 1-65534 范围内';
      return;
    }
    const duplicated = this.punchNodes.some((node) =>
      node.publicUdpIp === publicUdpIp &&
      node.publicUdpPort === publicUdpPort &&
      node.nodeId !== this.selectedPunchNode?.nodeId
    );
    if (duplicated) {
      this.apiMessage = '公网 UDP IP 和端口已存在，不能重复配置到多个打洞节点';
      return;
    }
    try {
      const isEdit = Boolean(this.selectedPunchNode);
      const path = isEdit ? `/api/ops/punch-nodes/${encodeURIComponent(this.selectedPunchNode!.nodeId)}` : '/api/ops/punch-nodes';
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
      this.closePunchNodeDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openPlanDialog(plan?: CustomerPlan): void {
    this.selectedPlan = plan ?? null;
    this.planForm = plan ? { ...plan } : {
      code: 'custom' as CustomerPlan['code'],
      name: '',
      ownDeviceLimit: 5,
      invitedDeviceLimit: 5,
      totalDeviceLimit: 10,
      relayMonthlyGb: 50,
      relayBandwidthMbps: 5,
      relayThrottleMbps: 1,
      p2pUnlimited: true,
      customDomain: false,
      acl: false,
      dedicatedRelay: false,
      auditLog: false,
      apiAccess: false,
      monthlyPrice: 0,
      yearlyPrice: 0,
    };
    this.showPlanDialog = true;
  }

  closePlanDialog(): void {
    this.showPlanDialog = false;
    this.selectedPlan = null;
  }

  async savePlanDialog(): Promise<void> {
    if (!this.planForm.code?.trim() || !this.planForm.name?.trim()) {
      this.apiMessage = '请输入套餐编码和名称';
      return;
    }
    try {
      const isEdit = Boolean(this.selectedPlan);
      const path = isEdit ? `/api/ops/plans/${encodeURIComponent(this.selectedPlan!.code)}` : '/api/ops/plans';
      const plan = await this.request<CustomerPlan>(isEdit ? 'PATCH' : 'POST', path, this.planForm);
      this.plans = [plan, ...this.plans.filter((item) => item.code !== plan.code)];
      this.closePlanDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openProductDialog(product?: Product): void {
    this.selectedProduct = product ?? null;
    this.productForm = product ? { ...product } : {
      name: '',
      type: 'plan',
      planCode: this.plans[0]?.code,
      period: 'monthly',
      validDays: 31,
      relayTrafficGb: 50,
      relayBandwidthMbps: 5,
      listPrice: 0,
      salePrice: 0,
      currency: 'CNY',
      autoRenew: false,
      status: 'active',
      description: '',
    };
    this.showProductDialog = true;
  }

  closeProductDialog(): void {
    this.showProductDialog = false;
    this.selectedProduct = null;
  }

  async saveProductDialog(): Promise<void> {
    if (!this.productForm.name?.trim()) {
      this.apiMessage = '请输入商品名称';
      return;
    }
    try {
      const isEdit = Boolean(this.selectedProduct);
      const path = isEdit ? `/api/ops/products/${encodeURIComponent(this.selectedProduct!.productId)}` : '/api/ops/products';
      const product = await this.request<Product>(isEdit ? 'PATCH' : 'POST', path, this.productForm);
      this.products = [product, ...this.products.filter((item) => item.productId !== product.productId)];
      this.closeProductDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openOrderDialog(order?: Order): void {
    this.selectedOrder = order ?? null;
    if (order) {
      this.orderForm = { ...order };
      return void (this.showOrderDialog = true);
    }
    const customer = this.customers[0];
    const product = this.products[0];
    this.orderForm = {
      customerId: customer?.customerId,
      customerEmail: customer?.email,
      productId: product?.productId,
      productName: product?.name,
      productType: product?.type,
      amount: product?.salePrice ?? 0,
      currency: 'CNY',
      payStatus: 'pending',
      provisionStatus: 'pending',
      channel: 'manual',
    } as Partial<Order>;
    this.showOrderDialog = true;
  }

  closeOrderDialog(): void {
    this.showOrderDialog = false;
    this.selectedOrder = null;
  }

  async saveOrderDialog(): Promise<void> {
    if (!this.orderForm.customerId || !this.orderForm.productId) {
      this.apiMessage = '请选择客户和商品';
      return;
    }
    try {
      const product = this.products.find((item) => item.productId === this.orderForm.productId);
      const customer = this.customers.find((item) => item.customerId === this.orderForm.customerId);
      const isEdit = Boolean(this.selectedOrder);
      const path = isEdit ? `/api/ops/orders/${encodeURIComponent(this.selectedOrder!.orderId)}` : '/api/ops/orders';
      const order = await this.request<Order>(isEdit ? 'PATCH' : 'POST', path, {
        orderId: this.selectedOrder?.orderId,
        customerId: this.orderForm.customerId,
        customerEmail: customer?.email ?? this.orderForm.customerEmail,
        productId: this.orderForm.productId,
        productName: product?.name ?? this.orderForm.productName,
        productType: product?.type ?? this.orderForm.productType,
        amount: this.orderForm.amount,
        currency: this.orderForm.currency ?? 'CNY',
        payStatus: this.orderForm.payStatus,
        provisionStatus: this.orderForm.provisionStatus,
        channel: this.orderForm.channel,
      });
      const formatted = {
        ...order,
        createdAt: this.formatDateTime(order.createdAt),
        paidAt: order.paidAt ? this.formatDateTime(order.paidAt) : undefined,
        validUntil: order.validUntil ? this.formatDate(order.validUntil) : undefined,
      };
      this.orders = [formatted, ...this.orders.filter((item) => item.orderId !== order.orderId)];
      this.closeOrderDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openCustomerDialog(customer: Customer): void {
    this.selectedCustomer = customer;
    this.customerForm = { ...customer };
    this.showCustomerDialog = true;
  }

  closeCustomerDialog(): void {
    this.showCustomerDialog = false;
    this.selectedCustomer = null;
  }

  async saveCustomerDialog(): Promise<void> {
    if (!this.selectedCustomer || !this.customerForm.email?.trim()) {
      this.apiMessage = '请输入客户邮箱';
      return;
    }
    try {
      const customer = await this.request<Customer>('PATCH', `/api/ops/customers/${encodeURIComponent(this.selectedCustomer.customerId)}`, this.customerForm);
      const formatted = { ...customer, planExpiresAt: this.formatDate(customer.planExpiresAt) };
      this.customers = [formatted, ...this.customers.filter((item) => item.customerId !== customer.customerId)];
      this.closeCustomerDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
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
      const updated = await this.request<OpsDevice>('PATCH', `/api/ops/devices/${encodeURIComponent(this.selectedDevice.deviceId)}`, {
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

  onClientDownloadFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0] ?? null;
    this.selectedDownloadFile = file;
    this.selectedDownloadFileName = file?.name ?? '';
  }

  async uploadClientDownload(): Promise<void> {
    this.apiMessage = '';
    if (!this.downloadForm.platform || !this.downloadForm.version?.trim() || !this.selectedDownloadFile) {
      this.apiMessage = '请选择平台、填写版本并选择安装包';
      return;
    }
    const body = new FormData();
    body.set('platform', this.downloadForm.platform);
    body.set('version', this.downloadForm.version);
    body.set('arch', this.downloadForm.arch ?? '');
    body.set('channel', this.downloadForm.channel ?? 'stable');
    body.set('status', this.downloadForm.status ?? 'active');
    body.set('releaseNotes', this.downloadForm.releaseNotes ?? '');
    body.set('file', this.selectedDownloadFile);
    try {
      const token = localStorage.getItem(this.opsTokenKey);
      const response = await fetch('/api/ops/client-downloads', {
        method: 'POST',
        headers: token ? { Authorization: `Bearer ${token}` } : undefined,
        body,
      });
      if (!response.ok) {
        if (response.status === 401) {
          this.logout('登录已过期，请重新登录');
        }
        throw new Error(await response.text() || `HTTP ${response.status}`);
      }
      const item = await response.json() as ClientDownload;
      this.clientDownloads = [
        { ...item, createdAt: this.formatDateTime(item.createdAt), updatedAt: this.formatDateTime(item.updatedAt) },
        ...this.clientDownloads.filter((download) => download.downloadId !== item.downloadId),
      ];
      this.selectedDownloadFile = null;
      this.selectedDownloadFileName = '';
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async deleteClientDownload(item: ClientDownload): Promise<void> {
    try {
      await this.request('DELETE', `/api/ops/client-downloads/${encodeURIComponent(item.downloadId)}`);
      this.clientDownloads = this.clientDownloads.filter((download) => download.downloadId !== item.downloadId);
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async saveAssignPlan(): Promise<void> {
    if (!this.selectedCustomer) {
      return;
    }
    this.apiMessage = '';
    try {
      const response = await this.request<{ customer: Customer; renewal: Renewal }>('POST', `/api/ops/customers/${encodeURIComponent(this.selectedCustomer.customerId)}/assign-plan`, {
        planCode: this.assignPlanCode,
        expiresAt: this.dateToUnix(this.assignExpiresAt),
        amount: this.renewalAmount,
        period: 'custom',
      });
      Object.assign(this.selectedCustomer, {
        ...response.customer,
        planExpiresAt: this.formatDate(response.customer.planExpiresAt),
      });
      this.renewals = [
        {
          ...response.renewal,
          paidAt: this.formatDate(response.renewal.paidAt),
          validUntil: this.formatDate(response.renewal.validUntil),
        },
        ...this.renewals,
      ];
      this.closeAssignPlan();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  openRenewalDialog(renewal: Renewal): void {
    this.selectedRenewal = renewal;
    this.renewalForm = { ...renewal };
    this.showRenewalDialog = true;
  }

  closeRenewalDialog(): void {
    this.showRenewalDialog = false;
    this.selectedRenewal = null;
  }

  async saveRenewalDialog(): Promise<void> {
    if (!this.selectedRenewal || !this.renewalForm.customerEmail || !this.renewalForm.planCode) {
      this.apiMessage = '请选择客户和套餐';
      return;
    }
    try {
      const renewal = await this.request<Renewal>('PATCH', `/api/ops/renewals/${encodeURIComponent(this.selectedRenewal.renewalId)}`, {
        ...this.renewalForm,
        paidAt: this.dateToUnix(String(this.renewalForm.paidAt)),
        validUntil: this.dateToUnix(String(this.renewalForm.validUntil)),
      });
      const formatted = {
        ...renewal,
        paidAt: this.formatDate(renewal.paidAt),
        validUntil: this.formatDate(renewal.validUntil),
      };
      this.renewals = [formatted, ...this.renewals.filter((item) => item.renewalId !== renewal.renewalId)];
      const customer = this.customers.find((item) => item.customerId === renewal.customerId);
      if (customer) {
        customer.planCode = renewal.planCode;
        customer.planExpiresAt = formatted.validUntil;
      }
      this.closeRenewalDialog();
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async toggleOperator(operator: OperatorUser): Promise<void> {
    const nextStatus = operator.status === 'active' ? 'disabled' : 'active';
    try {
      const updated = await this.request<OperatorUser>('PATCH', `/api/ops/operators/${encodeURIComponent(operator.operatorId)}`, {
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
      await this.request('PATCH', '/api/ops/auth/password', {
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
      await this.request('POST', `/api/ops/operators/${encodeURIComponent(this.selectedOperator.operatorId)}/password`, {
        newPassword: this.operatorNewPassword,
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
      const updated = await this.request<RelayNode>('PATCH', `/api/ops/relay-nodes/${encodeURIComponent(node.nodeId)}/status`, {
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
      await this.request('DELETE', `/api/ops/relay-nodes/${encodeURIComponent(node.nodeId)}`);
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
      const updated = await this.request<PunchNode>('PATCH', `/api/ops/punch-nodes/${encodeURIComponent(node.nodeId)}/status`, {
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
      await this.request('DELETE', `/api/ops/punch-nodes/${encodeURIComponent(node.nodeId)}`);
      this.punchNodes = this.punchNodes.filter((item) => item.nodeId !== node.nodeId);
      this.apiMessage = `UDP 打洞 ${node.name} 已删除`;
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async toggleProduct(product: Product): Promise<void> {
    const nextStatus = product.status === 'active' ? 'offline' : 'active';
    try {
      const updated = await this.request<Product>('PATCH', `/api/ops/products/${encodeURIComponent(product.productId)}`, {
        ...product,
        status: nextStatus,
      });
      Object.assign(product, updated);
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
  }

  async toggleDevice(device: OpsDevice): Promise<void> {
    const enabled = !device.deviceEnabled;
    try {
      const updated = await this.request<OpsDevice>('PATCH', `/api/ops/devices/${encodeURIComponent(device.deviceId)}`, {
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
      await this.request('DELETE', `/api/ops/devices/${encodeURIComponent(device.deviceId)}`);
      this.devices = this.devices.filter((item) => item.deviceId !== device.deviceId);
      this.notifyStateChanged();
    } catch (error) {
      this.apiMessage = this.errorMessage(error);
      this.notifyStateChanged();
    }
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
